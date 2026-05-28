# Pager — 技术 PRD（for Claude Code）

> Version: 1.0
> 面向对象：Claude Code agent，用于直接指导代码生成
> 产品阶段：MVP v1.0（仅 Claude Code hook 接入，macOS only）
> 最后更新：2026-05-28

---

## 0. 阅读说明

本文档是 Pager 的**完整技术规格**，面向 Claude Code 直接生成代码而写。

约定：
- 所有 `// [IMPL]` 注释标记需要实现的关键逻辑
- 所有 `// [TODO-v1.5]` 标记暂不实现、v1.5 再做的部分
- `// [PoC-VERIFIED]` 标记已在 PoC 阶段验证可行的代码

**不要改动的决策**（已经过产品迭代确认，不接受修改建议）：
1. hook 必须 fire-and-forget，不等响应，不阻塞 agent
2. 不在 Pager 内做 approve/deny 决策
3. 单进程架构（Daemon 和 UI 合在 Wails 主进程里）
4. AgentEvent 是唯一的跨层数据结构

---

## 1. 产品一句话

Pager 是 AI coding agents 的 macOS MenuBar 状态感知层。  
hook 单向异步推送事件 → Pager 弹通知 + 展示执行内容 → 用户一键跳转回对应终端 tab。

---

## 2. 项目结构

```
pager/
├── cmd/
│   └── bridge/
│       └── main.go              # pager-cc-bridge 可执行文件入口
├── internal/
│   ├── bridge/
│   │   ├── types.go             # CCHookInput 定义（CC stdin schema）
│   │   ├── extractor.go         # Content 提取规则
│   │   └── poster.go            # HTTP POST + 重试逻辑
│   ├── event/
│   │   └── types.go             # AgentEvent 统一结构（全项目共用）
│   ├── registry/
│   │   └── registry.go          # SessionRegistry（内存 map + 状态机）
│   ├── server/
│   │   └── server.go            # HTTP server（:7421），接收 AgentEvent
│   ├── notify/
│   │   └── notify.go            # macOS 系统通知（CGO → UNUserNotification）
│   └── terminal/
│       └── jump.go              # osascript 终端跳转
├── frontend/                    # React + Tailwind + zustand
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   │   ├── SessionList.tsx  # 会话列表（按时间倒序）
│   │   │   ├── SessionCard.tsx  # 单条会话卡片
│   │   │   └── StatusIcon.tsx   # MenuBar 图标状态
│   │   ├── store/
│   │   │   └── sessions.ts      # zustand store
│   │   └── bindings/            # Wails 自动生成，不要手动编辑
│   └── package.json
├── app.go                       # Wails app 入口，注册 Service
├── service.go                   # SessionService（暴露给前端的 Bindings）
├── wails.json
├── go.mod
└── scripts/
    ├── install-hooks.sh         # 写入 ~/.claude/settings.json
    └── install-launchd.sh       # 注册 launchd LaunchAgent
```

---

## 3. 核心数据结构

### 3.1 AgentEvent（全项目唯一事件结构）

文件：`internal/event/types.go`

```go
package event

import (
    "encoding/json"
    "time"
)

// EventType 枚举
const (
    EventPreToolUse  = "pre_tool_use"
    EventPostToolUse = "post_tool_use"
    EventStop        = "stop"
    EventError       = "error"
    EventNotification = "notification" // CC Notification hook（v1.5）
)

// Agent 枚举
const (
    AgentClaudeCode = "claude-code"
    AgentCodex      = "codex" // [TODO-v1.5]
)

// SessionStatus 枚举
const (
    StatusWaiting  = "waiting"   // pre_tool_use 触发，尚未 post
    StatusActive   = "active"    // post_tool_use 收到，工具已执行
    StatusFinished = "finished"  // stop 收到
    StatusError    = "error"
)

// AgentEvent 是 bridge → server → UI 的唯一传输结构
// bridge 负责填充所有字段，server 和 UI 只读
type AgentEvent struct {
    // === 来源识别 ===
    Agent          string `json:"agent"`           // AgentClaudeCode | AgentCodex
    Host           string `json:"host"`            // "local" 或 SSH 主机名（[TODO-v1.5] 远程）

    // === 会话标识（三元组唯一确定一个会话） ===
    CWD            string `json:"cwd"`             // CC 的工作目录（绝对路径）
    TTY            string `json:"tty"`             // 控制终端，如 /dev/ttys003
    SessionID      string `json:"session_id"`      // CC 自带的 session_id（可能为空，辅助用）

    // === 终端定位（跳转用） ===
    TermProgram    string `json:"term_program"`    // 来自 $TERM_PROGRAM：iTerm.app | Apple_Terminal | WezTerm | ...
    ITermSessionID string `json:"iterm_session_id,omitempty"` // 来自 $ITERM_SESSION_ID，iTerm2 精确定位用

    // === 事件内容 ===
    EventType      string `json:"event_type"`      // EventPreToolUse 等
    ToolName       string `json:"tool_name"`       // Bash | Edit | Write | Read | ...
    ToolUseID      string `json:"tool_use_id"`     // 关联 pre/post，用于自动消除等待状态
    Content        string `json:"content"`         // 可读摘要，截断到 60 字符（UI 列表用）
    ContentRaw     string `json:"content_raw"`     // 完整内容（UI 详情/hover 用）

    // === 元数据 ===
    RawPayload     json.RawMessage `json:"raw_payload,omitempty"` // CC 原始 stdin，调试用
    Timestamp      time.Time       `json:"timestamp"`
}

// SessionKey 从 AgentEvent 提取会话唯一键
func (e *AgentEvent) SessionKey() string {
    // host:cwd:tty 三元组
    return e.Host + ":" + e.CWD + ":" + e.TTY
}
```

### 3.2 Session（Registry 内部状态）

文件：`internal/registry/registry.go`

```go
package registry

import (
    "sync"
    "time"
    "pager/internal/event"
)

// Session 代表一个活跃的 agent 会话
type Session struct {
    Key        string            // AgentEvent.SessionKey()
    Agent      string
    Host       string
    CWD        string
    TTY        string
    TermProgram string
    ITermSessionID string
    Status     string            // event.StatusWaiting | Active | Finished | Error
    LastEvent  *event.AgentEvent // 最近一次事件
    // 待消除队列：tool_use_id → AgentEvent（用于 pre/post 自动关联）
    PendingTools map[string]*event.AgentEvent
    UpdatedAt  time.Time
}

// Registry 线程安全的会话注册表
type Registry struct {
    mu       sync.RWMutex
    sessions map[string]*Session // key: Session.Key
    // 回调：状态变更时通知 Wails 主进程 emit event
    onChange func(sessions []*Session)
}

func New(onChange func([]*Session)) *Registry {
    return &Registry{
        sessions: make(map[string]*Session),
        onChange: onChange,
    }
}

// [IMPL] Apply 处理一个 AgentEvent，更新 Registry 状态
// 规则：
//   pre_tool_use  → upsert session，Status=waiting，加入 PendingTools[tool_use_id]
//   post_tool_use → 从 PendingTools 删除对应 tool_use_id；若 PendingTools 空 → Status=active
//   stop          → Status=finished
//   error         → Status=error
// 每次变更后调用 onChange
func (r *Registry) Apply(e *event.AgentEvent) {
    // [IMPL]
}

// ListSorted 返回按 UpdatedAt 倒序的会话列表（最新在前）
func (r *Registry) ListSorted() []*Session {
    // [IMPL]
}

// GetByTTY 根据 tty 查找会话（跳转用）
func (r *Registry) GetByTTY(tty string) (*Session, bool) {
    // [IMPL]
}
```

---

## 4. bridge（pager-cc-bridge）

### 4.1 职责与约束

- 被 CC hook 调用，stdin 收到 CC 的 hook JSON
- 提取环境信息 + Content → 构造 AgentEvent → POST 127.0.0.1:7421/event
- **任何错误（网络/解析/超时）必须静默，exit 0**，绝不影响 CC
- HTTP 超时硬设 1 秒
- 整个 bridge 进程必须在 2 秒内退出

### 4.2 CC hook stdin 真实 schema

```
// [PoC-VERIFIED] 编译可用；字段名以你的 CC 版本实测为准
// 调试：在 main() 首行加 os.WriteFile("/tmp/pager-raw.json", raw, 0644) 看真实 payload
```

文件：`internal/bridge/types.go`

```go
package bridge

import "encoding/json"

// CCHookInput 是 CC 通过 stdin 传给 hook 的结构
// 字段名来自 CC 官方文档 + PoC 实测，以实际 CC 版本为准
type CCHookInput struct {
    SessionID string          `json:"session_id"`
    CWD       string          `json:"cwd"`
    ToolName  string          `json:"tool_name"`
    ToolInput json.RawMessage `json:"tool_input"` // 各工具不同，见 §4.3
    ToolUseID string          `json:"tool_use_id"`
}

// BashInput tool_input for Bash
type BashInput struct {
    Command string `json:"command"`
}

// FileInput tool_input for Edit / Write / Read
type FileInput struct {
    FilePath string `json:"file_path"`
    // Edit 还有 OldStr/NewStr，不关心
}

// GlobInput tool_input for Glob
type GlobInput struct {
    Pattern string `json:"pattern"`
}

// GrepInput tool_input for Grep
type GrepInput struct {
    Pattern string `json:"pattern"`
}

// WebFetchInput tool_input for WebFetch
type WebFetchInput struct {
    URL string `json:"url"`
}

// WebSearchInput tool_input for WebSearch
type WebSearchInput struct {
    Query string `json:"query"`
}

// TaskInput tool_input for Task（subagent）
type TaskInput struct {
    Description string `json:"description"`
}
```

### 4.3 Content 提取规则

文件：`internal/bridge/extractor.go`

```go
package bridge

import (
    "encoding/json"
    "fmt"
    "strings"
)

const contentMaxRunes = 60

// ExtractContent 从工具名 + tool_input 提取可读摘要
// 返回 (contentRaw, content)
// content 是截断到 contentMaxRunes 的展示版
// contentRaw 是完整版
// [PoC-VERIFIED] 提取逻辑已验证，Bash/Edit 提取正常，60字符截断正常
func ExtractContent(toolName string, toolInput json.RawMessage) (contentRaw, content string) {
    raw := extractRaw(toolName, toolInput)
    raw = strings.TrimSpace(raw)
    if raw == "" {
        raw = toolName
    }
    return raw, truncateRunes(raw, contentMaxRunes)
}

func extractRaw(toolName string, toolInput json.RawMessage) string {
    unmarshal := func(v interface{}) bool {
        return json.Unmarshal(toolInput, v) == nil
    }

    switch toolName {
    case "Bash":
        var in BashInput
        if unmarshal(&in) && in.Command != "" {
            return in.Command
        }
    case "Edit":
        var in FileInput
        if unmarshal(&in) && in.FilePath != "" {
            return "编辑 " + in.FilePath
        }
    case "Write":
        var in FileInput
        if unmarshal(&in) && in.FilePath != "" {
            return "写入 " + in.FilePath
        }
    case "Read":
        var in FileInput
        if unmarshal(&in) && in.FilePath != "" {
            return "读取 " + in.FilePath
        }
    case "Glob":
        var in GlobInput
        if unmarshal(&in) && in.Pattern != "" {
            return "查找 " + in.Pattern
        }
    case "Grep":
        var in GrepInput
        if unmarshal(&in) && in.Pattern != "" {
            return "搜索 " + in.Pattern
        }
    case "WebFetch":
        var in WebFetchInput
        if unmarshal(&in) && in.URL != "" {
            return "抓取 " + in.URL
        }
    case "WebSearch":
        var in WebSearchInput
        if unmarshal(&in) && in.Query != "" {
            return "搜索 " + in.Query
        }
    case "Task":
        var in TaskInput
        if unmarshal(&in) && in.Description != "" {
            return "子任务: " + in.Description
        }
    }

    // MCP 工具：mcp__github__create_pr 等
    if strings.HasPrefix(toolName, "mcp__") {
        // 取最后一段作为可读名
        parts := strings.Split(toolName, "__")
        return fmt.Sprintf("MCP: %s", parts[len(parts)-1])
    }

    return toolName // 兜底
}

func truncateRunes(s string, n int) string {
    r := []rune(s)
    if len(r) <= n {
        return s
    }
    return string(r[:n]) + "…"
}
```

### 4.4 bridge 主程序

文件：`cmd/bridge/main.go`

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "os/exec"
    "strings"
    "time"

    "pager/internal/bridge"
    "pager/internal/event"
    // poster 见 §4.5
)

// [PoC-VERIFIED] 整体结构已验证
func main() {
    // 核心保障：无论发生什么，exit 0
    defer os.Exit(0)

    eventType := "unknown"
    if len(os.Args) > 1 {
        eventType = os.Args[1] // pre_tool_use | post_tool_use | stop
    }

    raw, err := io.ReadAll(os.Stdin)
    if err != nil {
        return
    }

    // [IMPL] 调试模式：PAGER_DEBUG=1 时写原始 payload 到 /tmp/pager-raw.json
    if os.Getenv("PAGER_DEBUG") == "1" {
        _ = os.WriteFile("/tmp/pager-raw.json", raw, 0644)
    }

    var in bridge.CCHookInput
    _ = json.Unmarshal(raw, &in) // 失败也继续，尽量上报

    contentRaw, content := bridge.ExtractContent(in.ToolName, in.ToolInput)

    e := event.AgentEvent{
        Agent:          event.AgentClaudeCode,
        Host:           "local",
        SessionID:      in.SessionID,
        CWD:            firstNonEmpty(in.CWD, os.Getenv("PWD")),
        TTY:            detectTTY(),
        TermProgram:    os.Getenv("TERM_PROGRAM"),
        ITermSessionID: os.Getenv("ITERM_SESSION_ID"),
        EventType:      eventType,
        ToolName:       in.ToolName,
        ToolUseID:      in.ToolUseID,
        Content:        content,
        ContentRaw:     contentRaw,
        RawPayload:     raw,
        Timestamp:      time.Now(),
    }

    bridge.PostEvent(e) // 1 秒超时，失败静默
}

// [PoC-VERIFIED] tty 检测
func detectTTY() string {
    out, err := exec.Command("tty").Output()
    if err == nil {
        t := strings.TrimSpace(string(out))
        if t != "" && t != "not a tty" {
            return t
        }
    }
    return ""
}

func firstNonEmpty(vals ...string) string {
    for _, v := range vals {
        if v != "" {
            return v
        }
    }
    return ""
}

// 避免 unused import
var _ = fmt.Sprintf
```

### 4.5 HTTP poster

文件：`internal/bridge/poster.go`

```go
package bridge

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "time"

    "pager/internal/event"
)

const serverURL = "http://127.0.0.1:7421/event"

// PostEvent 向 Pager server 推送事件
// 超时 1 秒，失败静默（写 stderr，不影响 CC）
// [PoC-VERIFIED]
func PostEvent(e event.AgentEvent) {
    body, err := json.Marshal(e)
    if err != nil {
        return
    }
    client := &http.Client{Timeout: 1 * time.Second}
    req, err := http.NewRequest(http.MethodPost, serverURL, bytes.NewReader(body))
    if err != nil {
        return
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := client.Do(req)
    if err != nil {
        // Pager 未启动是正常情况，静默
        fmt.Fprintf(os.Stderr, "[pager-bridge] warn: %v\n", err)
        return
    }
    defer resp.Body.Close()
}
```

---

## 5. HTTP Server

文件：`internal/server/server.go`

```go
package server

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "pager/internal/event"
    "pager/internal/registry"
)

const ListenAddr = "127.0.0.1:7421"

type Server struct {
    reg *registry.Registry
}

func New(reg *registry.Registry) *Server {
    return &Server{reg: reg}
}

// Start 启动 HTTP server，在独立 goroutine 中运行
// 由 Wails app.go 在 OnStartup 中调用
func (s *Server) Start() {
    mux := http.NewServeMux()
    mux.HandleFunc("/event", s.handleEvent)
    mux.HandleFunc("/sessions", s.handleSessions) // 调试用
    go func() {
        log.Printf("[pager-server] 监听 %s", ListenAddr)
        if err := http.ListenAndServe(ListenAddr, mux); err != nil {
            log.Printf("[pager-server] 错误: %v", err)
        }
    }()
}

func (s *Server) handleEvent(w http.ResponseWriter, req *http.Request) {
    if req.Method != http.MethodPost {
        http.Error(w, "POST only", http.StatusMethodNotAllowed)
        return
    }
    var e event.AgentEvent
    if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // [IMPL] 交给 Registry 处理状态变更
    // Registry.Apply 内部会在状态变更时调用 onChange 回调
    // onChange 由 app.go 注入，负责 EmitEvent 到前端 + 触发系统通知
    s.reg.Apply(&e)

    w.WriteHeader(http.StatusOK)
    fmt.Fprintln(w, "ok")
}

// handleSessions 仅用于调试，生产可保留
func (s *Server) handleSessions(w http.ResponseWriter, req *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(s.reg.ListSorted())
}
```

---

## 6. 系统通知

文件：`internal/notify/notify.go`

```go
package notify

import (
    "os/exec"
    "strings"
    "fmt"
    "pager/internal/event"
)

// MVP 阶段用 osascript 实现
// [TODO-v1.5] 换成 UNUserNotificationCenter（CGO）
//   原因：UNUserNotification 支持 Action Button（点击不跳转到 App，直接通知上操作）
//   迁移条件：app 签名后再做，未签名 app 的 UNUserNotification 行为不可靠

// Show 根据事件类型弹系统通知
// [PoC-VERIFIED] osascript 通知在 macOS 可用
func Show(e *event.AgentEvent) {
    var title, body string
    switch e.EventType {
    case event.EventPreToolUse:
        title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
        body = e.Content
        if body == "" {
            body = "等待确认: " + e.ToolName
        }
    case event.EventStop:
        title = fmt.Sprintf("%s · %s", agentLabel(e.Agent), lastPath(e.CWD))
        body = "任务完成"
    case event.EventError:
        title = fmt.Sprintf("%s · %s [错误]", agentLabel(e.Agent), lastPath(e.CWD))
        body = e.Content
    default:
        return // 其他事件不弹通知
    }
    showOsascript(title, body)
}

func showOsascript(title, body string) {
    title = sanitize(title)
    body = sanitize(body)
    script := fmt.Sprintf(
        `display notification "%s" with title "Pager" subtitle "%s" sound name "Tink"`,
        body, title,
    )
    _ = exec.Command("osascript", "-e", script).Run()
}

func sanitize(s string) string {
    // 转义 AppleScript 内的双引号和反斜杠
    s = strings.ReplaceAll(s, `\`, `\\`)
    s = strings.ReplaceAll(s, `"`, `\"`)
    // 截断过长内容（通知 body 太长会被系统截断，不如我们先截）
    r := []rune(s)
    if len(r) > 100 {
        s = string(r[:100]) + "…"
    }
    return s
}

func agentLabel(agent string) string {
    switch agent {
    case event.AgentClaudeCode:
        return "Claude Code"
    case event.AgentCodex:
        return "Codex"
    default:
        return agent
    }
}

func lastPath(p string) string {
    parts := strings.Split(strings.TrimRight(p, "/"), "/")
    if len(parts) == 0 || p == "" {
        return p
    }
    return parts[len(parts)-1]
}
```

---

## 7. 终端跳转

文件：`internal/terminal/jump.go`

```go
package terminal

import (
    "fmt"
    "log"
    "os/exec"
    "strings"
)

// JumpRequest 跳转请求
type JumpRequest struct {
    TTY            string
    TermProgram    string
    ITermSessionID string // 优先用，比 tty 更稳定
}

// Jump 根据 JumpRequest 前台化对应终端 tab
// [PoC-VERIFIED] iTerm2 和 Terminal.app 的基本结构已验证
// 注意：首次调用会触发 macOS Automation 授权弹窗，需在 UI 引导流程中提前请求
func Jump(req JumpRequest) error {
    switch {
    case strings.Contains(req.TermProgram, "iTerm"):
        return jumpITerm(req)
    case req.TermProgram == "Apple_Terminal":
        return jumpAppleTerminal(req.TTY)
    case strings.Contains(req.TermProgram, "WezTerm"):
        return jumpWezTerm(req.TTY) // [TODO-v1.5]
    default:
        // 降级：只 activate app
        return activateApp(req.TermProgram)
    }
}

// jumpITerm 优先用 ITermSessionID（精确），fallback 用 tty
// [PoC-VERIFIED] 基本 AppleScript 结构可用，需实机验证 tty of session 属性
func jumpITerm(req JumpRequest) error {
    var script string
    if req.ITermSessionID != "" {
        // ITermSessionID 格式：w0t1p0:GUID
        // 可用 unique ID 精确定位
        script = fmt.Sprintf(`
tell application "iTerm2"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      repeat with s in sessions of t
        if (unique id of s) contains "%s" then
          select w
          tell t to select
          tell s to select
          return
        end if
      end repeat
    end repeat
  end repeat
end tell`, req.ITermSessionID)
    } else {
        // fallback：用 tty 匹配
        script = fmt.Sprintf(`
tell application "iTerm2"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      repeat with s in sessions of t
        if (tty of s) is "%s" then
          select w
          tell t to select
          tell s to select
          return
        end if
      end repeat
    end repeat
  end repeat
end tell`, req.TTY)
    }
    return runOsa(script)
}

// jumpAppleTerminal 通过 tty 定位 tab
// [PoC-VERIFIED] 结构可用，需实机验证 tty of tab 在当前 macOS 版本的行为
func jumpAppleTerminal(tty string) error {
    script := fmt.Sprintf(`
tell application "Terminal"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      if (tty of t) is "%s" then
        set selected of t to true
        set index of w to 1
        return
      end if
    end repeat
  end repeat
end tell`, tty)
    return runOsa(script)
}

func jumpWezTerm(tty string) error {
    // [TODO-v1.5] wezterm cli list → 查 pane_id by tty → wezterm cli activate-pane
    return activateApp("WezTerm")
}

func activateApp(termProgram string) error {
    app := strings.TrimSuffix(termProgram, ".app")
    if app == "" {
        app = "Terminal"
    }
    return runOsa(fmt.Sprintf(`tell application "%s" to activate`, app))
}

func runOsa(script string) error {
    cmd := exec.Command("osascript", "-e", script)
    out, err := cmd.CombinedOutput()
    if err != nil {
        log.Printf("[terminal] osascript error: %v, output: %s", err, strings.TrimSpace(string(out)))
    }
    return err
}
```

---

## 8. Wails 主程序

### 8.1 app.go

```go
package main

import (
    "context"

    "pager/internal/event"
    "pager/internal/notify"
    "pager/internal/registry"
    "pager/internal/server"

    "github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
    app *application.App
    reg *registry.Registry
    srv *server.Server
}

func NewApp() *App {
    return &App{}
}

// OnStartup 在 Wails 主进程启动时调用
// 启动 HTTP server，注册 Registry onChange 回调
func (a *App) OnStartup(ctx context.Context, options application.ServiceOptions) error {
    a.reg = registry.New(func(sessions []*registry.Session) {
        // Registry 状态变更时：
        // 1. 推送到前端 UI
        a.app.EmitEvent("sessions-updated", sessions)
        // 2. 对最新事件触发系统通知
        // [IMPL] 找到最新的 waiting 状态会话，调 notify.Show
    })

    a.srv = server.New(a.reg)
    a.srv.Start()
    return nil
}

// OnShutdown 清理
func (a *App) OnShutdown(ctx context.Context) {
    // [IMPL] 可选：保存会话历史到 SQLite（v1.5）
}
```

### 8.2 main.go（Wails v3 入口）

```go
package main

import (
    "embed"
    "log"

    "pager/service" // SessionService

    "github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
    app := application.New(application.Options{
        Name:        "Pager",
        Description: "AI coding agents 状态感知层",
        Services: application.Services{
            application.NewService(&service.SessionService{}),
        },
        Assets: application.AssetOptions{
            FS: assets,
        },
    })

    // MenuBar Tray
    tray := app.NewSystemTray()
    tray.SetIcon(grayIcon) // [IMPL] 加载内嵌图标资源

    // 弹出窗口（点击 tray icon 时显示）
    window := app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
        Title:  "Pager",
        Width:  400,
        Height: 600,
        Hidden: true,
        // 无边框，macOS 风格浮动面板
        Frameless:         true,
        AlwaysOnTop:       true,
        BackgroundColour:  application.NewRGBA(0, 0, 0, 0),
    })
    tray.AttachWindow(window) // Wails v3：点击图标自动在 icon 位置弹出

    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}
```

### 8.3 service.go（暴露给前端的 Bindings）

```go
package service

import (
    "pager/internal/registry"
    "pager/internal/terminal"
)

// SessionService 暴露给 React 前端的所有操作
// Wails 自动生成 frontend/src/bindings/ 下的 TypeScript 桩代码
type SessionService struct {
    reg *registry.Registry
}

// ListSessions 返回当前所有会话（前端初始化 + 手动刷新用）
// 实时更新用 Wails Events（sessions-updated），不需要轮询
func (s *SessionService) ListSessions() []*registry.Session {
    return s.reg.ListSorted()
}

// JumpToTerminal 跳转到指定会话对应的终端 tab
func (s *SessionService) JumpToTerminal(sessionKey string) error {
    // [IMPL]
    // 1. 从 reg 找到 session
    // 2. 构造 terminal.JumpRequest
    // 3. 调 terminal.Jump
    return nil
}

// DismissSession 手动标记会话为已读/关闭
func (s *SessionService) DismissSession(sessionKey string) {
    // [IMPL] 从 registry 移除或标记 dismissed
}
```

---

## 9. 前端规格

### 9.1 技术栈

```
React 18 + TypeScript
Tailwind CSS（仅核心 utility class，不用插件）
zustand（会话状态管理）
Wails v3 bindings（自动生成，不手动维护）
```

### 9.2 zustand store

文件：`frontend/src/store/sessions.ts`

```typescript
import { create } from 'zustand'
import { Events } from '@wailsio/runtime'
import { SessionService } from '../bindings/service'

export type SessionStatus = 'waiting' | 'active' | 'finished' | 'error'

export interface Session {
  Key: string
  Agent: string        // "claude-code" | "codex"
  Host: string
  CWD: string
  TTY: string
  TermProgram: string
  ITermSessionID: string
  Status: SessionStatus
  Content: string      // 截断版（60字符）
  ContentRaw: string   // 完整版
  ToolName: string
  UpdatedAt: string    // ISO 时间字符串
}

interface SessionStore {
  sessions: Session[]
  setSessions: (sessions: Session[]) => void
}

export const useSessionStore = create<SessionStore>((set) => ({
  sessions: [],
  setSessions: (sessions) =>
    // 按 UpdatedAt 倒序（最新在上）
    set({ sessions: [...sessions].sort(
      (a, b) => new Date(b.UpdatedAt).getTime() - new Date(a.UpdatedAt).getTime()
    )}),
}))

// 初始化：订阅 Go 推送的 sessions-updated 事件
export function initSessionSync() {
  // 初始加载
  SessionService.ListSessions().then((sessions) => {
    useSessionStore.getState().setSessions(sessions ?? [])
  })

  // 实时推送
  Events.On('sessions-updated', (event: { data: Session[] }) => {
    useSessionStore.getState().setSessions(event.data ?? [])
  })
}
```

### 9.3 SessionCard 组件规格

文件：`frontend/src/components/SessionCard.tsx`

```
每张卡片显示：
┌─────────────────────────────────────────────────────┐
│ [icon] Agent名 · cwd末段           状态标签  时间    │
│ Content（截断60字符）                        [跳转]  │
│ [展开] ContentRaw（hover 或点击展开）                │
└─────────────────────────────────────────────────────┘

Agent icon 规则（纯色圆点）：
  claude-code → 蓝色 #5B9BD5
  codex       → 绿色 #4CAF50
  unknown     → 灰色 #9E9E9E

状态标签颜色：
  waiting  → 红色背景  "等待 Ns"（显示等待秒数，每秒更新）
  active   → 蓝色背景  "执行中"
  finished → 灰色背景  "已完成"
  error    → 橙色背景  "错误"

时间：相对时间，如 "刚刚" / "2分钟前" / "10:32"

跳转按钮：
  调用 SessionService.JumpToTerminal(session.Key)
  loading 状态：按钮 spinner
  失败：Toast 提示"跳转失败，请检查终端权限"

ContentRaw 展开：
  默认折叠，点击 Content 行展开
  展开后显示完整文本，等宽字体（font-mono）
  Bash 命令：代码高亮（简单的 keyword 高亮即可，不引入大库）
```

### 9.4 MenuBar 状态图标规则

```
空闲（无活跃会话）      → 灰色图标
有活跃会话（非等待）    → 蓝色图标
有等待中会话            → 红色图标 + 角标（数字）

图标切换由 Go 侧在 Registry.Apply 后调用 tray.SetIcon 实现
不由前端控制（前端在 WebView 里，无法直接操作 tray icon）
```

---

## 10. CC Hook 配置

文件：`scripts/install-hooks.sh`

```bash
#!/bin/bash
# 安装 pager-cc-bridge hook 到 ~/.claude/settings.json
# 用法：./install-hooks.sh /path/to/pager-cc-bridge

set -e

BRIDGE_PATH="${1:?用法: $0 /path/to/pager-cc-bridge}"

if [ ! -x "$BRIDGE_PATH" ]; then
  echo "错误: $BRIDGE_PATH 不存在或不可执行"
  exit 1
fi

SETTINGS="$HOME/.claude/settings.json"

# 检查 CC 版本是否支持 async: true（需要 CC >= 某版本，具体版本待实测后补充）
CC_VERSION=$(claude --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1 || echo "0")
echo "CC 版本: $CC_VERSION"

# [IMPL] 用 jq 合并写入 hooks 配置，不覆盖现有其他 hooks
# 关键：全部用 async: true，不阻塞 CC

HOOK_CONFIG=$(cat <<EOF
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH pre_tool_use",
        "async": true,
        "timeout": 5
      }]
    }],
    "PostToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH post_tool_use",
        "async": true,
        "timeout": 5
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "$BRIDGE_PATH stop",
        "async": true,
        "timeout": 5
      }]
    }]
  }
}
EOF
)

# 备份原配置
[ -f "$SETTINGS" ] && cp "$SETTINGS" "$SETTINGS.pager-backup"

# 合并写入
if [ -f "$SETTINGS" ]; then
  jq -s '.[0] * .[1]' "$SETTINGS" <(echo "$HOOK_CONFIG") > "$SETTINGS.tmp" && mv "$SETTINGS.tmp" "$SETTINGS"
else
  mkdir -p "$(dirname $SETTINGS)"
  echo "$HOOK_CONFIG" > "$SETTINGS"
fi

echo "✓ Hook 已安装到 $SETTINGS"
echo "  重启 Claude Code 后生效"
```

---

## 11. launchd 自启

文件：`scripts/install-launchd.sh`

```bash
#!/bin/bash
# 注册 Pager 为 launchd LaunchAgent，实现开机自启 + 崩溃重启

APP_PATH="${1:?用法: $0 /Applications/Pager.app}"
PLIST="$HOME/Library/LaunchAgents/com.sapaude.pager.plist"

cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.sapaude.pager</string>
    <key>ProgramArguments</key>
    <array>
        <string>$APP_PATH/Contents/MacOS/Pager</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/.pager/pager.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/.pager/pager-error.log</string>
</dict>
</plist>
EOF

launchctl load "$PLIST"
echo "✓ Pager LaunchAgent 已注册"
```

---

## 12. PoC 验证结论（已知可行 / 已知待验证）

### 已验证（PoC 环境 + 容器编译）

| 项目 | 结论 |
|------|------|
| bridge 编译 | ✅ Go 1.22 编译通过，无依赖 |
| server 编译 | ✅ Go 1.22 编译通过，无依赖 |
| bridge stdin → Content 提取 | ✅ Bash/Edit 提取正确，60字符截断正常 |
| bridge → server HTTP POST | ✅ 端到端通，事件正确解析分流 |
| /sessions API | ✅ 返回正确 |
| osascript 通知调用 | ✅ macOS 上工作（容器预期失败） |
| tty 检测（`tty` 命令） | ✅ 在真实终端中工作；容器中 tty 为空是正常的 |
| 整体链路耐异常性 | ✅ server 不启动时 bridge 1秒超时静默退出，不阻塞 |

### 需在 Mac 上实机验证（M1 PoC 步骤 4-5）

| 项目 | 风险等级 | 说明 |
|------|----------|------|
| iTerm2 `tty of session` AppleScript 属性 | 中 | 需确认当前 iTerm2 版本可用；失败则 fallback 用 ITermSessionID |
| iTerm2 `unique id of session` 定位 | 中 | ITermSessionID 格式需实测 |
| Terminal.app `tty of tab` | 中 | 历史上有版本差异 |
| CC 真实 hook stdin 字段名 | 高 | `cwd`/`tool_name`/`tool_input`/`tool_use_id` 以实测为准；用 PAGER_DEBUG=1 看原始 payload |
| `async: true` 当前 CC 版本支持 | 高 | 若不支持，bridge 走同步路径，需验证 1s 超时是否够用 |
| macOS Automation 授权弹窗 | 低 | 首次运行会弹，正常用户授权流程 |
| App Store 沙盒：端口监听 + osascript | 高 | 需单独 spike，可能需要走 Developer ID 分发 |

### 待实测后填写

```
CC 版本：___________
async: true 支持：是 / 否（若否，降级方案：___________）
CC hook stdin 真实字段（PAGER_DEBUG=1 输出）：

{
  // 粘贴 /tmp/pager-raw.json 内容
}

iTerm2 tty 跳转：成功 / 失败（失败原因：___________）
Terminal.app tty 跳转：成功 / 失败
```

---

## 13. 开发里程碑

| 里程碑 | 任务 | 完成标准 |
|--------|------|----------|
| **M1** PoC 链路验证 | 见 `pager-poc/README.md` | 验证清单全打勾；实测结论填入 §12 |
| **M2** Wails v3 基础框架 | `main.go` + `app.go` + tray + window attach | 点击 tray icon 弹出空白面板 |
| **M3** 会话列表 UI | SessionCard + zustand store + Wails Events | 模拟事件后列表实时更新 |
| **M4** 跳转接通 | SessionService.JumpToTerminal + terminal.Jump | 点列表跳转按钮，iTerm2/Terminal.app 前台化 |
| **M5** 系统通知 | notify.Show + 通知点击唤起面板 | PreToolUse 触发弹通知 |
| **M6** 状态自动消除 | Registry pre/post tool_use_id 关联 | PostToolUse 收到后等待状态消除 |
| **M7** 打包签名 | Wails build + notarize | .app 可在其他 Mac 安装运行 |
| **M8** LaunchAgent | install-launchd.sh | 重启 Mac 后 Pager 自启 |

---

## 14. 不做的事（硬边界）

以下功能在 v1.0 **不实现**，如果在开发过程中有"顺便做了"的冲动，拒绝：

- hook 返回值影响 CC 行为（approve / deny / defer）
- Pager 内文本回复 CC
- 多机器统一视图
- Codex / 其他 agent 接入（AgentEvent 结构预留了字段，但 bridge 不写）
- SQLite 持久化（内存 Registry 足够）
- WebSocket（Wails Events 已满足 Go → UI 推送需求）
- Linux / Windows 支持

