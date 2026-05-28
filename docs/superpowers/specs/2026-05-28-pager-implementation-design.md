# Pager Implementation Design

> Date: 2026-05-28
> Source: CLAUDE.md (技术 PRD v1.0)
> Strategy: 分层验证推进

## Context

Pager 是一个 macOS MenuBar 应用，作为 AI coding agents（当前仅 Claude Code）的状态感知层。核心链路：

```
CC hook → bridge (stdin→HTTP) → server → registry → UI + 通知
```

PRD 已完整定义所有数据结构、文件路径、函数签名和实现规则。本文档不重复 PRD 内容，只记录实施顺序和验证标准。

## Architecture (from PRD, not modified)

- **bridge** (`cmd/bridge/main.go`): 独立可执行文件，被 CC hook 调用，1秒超时 fire-and-forget
- **server** (`internal/server/`): HTTP server on :7421，接收 AgentEvent
- **registry** (`internal/registry/`): 内存状态机，管理 Session 生命周期
- **notify** (`internal/notify/`): osascript 系统通知
- **terminal** (`internal/terminal/`): osascript 终端跳转
- **Wails app** (root `main.go`, `app.go`, `service.go`): 单进程 MenuBar 应用
- **frontend** (`frontend/`): React + Tailwind + zustand

## Implementation Layers

### Layer 1: Pure Go Backend (可在任何环境编译验证)

**目标**: bridge + server + registry + notify + terminal 全部编译通过

文件清单:
1. `go.mod` — module pager, Go 1.24
2. `internal/event/types.go` — AgentEvent 结构 + 常量
3. `internal/bridge/types.go` — CCHookInput + 各工具 Input 结构
4. `internal/bridge/extractor.go` — ExtractContent 逻辑
5. `internal/bridge/poster.go` — PostEvent HTTP 客户端
6. `internal/registry/registry.go` — Registry + Session + Apply/ListSorted/GetByTTY
7. `internal/server/server.go` — HTTP server
8. `internal/notify/notify.go` — osascript 通知
9. `internal/terminal/jump.go` — 终端跳转
10. `cmd/bridge/main.go` — bridge 主程序

**验证标准**: `go build ./...` 零错误

### Layer 2: Wails App Shell (需要 Wails v3 SDK)

**目标**: main.go + app.go + service.go 编译通过（依赖 Wails v3 模块）

文件清单:
1. `app.go` — App struct + OnStartup + OnShutdown
2. `service.go` — SessionService (暴露给前端的 Bindings)
3. `main.go` — Wails v3 入口，tray + window

**验证标准**: `go build .` 通过（需 wails v3 依赖可用）

### Layer 3: Frontend (需要 Node.js)

**目标**: React 应用编译通过，组件结构就位

文件清单:
1. `frontend/package.json` — 依赖声明
2. `frontend/tsconfig.json`
3. `frontend/tailwind.config.js`
4. `frontend/index.html`
5. `frontend/src/main.tsx` — 入口
6. `frontend/src/App.tsx`
7. `frontend/src/store/sessions.ts` — zustand store
8. `frontend/src/components/SessionList.tsx`
9. `frontend/src/components/SessionCard.tsx`
10. `frontend/src/components/StatusIcon.tsx`

**验证标准**: `npm run build` 零错误

### Layer 4: Scripts & Config

1. `scripts/install-hooks.sh`
2. `scripts/install-launchd.sh`
3. `wails.json`

## Key Implementation Details (from PRD)

### Registry.Apply 状态机规则:
- `pre_tool_use` → upsert session, Status=waiting, 加入 PendingTools[tool_use_id]
- `post_tool_use` → 从 PendingTools 删除 tool_use_id; 若 PendingTools 空 → Status=active
- `stop` → Status=finished
- `error` → Status=error
- 每次变更后调用 onChange 回调

### Bridge 约束:
- 任何错误静默, exit 0
- HTTP 超时 1 秒
- 整个进程 2 秒内退出

### 通知规则:
- pre_tool_use → 弹通知（标题: Agent · cwd末段, 内容: Content）
- stop → 弹通知（"任务完成"）
- error → 弹通知
- post_tool_use → 不弹通知

## Constraints (from PRD, non-negotiable)

1. hook fire-and-forget, 不等响应, 不阻塞 agent
2. 不在 Pager 内做 approve/deny
3. 单进程架构 (Daemon + UI 在 Wails 主进程)
4. AgentEvent 是唯一跨层数据结构

## Out of Scope (v1.0)

- hook 返回值影响 CC
- 多机器视图
- Codex/其他 agent 接入
- SQLite 持久化
- WebSocket
- Linux/Windows
