# VibeCoding Pager Architecture & Directory Convention

本文档定义 VibeCoding Pager 项目的目录规范和架构约定，适用于所有 Go + Wails v3 开发。

## 四象限分层架构

```
internal/
├── domain/       领域层：纯业务规则 + 实体（零框架依赖）
├── adapter/      适配层：外部系统交互（OS API、HTTP、CLI）
├── infra/        基础设施：技术通用组件（日志、配置、存储）
└── wails/        呈现层：Wails UI 框架绑定
```

### 依赖规则

| 层 | 可以 import | 不可以 import |
|---|------------|--------------|
| `domain/` | 标准库 only | internal 下任何其他包 |
| `infra/` | 标准库 | domain, adapter, wails |
| `adapter/` | domain + infra + 标准库 | wails |
| `wails/` | 所有层 | — |

**违反依赖方向 = 架构退化，必须修复。**

## 目录职责

### `internal/domain/` — 领域层

纯业务逻辑，不依赖任何框架或基础设施。应当可以独立编译和测试。

```
domain/
├── entity/           公共领域实体（跨模块共享的 struct/常量，如 AgentEvent）
├── session/          Agent 会话管理（Registry 状态机）
└── usage/            用量统计（v1.5，按需新增）
```

**`entity/` 用途**：当多个业务模块需要共享相同的数据结构时（如 `AgentEvent` 被 session、usage、notify 共同使用），放入 `domain/entity/` 避免循环依赖。

**新增业务模块时**：在 `domain/` 下平铺新目录即可。

### `internal/adapter/` — 适配层

与外部系统的交互点。每个子目录对应一个外部系统。

```
adapter/
├── httpapi/          Inbound:  HTTP server（接收 bridge POST 的 AgentEvent）
├── notify/           Outbound: macOS 系统通知（osascript）
├── terminal/         Outbound: 终端 Tab 跳转（AppleScript）
└── bridge/           Shared:   CC hook 事件解析逻辑（供 cmd/bridge 使用）
```

**方向标识**：
- Inbound adapter = 外部调用进来（httpapi）
- Outbound adapter = 我们调用外部（notify, terminal）
- Shared = 供 cmd/ 入口使用的共享逻辑（bridge）

### `internal/infra/` — 基础设施层

通用技术组件，无业务语义。被所有层使用。

```
infra/
├── log/              slog 初始化 + Module logger 工厂
├── config/           JSON 配置文件持久化（Settings struct）
└── store/            数据存储抽象（interface，v1.5 实现 SQLite）
```

### `internal/wails/` — 框架适配层

所有与 Wails v3 框架直接耦合的代码。

```
wails/
├── app.go            PagerApp: 组装 + lifecycle + event emission
├── hotkey.go         HotkeyManager: 全局快捷键注册
├── session_svc.go    SessionBinding: 前端会话操作绑定
├── settings_svc.go   SettingsBinding: 前端设置操作绑定
└── assets/           Go embed 资源（tray icon PNG）
```

## 命名规范

### 文件命名

| 层 | 模式 | 示例 |
|---|------|------|
| wails 绑定 | `*_svc.go` | `session_svc.go` |
| wails 管理器 | 功能名.go | `hotkey.go` |
| adapter/domain/infra | 功能名.go | `registry.go`, `notify.go` |
| 测试 | `*_test.go` | `registry_test.go` |

### Struct 命名

| 层 | 模式 | 示例 |
|---|------|------|
| wails 绑定 | `XxxBinding` | `SessionBinding` |
| wails 管理器 | `XxxManager` | `HotkeyManager` |
| domain 实体 | 领域术语 | `Registry`, `Session`, `AgentEvent` |
| infra | 技术术语 | `Settings`, `Store` |

### Import 别名

当包名冲突时使用别名：

```go
import (
    infralog "pager/internal/infra/log"      // 避免与 slog 冲突
    "pager/internal/domain/session"
)
```

## 根目录文件

| 文件 | 用途 | 是否 git 跟踪 |
|------|------|--------------|
| `main.go` | Wails 入口（仅组装 + Run） | ✓ |
| `Makefile` | 开发命令 | ✓ |
| `go.mod` / `go.sum` | Go 模块 | ✓ |
| `wails.json` | Wails 配置 | ✓ |
| `Taskfile.yml` | wails3 内部依赖 | ✓ |
| `CLAUDE.md` | AI 协作指令 | ✓ |
| `ARCHITECTURE.md` | 本文档 | ✓ |
| `.gitignore` | Git 忽略规则 | ✓ |

**禁止出现在根目录的**：
- Go 业务代码文件（`app.go`, `service.go` 等）
- 编译产物（`bridge`, `*.app`, `bin/`）
- IDE 配置（`.idea/`, `.vscode/`）

## cmd/ 约定

每个子目录是一个可独立编译的 CLI binary：

```
cmd/
├── bridge/       vibecoding-pager-cc-bridge（hook 使用，必须 exit 0）
└── icongen/      图标生成器（开发工具）
```

## frontend/ 约定

遵循 React + TypeScript + Tailwind 标准结构：

```
frontend/src/
├── assets/       静态资源（UI 用的图片/SVG）
├── components/   通用 UI 组件
├── pages/        路由页面
├── store/        zustand 状态管理
├── i18n/         多语言翻译文件
└── bindings/     Wails 自动生成（勿手动编辑）
```

## 新增模块 Checklist

添加新业务模块时：

1. 确定所属层（domain? adapter? infra? wails?）
2. 在对应目录下创建子目录
3. 确认依赖方向合规（domain 不 import 外层）
4. 添加 `_test.go` 单元测试
5. 更新本文档的目录树（如有必要）

## 技术选型

| 组件 | 选择 | 理由 |
|------|------|------|
| 日志 | `log/slog`（Go 1.21+ 内置） | 标准库优先，结构化，无外部依赖 |
| 配置 | JSON 文件 (`~/.config/pager/`) | 简单可靠 |
| 存储 | v1 内存 / v1.5 SQLite | 渐进增强 |
| 热键 | `golang.design/x/hotkey` | 唯一支持 macOS 的纯 Go 方案 |
| 前端状态 | zustand | 轻量、类型安全 |
| 多语言 | i18next | React 生态标准 |
