# Pager 项目重构设计 Spec

Date: 2026-05-29

## Overview

将 Pager 项目从当前"根目录堆积"状态重构为四象限分层架构（Domain / Adapter / Infrastructure / Wails），建立清晰的职责边界、统一的命名规范和可扩展的目录结构。

## 目标

1. 根目录仅保留 `main.go`（Wails 入口）+ 配置文件
2. Go 代码按四层组织：`domain/` → `adapter/` → `infra/` → `wails/`
3. 依赖方向严格单向：外层 → 内层（domain 零外部依赖）
4. 引入 `slog` 结构化日志替代 `log.Printf`
5. 为未来特性（usage 统计、SQLite 存储）预留位置
6. 清理编译产物和冗余文件

## 架构设计

### 四象限分层

```
internal/
├── domain/       纯业务规则 + 领域实体（零框架依赖）
├── adapter/      外部系统适配器（OS API、HTTP、CLI）
├── infra/        技术基础设施（日志、配置、存储）
└── wails/        UI 框架绑定层（Wails 生命周期 + 前端暴露）
```

### 依赖方向

```
        ┌─────────┐
        │  wails  │ ← 呈现层（最外层）
        └────┬────┘
             │ depends on
        ┌────▼────┐
        │ adapter │ ← 适配层
        └────┬────┘
             │ depends on
        ┌────▼────┐
        │ domain  │ ← 领域层（最内层，零 import）
        └─────────┘
             ▲
        ┌────┴────┐
        │  infra  │ ← 基础设施（被所有层使用）
        └─────────┘
```

**规则**：
- `domain/` 不 import `internal/` 下的任何其他包
- `infra/` 不 import `domain/`、`adapter/`、`wails/`
- `adapter/` 可 import `domain/` + `infra/`
- `wails/` 可 import 所有层

### 完整目录结构

```
pager/
├── main.go                              # 入口：Wails app 组装 + Run()
├── Makefile                             # 开发生命周期命令
├── go.mod
├── go.sum
├── wails.json                           # Wails 项目配置
├── CLAUDE.md                            # AI 协作指令
├── ARCHITECTURE.md                      # 目录规范文档（本次新增）
├── .gitignore
│
├── internal/
│   ├── domain/                          # 【领域层】纯业务逻辑
│   │   ├── entity/                      #   公共领域实体（跨模块共享的 struct/常量）
│   │   │   └── event.go                #     AgentEvent struct + EventType/Status/Attention 常量
│   │   └── session/                     #   Agent 会话管理
│   │       ├── registry.go              #     Registry 状态机 + onChange callback
│   │       └── registry_test.go         #     单元测试
│   │
│   ├── adapter/                         # 【适配层】外部系统交互
│   │   ├── httpapi/                     #   Inbound: HTTP server 接收 bridge 事件
│   │   │   ├── server.go               #     HTTP handler + Start()
│   │   │   └── server_test.go
│   │   ├── notify/                      #   Outbound: macOS 系统通知
│   │   │   └── notify.go               #     ShowFull + ShouldNotify + 多语言文案
│   │   ├── terminal/                    #   Outbound: 终端 Tab 跳转
│   │   │   └── jump.go                 #     iTerm2 / Terminal.app AppleScript
│   │   └── bridge/                      #   Shared: CC hook 事件解析
│   │       ├── extractor.go            #     工具内容提取
│   │       ├── attention.go            #     AttentionLevel 判定规则
│   │       ├── attention_test.go
│   │       ├── extractor_test.go
│   │       ├── poster.go              #     HTTP POST client
│   │       └── types.go               #     CCHookInput + tool input structs
│   │
│   ├── infra/                           # 【基础设施层】通用技术组件
│   │   ├── log/                         #   slog 初始化
│   │   │   └── log.go                  #     New(module) → *slog.Logger
│   │   ├── config/                      #   配置持久化
│   │   │   ├── config.go              #     Settings struct + Load/Save + Defaults
│   │   │   └── config_test.go
│   │   └── store/                       #   数据存储抽象（预留）
│   │       └── store.go               #     Store interface (v1: in-memory, v1.5: SQLite)
│   │
│   └── wails/                           # 【框架适配层】Wails UI 绑定
│       ├── app.go                       #   PagerApp: lifecycle + event emission + tray
│       ├── hotkey.go                    #   HotkeyManager: 全局快捷键
│       ├── session_svc.go              #   SessionBinding: 前端会话操作
│       ├── settings_svc.go             #   SettingsBinding: 前端设置操作
│       └── assets/                      #   Go embed 资源
│           ├── tray-icon.png
│           └── tray-icon@2x.png
│
├── cmd/
│   ├── bridge/                          # pager-cc-bridge CLI
│   │   └── main.go
│   └── icongen/                         # 开发工具：生成图标
│       └── main.go
│
├── frontend/                            # React + TypeScript + Tailwind
│   ├── src/
│   │   ├── assets/                      # 前端静态资源（UI 用）
│   │   ├── components/
│   │   ├── pages/
│   │   ├── store/
│   │   └── i18n/
│   ├── bindings/                        # Wails 自动生成（勿手动编辑）
│   └── dist/                            # 构建产物（gitignore）
│
├── build/                               # Wails 构建 + macOS 打包
│   ├── appicon.png
│   ├── config.yml
│   ├── Taskfile.yml                     # wails3 内部依赖
│   └── darwin/
│
├── scripts/                             # 安装/部署脚本
│   ├── install-hooks.sh
│   └── install-launchd.sh
│
├── docs/                                # 项目文档
│   ├── PRD.md
│   └── ...
│
└── Taskfile.yml                         # wails3 dev 内部依赖（勿删）
```

## 文件迁移映射

### Go 文件

| 原路径 | 新路径 | 操作 |
|--------|--------|------|
| `app.go` | `internal/wails/app.go` | 移动 + 改包名 |
| `service.go` | `internal/wails/session_svc.go` | 移动 + 改包名 + 重命名 struct |
| `settings_service.go` | `internal/wails/settings_svc.go` | 移动 + 改包名 + 重命名 struct |
| `hotkey.go` | `internal/wails/hotkey.go` | 移动 + 改包名 |
| `internal/event/types.go` | `internal/domain/entity/event.go` | 移动 + 改包名 |
| `internal/registry/registry.go` | `internal/domain/session/registry.go` | 移动 + 改包名 |
| `internal/registry/registry_test.go` | `internal/domain/session/registry_test.go` | 移动 + 改包名 |
| `internal/server/server.go` | `internal/adapter/httpapi/server.go` | 移动 + 改包名 |
| `internal/server/server_test.go` | `internal/adapter/httpapi/server_test.go` | 移动 + 改包名 |
| `internal/notify/notify.go` | `internal/adapter/notify/notify.go` | 移动（包名不变） |
| `internal/terminal/jump.go` | `internal/adapter/terminal/jump.go` | 移动（包名不变） |
| `internal/bridge/*.go` | `internal/adapter/bridge/*.go` | 移动（包名不变） |
| `internal/settings/settings.go` | `internal/infra/config/config.go` | 移动 + 改包名 + 重命名 |
| `internal/settings/settings_test.go` | `internal/infra/config/config_test.go` | 移动 + 改包名 |
| — (新建) | `internal/infra/log/log.go` | 新增 slog 模块 |
| — (新建) | `internal/infra/store/store.go` | 新增存储接口占位 |

### 资源文件

| 原路径 | 新路径 | 操作 |
|--------|--------|------|
| `assets/tray-icon.png` | `internal/wails/assets/tray-icon.png` | 移动 |
| `assets/tray-icon@2x.png` | `internal/wails/assets/tray-icon@2x.png` | 移动 |
| `assets/icon.png` | `build/appicon.png` | 合并（build 已有） |
| `assets/` 目录 | 删除 | 文件已迁移 |

### 清理

| 路径 | 操作 | 理由 |
|------|------|------|
| `bridge`（根目录二进制） | 删除 + 加入 .gitignore | 编译产物不应提交 |
| `internal/event/` | 删除 | 内容已迁移到 domain/session/ |
| `internal/registry/` | 删除 | 内容已迁移到 domain/session/ |
| `internal/server/` | 删除 | 内容已迁移到 adapter/httpapi/ |
| `internal/settings/` | 删除 | 内容已迁移到 infra/config/ |

## 命名规范

### 文件命名

| 层 | 文件后缀 | 示例 |
|---|----------|------|
| wails 绑定 | `*_svc.go` | `session_svc.go`, `settings_svc.go` |
| wails 管理器 | 功能名.go | `hotkey.go`, `app.go` |
| adapter | 功能名.go | `server.go`, `notify.go`, `jump.go` |
| domain | 领域概念.go | `registry.go`, `event.go` |
| infra | 模块名.go | `config.go`, `log.go`, `store.go` |

### Struct 命名

| 层 | 命名模式 | 示例 |
|---|----------|------|
| wails 绑定 | `XxxBinding` | `SessionBinding`, `SettingsBinding` |
| wails 管理器 | `XxxManager` | `HotkeyManager`, `TrayManager` |
| adapter | 领域术语 | `Server`, `Notifier` |
| domain | 领域实体 | `Registry`, `Session`, `AgentEvent` |
| infra | 技术术语 | `Settings`, `Store` |

## slog 日志设计

### 初始化

```go
// internal/infra/log/log.go
package log

import (
    "log/slog"
    "os"
)

var defaultLevel = &slog.LevelVar{}

// Init sets up the global slog handler with JSON output to stderr.
func Init(level slog.Level) {
    defaultLevel.Set(level)
    handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
        Level: defaultLevel,
    })
    slog.SetDefault(slog.New(handler))
}

// Module returns a logger with a "module" attribute for filtering.
func Module(name string) *slog.Logger {
    return slog.Default().With("module", name)
}
```

### 使用

```go
// 各模块初始化 logger
var logger = log.Module("hotkey")

// 使用
logger.Info("registered", "key", hotkeyStr)
logger.Error("register failed", "err", err, "key", hotkeyStr)
```

## infra/store 预留接口

```go
// internal/infra/store/store.go
package store

import "pager/internal/domain/session"

// SessionStore abstracts session persistence.
// v1: in-memory (Registry itself). v1.5: SQLite.
type SessionStore interface {
    Save(s *session.Session) error
    List() ([]*session.Session, error)
    Remove(key string) error
}
```

## main.go 重构后

```go
package main

import (
    "embed"
    "log/slog"

    "github.com/wailsapp/wails/v3/pkg/application"
    infralog "pager/internal/infra/log"
    "pager/internal/wails"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
    infralog.Init(slog.LevelInfo)

    app := wails.NewPagerApp(assets)
    if err := app.Run(); err != nil {
        slog.Error("fatal", "err", err)
    }
}
```

`internal/wails/app.go` 中的 `NewPagerApp()` 承担所有组装逻辑（创建 registry、server、tray、windows、menu、hotkey），返回可运行的 Wails application。

## 不实现（本次范围外）

- `internal/domain/usage/` 模块（v1.5 实现）
- `internal/infra/store/` 的 SQLite 实现（v1.5 实现）
- Bug 修复（单独 spec）
- README.md 编写（重构完成后补充）
