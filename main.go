package main

import (
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"pager/internal/settings"
)

// frontend/dist is built by the React toolchain (npm run build).
// For the initial build / CI the placeholder index.html is sufficient.
//
//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/tray-icon@2x.png
var trayIconData []byte

func main() {
	// ── Core state layer ────────────────────────────────────────────────────────
	// App owns the session registry and HTTP server.
	// NewApp wires the registry onChange callback (emits Wails events + notifications).
	myApp := NewApp()

	// SessionService exposes registry operations to the React frontend.
	// It shares the same registry pointer as App.
	svc := &SessionService{reg: myApp.registry()}

	// window is declared here (nil) so the settingsSvc closure below can capture it by
	// reference.  It is assigned after wailsApp is created; Go closures capture variables
	// (not values), so by the time onChange fires the pointer is always valid.
	var window *application.WebviewWindow

	settingsSvc := NewSettingsService(func(cfg settings.Settings) {
		registerHotkey(window, cfg.HotkeyToggle)
	})

	// ── Wails application ───────────────────────────────────────────────────────
	// API adaptations from PRD:
	//   PRD assumed application.Options{Assets: {FS: assets}}
	//   Real API:  application.Options{Assets: {Handler: BundledAssetFileServer(assets)}}
	//
	//   PRD assumed application.Services{application.NewService(&service.SessionService{})}
	//   Real API:  []application.Service{application.NewService(svc)}
	//
	//   PRD assumed app.NewSystemTray() / app.NewWebviewWindowWithOptions()
	//   Real API:  app.SystemTray.New()  / app.Window.NewWithOptions()
	wailsApp := application.New(application.Options{
		Name:        "Pager",
		Description: "AI coding agents 状态感知层",

		// Register both services.
		// myApp (App) implements ServiceStartup — it starts the HTTP server.
		// svc (SessionService) exposes bindings to the frontend.
		Services: []application.Service{
			application.NewService(myApp),
			application.NewService(svc),
			application.NewService(settingsSvc),
		},

		// Embed the built React app.
		// BundledAssetFileServer scans the FS for index.html automatically,
		// so //go:embed all:frontend/dist works without an fs.Sub call.
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},

		// ActivationPolicyAccessory hides Pager from the macOS Dock and app-switcher —
		// it is a pure MenuBar / background app.
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	// ── System tray ─────────────────────────────────────────────────────────────
	tray := wailsApp.SystemTray.New()

	// Custom Pager tray icon (blue-purple gradient squircle with white P).
	tray.SetTemplateIcon(trayIconData)
	tray.SetTooltip("Pager — AI agent monitor")

	// Give App a reference to the tray so the registry onChange callback can update
	// the icon / label when session status changes.
	myApp.setTray(tray)

	// ── Popup window ────────────────────────────────────────────────────────────
	// Frameless, always-on-top, initially hidden.
	// Clicking the tray icon shows it (AttachWindow below).
	//
	// API adaptations from PRD:
	//   PRD used app.NewWebviewWindowWithOptions() — real API is app.Window.NewWithOptions()
	//   PRD used application.NewRGBA(0,0,0,0)     — confirmed available in this version
	window = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "Pager",
		Name:          "pager-panel",
		Width:         400,
		Height:        600,
		Hidden:        true,
		Frameless:     true,
		AlwaysOnTop:   true,
		DisableResize: true,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	})

	// ── Settings window ─────────────────────────────────────────────────────────
	// Non-frameless, normal resize-disabled window for the settings UI.
	// Hidden by default; opened via the tray menu "偏好设置..." item.
	settingsWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "Pager 设置",
		Name:          "pager-settings",
		Width:         720,
		Height:        520,
		Hidden:        true,
		DisableResize: true,
		URL:           "#/settings",
	})

	// ── Tray menu ───────────────────────────────────────────────────────────────
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

	// Attach the popup window so clicking the tray icon toggles it.
	// WindowOffset(5) leaves a small gap between icon and panel edge.
	tray.AttachWindow(window).WindowOffset(5)

	// Register initial global hotkey after a brief delay — the macOS Carbon event
	// loop (needed by golang.design/x/hotkey) only becomes active inside wailsApp.Run().
	// Calling registerHotkey before Run() causes a SIGTRAP crash.
	go func() {
		time.Sleep(500 * time.Millisecond)
		initialCfg, _ := settings.LoadFrom(settings.DefaultPath())
		registerHotkey(window, initialCfg.HotkeyToggle)
	}()

	// ── Run ─────────────────────────────────────────────────────────────────────
	if err := wailsApp.Run(); err != nil {
		log.Fatalf("[pager] fatal: %v", err)
	}
}
