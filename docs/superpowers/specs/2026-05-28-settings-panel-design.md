# Pager Settings Panel Design Spec

Date: 2026-05-28

## Overview

为 Pager 添加独立的配置面板窗口和右键 Tray Menu 操作，让用户可以自定义外观、快捷键和通知行为。

## 功能范围

### 1. Tray 右键菜单

| 菜单项 | 快捷键 | 行为 |
|--------|--------|------|
| 偏好设置... | `CMD+,` | 打开/聚焦配置面板窗口 |
| 退出 Pager | `Cmd+Q` | 退出应用 |

### 2. 配置面板窗口

**窗口形态：** macOS 标准窗口（原生标题栏、红绿灯按钮、可拖拽、CMD+W 关闭）

**窗口尺寸：** 720 × 520px，居中显示，不可调整大小

**布局结构：** 左右栏

- 左侧导航栏：固定 180px 宽度（约 25%）
- 右侧内容区：flex: 1，可滚动

**导航项（仅 2 项）：**

| 导航项 | 图标 | 说明 |
|--------|------|------|
| 基本设置 | ⚙️ | 外观 + 快捷键 + 通知（合并为一页） |
| 关于 | ℹ️ | 版本信息 + 作者 + 检查更新 |

### 3. 基本设置页面

右侧内容区按板块分组，每个板块用白色圆角卡片（border-radius: 10px, 1px solid border）包裹。板块之间间距 16px。板块标题为灰色小字标签（12px, font-weight: 600, opacity 0.45）。

#### 板块：外观

| 设置项 | 控件类型 | 选项 | 默认值 |
|--------|----------|------|--------|
| 语言 | Dropdown | 中文 / English | 中文 |

语言切换即时生效（i18next hot switch），无需重启应用。
| 主题 | Segmented Control | 浅色 / 深色 / 系统 | 系统 |
| 透明度 | Slider | 0% ~ 100% | 75% |

#### 板块：快捷键

| 设置项 | 控件类型 | 说明 | 默认值 |
|--------|----------|------|--------|
| 唤起窗口 | Hotkey Recorder | 全局快捷键打开 Pager popup 面板 | `⌥E` (Alt+E) |

**热键录制交互：** 点击快捷键区域进入录制模式（高亮边框），按下新组合键后自动保存。支持 Escape 取消。

#### 板块：通知

| 设置项 | 控件类型 | 选项 | 默认值 |
|--------|----------|------|--------|
| 通知级别 | Dropdown | 所有消息 / 仅需确认时 | 仅需确认时 |

**通知级别逻辑：**

- **所有消息（"all"）：** 所有到达 registry 的 AgentEvent 均触发 macOS 系统通知（pre_tool_use / stop / error，与当前行为一致。post_tool_use 仍不通知——它是工具完成确认，无需打扰用户）
- **仅需确认时（"attention_only"）：** 仅当 `event.AttentionLevel == "attention"`（需用户确认的 pre_tool_use）或 `event.EventType == stop / error` 时通知。普通工具调用（running 级别的 pre_tool_use）不通知。

### 4. 关于页面

居中布局，内容垂直居中显示：

1. **App 图标区：** 72px 渐变色圆角方形图标 + 名称 "Pager" + 描述 "AI coding agents 状态感知层"
2. **信息卡片：** 版本号（如 1.0.0 build 1）+ 作者（sapaude）
3. **操作卡片：** 检查更新（预留，v1 实现为打开 GitHub releases 页面）+ GitHub 仓库链接
4. **底部版权：** "© 2026 sapaude · Built with Wails v3"

## 架构设计

### 配置持久化

**存储路径：** `~/.config/pager/settings.json`

**配置结构：**

```json
{
  "language": "zh",
  "theme": "system",
  "opacity": 75,
  "hotkey_toggle": "Alt+E",
  "notification_level": "attention_only"
}
```

字段说明：

| 字段 | 类型 | 取值 | 说明 |
|------|------|------|------|
| language | string | "zh" / "en" | 界面语言 |
| theme | string | "light" / "dark" / "system" | 主题模式 |
| opacity | int | 0-100 | 面板背景透明度百分比 |
| hotkey_toggle | string | 修饰符+键名 | 全局唤起快捷键 |
| notification_level | string | "all" / "attention_only" | 通知级别 |

### Go 侧新增

**`internal/settings/settings.go`** — SettingsService

```
Load() → 从文件读取，不存在则返回默认值
Save(cfg) → 写入文件（atomic write: tmp + rename）
Get(key) → 获取单个配置项
Set(key, value) → 设置单个配置项并持久化
```

**`settings_service.go`**（根目录）— Wails bindings 暴露给前端

```
GetSettings() Settings        — 前端初始化时拉取全量配置
UpdateSettings(Settings) error — 前端保存时写入
```

注册为 Wails Service，与现有 SessionService 并列。

### 前端新增

**`frontend/src/store/settings.ts`** — zustand store

- 状态：Settings 对象
- 动作：loadSettings()、updateSettings(partial)
- 初始化时调用 Go binding GetSettings()

**`frontend/src/pages/SettingsPanel.tsx`** — 配置面板主页面（左右栏布局容器）

**`frontend/src/pages/settings/GeneralSettings.tsx`** — 基本设置内容区

**`frontend/src/pages/settings/AboutSettings.tsx`** — 关于页面内容区

### 窗口管理

在 `main.go` 中新增配置面板窗口：

```go
settingsWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
    Title:         "Pager 设置",
    Name:          "pager-settings",
    Width:         720,
    Height:        520,
    Hidden:        true,
    DisableResize: true,
    URL:           "#/settings",  // hash route
})
```

**路由方案：** 前端使用 hash router（`#/` = session list popup, `#/settings` = 配置面板）。两个窗口加载同一份 embedded 前端资源（`frontend/dist/index.html`），通过 URL hash 区分渲染内容。Wails v3 的 WebviewWindow `URL` 字段支持 `#/path` 形式指定初始 hash。popup window 不设 URL（默认 `#/`），settings window 设为 `#/settings`。

### Tray Menu 改造

当前 `main.go` 中 tray 仅有 "Quit Pager" 菜单。改造为：

```go
trayMenu := wailsApp.NewMenu()
trayMenu.Add("偏好设置...").
    SetAccelerator("CmdOrCtrl+,").
    OnClick(func(_ *application.Context) {
        settingsWindow.Show()
        settingsWindow.Focus()
    })
trayMenu.AddSeparator()
trayMenu.Add("退出 Pager").
    SetAccelerator("CmdOrCtrl+Q").
    OnClick(func(_ *application.Context) {
        wailsApp.Quit()
    })
tray.SetMenu(trayMenu)
```

### 全局热键

Wails v3 不内置全局热键注册。方案：

- 使用 Go 的 `golang.design/x/hotkey` 库注册系统级全局热键
- 热键触发时调用 popup window 的 Show/Hide toggle
- 配置变更时 unregister 旧热键 + register 新热键

### 通知级别集成

修改 `internal/notify/notify.go` 中的 `Show()` 函数：

- 接收 notification_level 配置参数
- `"all"` 模式：保持当前行为（pre_tool_use / stop / error 全部通知）
- `"attention_only"` 模式：仅当 event.AttentionLevel == "attention" 或 event.EventType == stop/error 时通知

### 主题/透明度应用

- **主题：** 前端根据 settings.theme 设置 `<html>` 的 class（`light` / `dark`），CSS 变量体系已支持 dark mode（现有 `prefers-color-scheme` media query 改为 class-based）
- **透明度：** 修改 `--pager-bg` 的 alpha 值，通过 Wails event 通知 popup window 实时更新

## 多语言 i18n

**方案：** 前端使用轻量 i18n 方案（`i18next` + `react-i18next`），Go 侧不涉及多语言。

**翻译文件结构：**

```
frontend/src/i18n/
├── index.ts          # i18next 初始化配置
├── locales/
│   ├── zh.json       # 中文翻译
│   └── en.json       # 英文翻译
```

**翻译范围：**

- 配置面板所有 UI 文本（导航、标签、描述、选项）
- Session popup 面板文本（header、filter 标签、card 内容标签）
- 通知文本（`notify.go` 中的中文字符串需改为从配置读取 language 后选择对应文案，或保持 osascript 通知用固定语言）

**语言切换逻辑：**

1. 前端 settings store 中 language 变更时调用 `i18next.changeLanguage(lang)`
2. 所有组件通过 `useTranslation()` hook 获取翻译文本，自动响应语言变化
3. Go 侧通知文本：根据 settings.language 选择中/英文案（硬编码 map，不引入 Go i18n 库）

## App Icon

**设计：** 与关于页面一致的渐变色圆角方形图标

- 渐变：`linear-gradient(135deg, #007aff, #5856d6)`（蓝→紫）
- 形状：macOS 标准圆角方形（squircle）
- 前景：白色 📟 符号或简化的 "P" 字母
- 尺寸：生成 1024×1024 母版，导出 16/32/64/128/256/512/1024 px 各尺寸
- 格式：`icon.icns`（macOS app icon）+ 22×22 template PNG（tray icon）

**文件位置：**

```
assets/
├── icon.icns              # macOS .app icon
├── icon.png               # 1024px 母版
├── tray-icon.png          # 22×22 template icon (monochrome)
└── tray-icon@2x.png       # 44×44 retina tray icon
```

**集成：**

- `main.go` 中 `tray.SetTemplateIcon()` 改用自定义 tray icon
- Wails build config 中引用 `assets/icon.icns` 作为 .app 图标

## 不实现（本次范围外）

- 热键录制的冲突检测（与其他应用快捷键冲突时不处理）
- 检查更新的自动更新功能（仅打开 GitHub releases）
- 配置导入/导出
- 配置文件 schema migration（v1 只有一个版本）
