# SQLite Event Store 设计规格

> 引入 SQLite 持久化事件日志，支持会话历史回溯与 VibeCoding 质量分析。

## 概述

### 目标

1. 持久化所有 Hook 事件，支持历史回溯和 VibeCoding 质量分析（交互次数越多 → 可能需求没交代清楚）
2. 全量对接 Claude Code 8 种 Hook 事件类型
3. 前端排序改为：项目按名称排序，Session 按时间排序
4. 支持用户主动删除会话（软删除），空项目自动隐藏
5. 支持用户配置启动加载范围和手动清理历史数据

### 架构模式

**Event Sourcing**：SessionTracker (内存状态机，驱动实时 UI) + EventStore (SQLite 持久化，驱动历史分析和重启恢复)。

```
AgentEvent 到达 HTTP Server
    ├──→ SessionTracker.TrackEvent(e)     ← 内存状态机，< 1μs，立即推 UI
    └──→ EventStore.Record(e)             ← SQLite WAL，异步批量写入
```

### 不变约束

- Hook is fire-and-forget：bridge 永不被阻塞
- 单进程架构不变（Daemon + UI 同一 Wails 进程）
- AgentEvent 仍是唯一跨层数据结构
- 四层架构依赖规则不变：domain ← adapter ← wails；infra 被所有层使用

---

## 数据模型

### DDL — t_sessions

```sql
CREATE TABLE t_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,  -- 自增主键
    session_key      TEXT    NOT NULL UNIQUE,            -- 会话唯一标识 (session_id 或 host:cwd:tty)
    session_id       TEXT    NOT NULL DEFAULT '',        -- CC 原生 session_id
    agent            TEXT    NOT NULL DEFAULT 'claude-code', -- 代理类型: claude-code / codex
    host             TEXT    NOT NULL DEFAULT '',        -- 主机名
    cwd              TEXT    NOT NULL DEFAULT '',        -- 工作目录
    project_name     TEXT    NOT NULL DEFAULT '',        -- 项目名 (CWD 末段，用于分组排序)
    tty              TEXT    NOT NULL DEFAULT '',        -- 终端 TTY 路径
    term_program     TEXT    NOT NULL DEFAULT '',        -- 终端程序: iTerm2 / Terminal.app
    iterm_session_id TEXT    NOT NULL DEFAULT '',        -- iTerm2 会话ID (跳转用)
    status           TEXT    NOT NULL DEFAULT 'active',  -- 当前状态: waiting/active/finished/error
    attention_level  TEXT    NOT NULL DEFAULT 'running', -- 注意力级别: attention/running/done
    agent_label      TEXT    NOT NULL DEFAULT 'CC',      -- 代理标签
    created_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 首次事件时间
    updated_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 最后事件时间
    deleted_at       TEXT             DEFAULT NULL       -- 软删除标记 (NULL=未删除)
);

CREATE INDEX idx_sessions_project ON t_sessions(project_name, updated_at DESC);
CREATE INDEX idx_sessions_updated ON t_sessions(updated_at DESC);
CREATE INDEX idx_sessions_deleted ON t_sessions(deleted_at);
```

### DDL — t_events

```sql
CREATE TABLE t_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,  -- 自增主键
    session_key     TEXT    NOT NULL,                   -- 所属会话标识 (FK → t_sessions.session_key)
    event_type      TEXT    NOT NULL,                   -- 事件类型 (8种，见下)
    tool_name       TEXT    NOT NULL DEFAULT '',        -- 工具名称: Bash/Edit/Read/Write/...
    tool_use_id     TEXT    NOT NULL DEFAULT '',        -- 工具调用唯一ID
    content         TEXT    NOT NULL DEFAULT '',        -- 截断显示内容 (≤60 rune)
    content_raw     TEXT    NOT NULL DEFAULT '',        -- 完整内容摘要
    attention_level TEXT    NOT NULL DEFAULT '',        -- 事件级别
    permission_mode TEXT    NOT NULL DEFAULT '',        -- 权限模式
    raw_payload     BLOB             DEFAULT NULL,     -- 完整 stdin JSON (原始字节，全量存储)
    timestamp       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 事件发生时间

    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
);

CREATE INDEX idx_events_session_ts ON t_events(session_key, timestamp DESC);
CREATE INDEX idx_events_type       ON t_events(event_type);
CREATE INDEX idx_events_timestamp  ON t_events(timestamp DESC);
```

### event_type 枚举（全量 8 种）

| event_type | Hook 来源 | 说明 | 状态机影响 |
|---|---|---|---|
| `pre_tool_use` | PreToolUse | 工具执行前 | Status → waiting, 添加 PendingTools |
| `post_tool_use` | PostToolUse | 工具执行后 | 移除 PendingTools, 可能 → active |
| `stop` | Stop | 主 Agent 完成 | Status → finished |
| `error` | — | 错误 | Status → error |
| `notification` | Notification | 系统通知 | 不影响状态 |
| `session_start` | SessionStart | 会话启动/恢复 | 创建新 Session, Status = active |
| `user_prompt_submit` | UserPromptSubmit | 用户输入提交 | 不影响状态（仅记录） |
| `subagent_stop` | SubagentStop | 子 Agent 完成 | 不影响主 Session 状态 |
| `pre_compact` | PreCompact | 对话压缩前 | 不影响状态（仅记录） |

---

## Go 模块设计

### SessionTracker 重构 (domain/session/)

原 `Registry` 重命名为 `Tracker`，方法名具体化：

```go
package session

// Tracker maintains real-time session state from an event stream.
type Tracker struct {
    mu       sync.RWMutex
    sessions map[string]*Session
    onUpdate func(sessions []*Session)
}

func NewTracker(onUpdate func([]*Session)) *Tracker

func (t *Tracker) TrackEvent(e *entity.AgentEvent)             // 处理事件，驱动状态转换
func (t *Tracker) Replay(events []*entity.AgentEvent)          // 从历史事件重建状态 (启动用)
func (t *Tracker) ListByRecent() []*Session                    // 按最近活跃排序
func (t *Tracker) Session(key string) (*Session, bool)         // 按 key 查找
func (t *Tracker) SessionByTTY(tty string) (*Session, bool)    // 按 TTY 查找
func (t *Tracker) Dismiss(key string)                          // 用户主动移除
```

`TrackEvent` 内部状态机逻辑不变（switch/case on EventType），新增对 `session_start` 的处理：创建 Session 并设置 Status = active。

### EventStore Interface (infra/store/)

```go
package store

import "pager/internal/domain/entity"

// EventStore defines the contract for persistent event storage.
type EventStore interface {
    // Record 异步写入事件 (放入 channel，不阻塞调用方)
    // Channel 满时丢弃当前事件并打印 slog.Warn
    Record(e *entity.AgentEvent)

    // LoadRecentSessions 加载最近 N 小时内未删除的会话及其事件
    // 返回按时间升序排列的事件列表，用于 Tracker.Replay()
    LoadRecentSessions(hours int) ([]*entity.AgentEvent, error)

    // SessionEvents 查询某会话的所有事件 (按时间升序)
    SessionEvents(sessionKey string) ([]*entity.AgentEvent, error)

    // DismissSession 软删除: UPDATE t_sessions SET deleted_at = NOW() WHERE session_key = ?
    DismissSession(sessionKey string) error

    // PurgeOlderThan 物理删除指定天数前的数据 (t_events + t_sessions)
    PurgeOlderThan(days int) (affected int64, err error)

    // PurgeAll 物理删除所有数据
    PurgeAll() (affected int64, err error)

    // Stats 返回数据库统计信息 (大小、事件总数)
    Stats() (dbSizeBytes int64, eventCount int64, err error)

    // Close 优雅关闭: flush 剩余 buffer → 关闭 DB 连接
    Close() error
}
```

### SQLite 实现 (infra/store/sqlite.go)

关键设计：

- **DB 文件位置**：`~/.config/pager/pager.db`（复用现有配置目录）
- **WAL 模式**：`PRAGMA journal_mode=WAL;` 提升并发读写性能
- **批量写入器**：

```go
type sqliteStore struct {
    db      *sql.DB
    buf     chan *entity.AgentEvent  // cap=512
    ctx     context.Context
    cancel  context.CancelFunc
    wg      sync.WaitGroup
}

// flushLoop 后台 goroutine，批量消费 channel
func (s *sqliteStore) flushLoop() {
    ticker := time.NewTicker(2 * time.Second)
    batch := make([]*entity.AgentEvent, 0, 50)

    for {
        select {
        case e := <-s.buf:
            batch = append(batch, e)
            if len(batch) >= 50 {
                s.writeBatch(batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                s.writeBatch(batch)
                batch = batch[:0]
            }
        case <-s.ctx.Done():
            // drain remaining
            close(s.buf)
            for e := range s.buf {
                batch = append(batch, e)
            }
            if len(batch) > 0 {
                s.writeBatch(batch)
            }
            return
        }
    }
}
```

- **Record 方法**（非阻塞 + 丢弃告警）：

```go
func (s *sqliteStore) Record(e *entity.AgentEvent) {
    select {
    case s.buf <- e:
    default:
        slog.Warn("event buffer full, dropping event",
            "session_key", e.SessionKey(),
            "event_type", e.EventType,
        )
    }
}
```

---

## HTTP Server 变更

`internal/adapter/httpapi/server.go` 增加 EventStore 依赖：

```go
type Server struct {
    tracker *session.Tracker   // 原 reg *session.Registry
    store   store.EventStore   // 新增
}

func (s *Server) HandleEvent(w http.ResponseWriter, req *http.Request) {
    // ... decode event ...
    s.tracker.TrackEvent(&e)   // 内存状态机
    s.store.Record(&e)         // 异步持久化
    w.WriteHeader(http.StatusOK)
}
```

---

## Bridge 扩展

### 新增事件类型支持

`cmd/bridge/main.go` 的 `parseArgs` 需识别新事件类型：

- `session_start` → 从 stdin 解析 `source` 字段
- `user_prompt_submit` → 从 stdin 解析 `prompt` 字段作为 content
- `subagent_stop` → 记录事件
- `pre_compact` → 记录事件

### 新增 CCHookInput 字段 (adapter/bridge/types.go)

```go
type CCHookInput struct {
    SessionID      string          `json:"session_id"`
    TranscriptPath string          `json:"transcript_path"`
    CWD            string          `json:"cwd"`
    HookEventName  string          `json:"hook_event_name"`
    ToolName       string          `json:"tool_name"`
    ToolInput      json.RawMessage `json:"tool_input"`
    ToolUseID      string          `json:"tool_use_id"`
    PermissionMode string          `json:"permission_mode"`
    // 新增字段
    Prompt         string          `json:"prompt"`          // UserPromptSubmit
    Source         string          `json:"source"`          // SessionStart: startup/resume/clear
    Trigger        string          `json:"trigger"`         // PreCompact: manual/auto
    Message        string          `json:"message"`         // Notification
    StopHookActive bool            `json:"stop_hook_active"` // Stop/SubagentStop
}
```

### ExtractContent 扩展 (adapter/bridge/extractor.go)

新增对 `user_prompt_submit`、`session_start` 等事件的内容提取。

---

## App 启动流程变更

`internal/wails/app.go` 启动逻辑：

```
1. 初始化 slog
2. 加载 Settings (含 session_load_hours)
3. 初始化 EventStore (打开 SQLite + 执行 schema 迁移 + 启动 flushLoop)
4. 初始化 SessionTracker
5. 从 EventStore.LoadRecentSessions(hours) 获取历史事件
6. SessionTracker.Replay(events) 重建内存状态
7. 初始化 HTTP Server (注入 tracker + store)
8. 启动 Wails App
```

---

## 前端变更

### 排序逻辑 (store/sessions.ts)

```typescript
// useProjectGroups() 修改
// 当前: groups.sort by 最近活跃时间
// 变更: groups.sort by 项目名称字母序
groups.sort((a, b) => a.project.localeCompare(b.project))

// Session 内部排序保持: UpdatedAt DESC (最新在前)
```

### 会话删除

前端调用 `SessionBinding.DismissSession(key)` → 已有 API，后端逻辑增加 `store.DismissSession`。

项目块隐藏：前端过滤逻辑已有（`filterExpiredDone`），补充对 soft-deleted 的过滤（实际上从 Tracker 移除后自然不会出现在列表中）。

### Settings 数据管理 UI (pages/SettingsPanel.tsx)

新增「数据管理」区域：

- 启动加载范围：数字输入框，单位为小时
- 清理按钮：7天前 / 30天前 / 全部（带二次确认对话框）
- 统计显示：数据库大小、事件总数

---

## 配置变更

`internal/infra/config/config.go` 新增：

```go
type Settings struct {
    // ... existing fields ...
    SessionLoadHours int `json:"session_load_hours"` // 默认 24
}
```

默认值在 `DefaultSettings()` 中设置为 24。

---

## 文件变更清单

| 文件 | 变更 |
|------|------|
| `internal/domain/session/registry.go` | 重命名为 `tracker.go`，重构 API |
| `internal/domain/session/registry_test.go` | 重命名为 `tracker_test.go`，适配新 API |
| `internal/infra/store/store.go` | 重写：新 EventStore interface |
| `internal/infra/store/sqlite.go` | **新建**：SQLite 实现 + 批量写入 |
| `internal/infra/store/sqlite_test.go` | **新建**：测试 |
| `internal/infra/store/schema.sql` | **新建**：DDL 文件 |
| `internal/adapter/httpapi/server.go` | 注入 EventStore，HandleEvent 增加 Record 调用 |
| `internal/adapter/bridge/types.go` | 扩展 CCHookInput 字段 |
| `internal/adapter/bridge/extractor.go` | 新增事件类型内容提取 |
| `internal/adapter/bridge/extractor_test.go` | 补充新事件测试 |
| `cmd/bridge/main.go` | 支持新事件类型解析 |
| `internal/wails/app.go` | 启动流程加入 DB 初始化 + Replay |
| `internal/wails/session_svc.go` | 适配 Tracker API |
| `internal/wails/settings_svc.go` | 新增数据管理绑定方法 |
| `internal/infra/config/config.go` | 新增 SessionLoadHours 字段 |
| `frontend/src/store/sessions.ts` | 排序逻辑变更 |
| `frontend/src/pages/SettingsPanel.tsx` | 新增数据管理 UI 区域 |
| `go.mod` | 新增 `modernc.org/sqlite` 依赖 |

---

## SQLite 驱动选择

使用 `modernc.org/sqlite`（纯 Go，无需 CGo），理由：
- 纯 Go 编译，无 CGo 跨编译问题
- macOS 下性能足够（桌面 app 量级）
- 与 Wails 构建流程兼容

---

## 风险与约束

1. **首次迁移**：v1 → v1.5 升级时 DB 不存在，需自动创建 schema
2. **数据库锁**：单写 goroutine 避免 SQLite 写锁竞争
3. **Replay 性能**：24 小时内事件量级 ~10K-50K 条，重建耗时 <1s（可接受）
4. **磁盘占用**：全量 raw_payload 存储，靠用户手动清理控制增长
