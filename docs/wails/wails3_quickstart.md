# Wails v3 快速入门指南 (v3.0.0-alpha.96)

> 二八法则：掌握 20% 核心概念，覆盖 80% 开发场景
> 
> 最后更新：2026-05-28

---

## 目录

1. [Wails v3 是什么](#1-wails-v3-是什么)
2. [核心架构（一张图搞懂）](#2-核心架构一张图搞懂)
3. [三个核心概念](#3-三个核心概念)
4. [环境准备](#4-环境准备)
5. [HelloWorld 实战](#5-helloworld-实战)
6. [开发工作流](#6-开发工作流)
7. [常用 API 速查](#7-常用-api-速查)
8. [与 v2 的关键区别](#8-与-v2-的关键区别)

---

## 1. Wails v3 是什么

**一句话：** 用 Go 写后端逻辑 + 用任意前端框架写 UI，编译成一个原生桌面应用。

**不是 Electron。** 不打包 Chromium，使用系统原生 WebView（macOS 上是 WKWebView），最终产物 ~20MB（vs Electron ~150MB）。

**适合什么场景：**
- 开发者工具（CLI companion、状态面板）
- MenuBar / System Tray 小工具
- 内部工具（不需要跨平台浏览器兼容的 desktop app）

---

## 2. 核心架构（一张图搞懂）

```
┌─────────────────────────────────────────────────────────────┐
│                      Go 进程 (你的 app)                       │
│                                                             │
│  ┌───────────────────┐         ┌──────────────────────┐    │
│  │   你的 Service     │◄─ RPC ─►│   WebView (系统原生)   │    │
│  │   (Go struct)     │         │   ┌──────────────┐   │    │
│  │                   │◄─Event─►│   │  React/Vue   │   │    │
│  │  • 业务逻辑       │         │   │  你的前端代码  │   │    │
│  │  • 系统调用       │         │   └──────────────┘   │    │
│  │  • 文件/网络      │         └──────────────────────┘    │
│  └───────────────────┘                                      │
│                                                             │
│  [System Tray]  [通知]  [窗口管理]  [对话框]                   │
└─────────────────────────────────────────────────────────────┘
```

**两条通信通道：**

| 通道 | 方向 | 用途 | 类比 |
|------|------|------|------|
| **Service (RPC)** | 前端 → Go → 前端 | 请求-响应，前端调 Go 方法 | HTTP API |
| **Event** | 双向广播 | 推送通知，状态变更 | WebSocket |

---

## 3. 三个核心概念

### 3.1 Service（服务）

> Go struct 的 exported 方法 = 前端可调用的 API

```go
type GreetService struct{}

// 前端可直接调用这个方法
func (g *GreetService) Greet(name string) string {
    return "Hello, " + name + "!"
}
```

注册后，Wails 自动生成 TypeScript binding：
```typescript
// 自动生成，不要手改
export function Greet(name: string): Promise<string>;
```

前端调用：
```typescript
import { Greet } from '../bindings/.../greetservice'
const result = await Greet("World")  // "Hello, World!"
```

**核心规则：**
- 方法必须 exported（大写开头）
- 参数和返回值必须 JSON 可序列化
- 可以返回 `(T, error)`，error 会传给前端的 Promise.reject

### 3.2 Event（事件）

> 发布-订阅模式，任一方发射，任一方监听

**Go → 前端（最常用）：推送状态更新**
```go
app.Event.Emit("data-updated", newData)
```

**前端监听：**
```typescript
import { Events } from '/wails/runtime.js'
Events.On('data-updated', (event) => {
    setState(event.data)
})
```

**什么时候用 Service vs Event：**
- 前端需要"拿结果" → Service（请求-响应）
- Go 需要"通知前端" → Event（单向推送）

### 3.3 Window（窗口）

> 命令式创建，不在配置里声明

```go
window := app.Window.NewWithOptions(application.WebviewWindowOptions{
    Title:  "My App",
    Width:  800,
    Height: 600,
})
```

支持多窗口、无边框、透明背景、毛玻璃效果。

---

## 4. 环境准备

### 必需

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | 1.21+ | 后端编译 |
| Node.js | 18+ | 前端构建 |
| Xcode CLI Tools | 最新 | macOS CGO 编译 |

### 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.96
```

验证：
```bash
wails3 version   # v3.0.0-alpha.96
wails3 doctor    # 检查所有依赖
```

---

## 5. HelloWorld 实战

### 5.1 创建项目

```bash
wails3 init -n helloworld -t react-ts
cd helloworld
```

生成的目录结构：
```
helloworld/
├── build/
│   └── config.yml         ← dev/build 配置
├── frontend/
│   ├── src/
│   │   └── App.tsx        ← 你的前端
│   ├── bindings/          ← 自动生成，不手改
│   └── package.json
├── main.go                ← 入口
├── greetservice.go        ← 示例 Service
├── go.mod
└── Taskfile.yaml          ← 构建任务
```

### 5.2 理解 main.go

```go
package main

import (
    "embed"
    "log"
    "github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
    // 1. 创建 app，注册 Service
    app := application.New(application.Options{
        Name: "HelloWorld",
        Services: []application.Service{
            application.NewService(&GreetService{}),
        },
        Assets: application.AssetOptions{
            Handler: application.BundledAssetFileServer(assets),
        },
    })

    // 2. 创建窗口
    app.Window.NewWithOptions(application.WebviewWindowOptions{
        Title: "Hello World",
        Width: 800, Height: 600,
    })

    // 3. 运行
    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}
```

### 5.3 编写 Service

```go
// greetservice.go
package main

import (
    "context"
    "fmt"
    "github.com/wailsapp/wails/v3/pkg/application"
)

type GreetService struct {
    counter int
}

// 前端可调用
func (g *GreetService) Greet(name string) string {
    g.counter++
    return fmt.Sprintf("Hello %s! (第 %d 次调用)", name, g.counter)
}

// 可选：生命周期钩子
func (g *GreetService) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
    fmt.Println("[GreetService] 启动完成")
    return nil
}
```

### 5.4 生成 Binding

每次修改 Service 的 exported 方法后，运行：

```bash
wails3 generate bindings
```

自动产出 `frontend/bindings/` 下的 TypeScript 文件。

### 5.5 前端调用

```tsx
// frontend/src/App.tsx
import { useState } from 'react'
import { Greet } from '../bindings/github.com/you/helloworld/greetservice'

function App() {
    const [name, setName] = useState('')
    const [result, setResult] = useState('')

    const handleGreet = async () => {
        const msg = await Greet(name)  // 调用 Go 方法！
        setResult(msg)
    }

    return (
        <div style={{ padding: '2rem' }}>
            <h1>Wails v3 Hello World</h1>
            <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="输入名字"
            />
            <button onClick={handleGreet}>打招呼</button>
            {result && <p>{result}</p>}
        </div>
    )
}

export default App
```

### 5.6 启动开发模式

```bash
wails3 dev
```

窗口弹出，修改 Go 文件自动重编译重启，修改前端文件热更新。

### 5.7 构建发布

```bash
wails3 build     # 编译二进制
wails3 package   # 打包成 .app (macOS)
```

---

## 6. 开发工作流

### 日常开发循环

```
编辑 Go Service 方法
       ↓
wails3 generate bindings  ← 只在改了方法签名时需要
       ↓
前端 import 使用
       ↓
wails3 dev（自动重编译 + 热更新）
```

### build/config.yml 解析

```yaml
version: '3'

info:
  productName: "MyApp"
  productIdentifier: "com.example.myapp"
  version: "1.0.0"

dev_mode:
  root_path: .
  debounce: 1000        # 文件变更后等 1s 再重建（防抖）
  ignore:
    dir: [.git, node_modules, frontend]
    watched_extension: ["*.go"]
  executes:
    - cmd: cd frontend && npm install
      type: once         # 只在首次运行
    - cmd: cd frontend && npm run dev
      type: background   # 后台运行 Vite
    - cmd: go build -o bin/MyApp .
      type: blocking     # 等编译完成
    - cmd: ./bin/MyApp
      type: primary      # 主进程，Go 文件改了会 kill + 重启
```

**四种 type：**
| type | 行为 |
|------|------|
| `once` | 只在首次启动时运行一次 |
| `background` | 后台常驻（Vite dev server） |
| `blocking` | 必须成功完成才进下一步（go build） |
| `primary` | 你的 app 进程，文件变更时 kill 重启 |

### 常用命令

```bash
wails3 dev                    # 开发模式
wails3 generate bindings      # 重新生成前端绑定
wails3 build                  # 生产构建
wails3 package                # 打包 .app
wails3 doctor                 # 环境检查
```

---

## 7. 常用 API 速查

### Go 侧

```go
// === 创建 app ===
app := application.New(application.Options{
    Name:     "MyApp",
    Services: []application.Service{application.NewService(&MySvc{})},
    Assets:   application.AssetOptions{Handler: application.BundledAssetFileServer(assets)},
    Mac: application.MacOptions{
        ActivationPolicy: application.ActivationPolicyAccessory, // MenuBar app，无 Dock 图标
    },
})

// === 窗口 ===
w := app.Window.NewWithOptions(application.WebviewWindowOptions{
    Title: "Title", Width: 800, Height: 600,
    Frameless: true, AlwaysOnTop: true, Hidden: true,
    BackgroundColour: application.NewRGBA(0, 0, 0, 0), // 透明
})
w.Show()
w.Hide()
w.SetTitle("New Title")
w.Center()

// === 事件 ===
app.Event.Emit("event-name", data)           // 广播给所有前端
app.Event.On("event-name", func(e *application.CustomEvent) { ... })  // Go 监听

// === System Tray ===
tray := app.SystemTray.New()
tray.SetTemplateIcon(iconBytes)               // macOS template icon（自适应深色模式）
tray.AttachWindow(window).WindowOffset(5)     // 点击 tray 弹出窗口

// === Service 生命周期 ===
func (s *MySvc) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error { ... }
func (s *MySvc) ServiceShutdown() error { ... }
```

### 前端侧

```typescript
// === 调用 Go Service ===
import { MyMethod } from '../bindings/.../myservice'
const result = await MyMethod(arg)

// === 事件 ===
import { Events } from '/wails/runtime.js'
Events.On('event-name', (event) => { console.log(event.data) })
Events.Off('event-name')
await Events.Emit({ name: 'from-frontend', data: payload })

// === 窗口控制 ===
import { Window } from '/wails/runtime.js'
Window.Hide()
Window.Show()
Window.SetTitle('New Title')
```

---

## 8. 与 v2 的关键区别

| 维度 | v2 | v3 |
|------|----|----|
| 入口 | `wails.Run(&options.App{...})` | `application.New(opts) → app.Run()` |
| 绑定 | `Bind: []interface{}{&Svc{}}` | `Services: []application.Service{...}` |
| 事件 | `runtime.EventsEmit(ctx, ...)` | `app.Event.Emit(name, data)` |
| 窗口 | 在 Options 里声明 | `app.Window.NewWithOptions(...)` |
| 前端运行时 | `@wailsapp/runtime` (npm) | `/wails/runtime.js` (Wails 注入) |
| CLI | `wails` | `wails3` |
| 生命周期 | `OnStartup(ctx)` | `ServiceStartup(ctx, opts)` |
| System Tray | 不支持 | `app.SystemTray.New()` |
| 多窗口 | 不支持 | 支持 |

**最大的思维转变：**
- v2：一切通过 `ctx` 访问 runtime
- v3：通过 `app` 实例访问一切，Service 自带生命周期

---

## 附录：MenuBar App 模板

以下是 Pager 项目使用的 MenuBar app 骨架，供快速复用：

```go
package main

import (
    "embed"
    "log"
    "github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
    app := application.New(application.Options{
        Name: "MyTrayApp",
        Services: []application.Service{
            application.NewService(&MyService{}),
        },
        Assets: application.AssetOptions{
            Handler: application.BundledAssetFileServer(assets),
        },
        Mac: application.MacOptions{
            ActivationPolicy: application.ActivationPolicyAccessory, // 无 Dock 图标
        },
    })

    // System Tray
    tray := app.SystemTray.New()
    tray.SetTemplateIcon(iconBytes) // 22x22 黑色 PNG

    // Popup 窗口
    window := app.Window.NewWithOptions(application.WebviewWindowOptions{
        Width: 400, Height: 600,
        Hidden: true, Frameless: true, AlwaysOnTop: true,
        BackgroundColour: application.NewRGBA(0, 0, 0, 0),
    })
    tray.AttachWindow(window).WindowOffset(5)

    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}
```

配合 `build/darwin/Info.plist` 中的 `LSUIElement = true` 确保 Dock 不显示图标。

---

> **总结：Wails v3 的 80/20 就是三件事：**
> 1. **Service** — Go struct 方法 = 前端 API
> 2. **Event** — Go 推数据给前端
> 3. **Window** — 命令式创建和控制窗口
>
> 掌握这三个，就能构建 80% 的桌面应用场景。
