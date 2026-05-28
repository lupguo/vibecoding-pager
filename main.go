package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/icons"
)

// frontend/dist is built by the React toolchain (npm run build).
// For the initial build / CI the placeholder index.html is sufficient.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// ── Core state layer ────────────────────────────────────────────────────────
	// App owns the session registry and HTTP server.
	// NewApp wires the registry onChange callback (emits Wails events + notifications).
	myApp := NewApp()

	// SessionService exposes registry operations to the React frontend.
	// It shares the same registry pointer as App.
	svc := &SessionService{reg: myApp.Registry()}

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

	// Default icon: Wails built-in macOS template icon (monochrome, auto-tinted by OS).
	// TODO(v1.5): replace with custom Pager icon assets.
	tray.SetTemplateIcon(icons.SystrayMacTemplate)
	tray.SetTooltip("Pager — AI agent monitor")

	// Give App a reference to the tray so the registry onChange callback can update
	// the icon / label when session status changes.
	myApp.SetTray(tray)

	// ── Quit menu ───────────────────────────────────────────────────────────────
	quitMenu := wailsApp.NewMenu()
	quitMenu.Add("Quit Pager").OnClick(func(_ *application.Context) {
		wailsApp.Quit()
	})
	tray.SetMenu(quitMenu)

	// ── Popup window ────────────────────────────────────────────────────────────
	// Frameless, always-on-top, initially hidden.
	// Clicking the tray icon shows it (AttachWindow below).
	//
	// API adaptations from PRD:
	//   PRD used app.NewWebviewWindowWithOptions() — real API is app.Window.NewWithOptions()
	//   PRD used application.NewRGBA(0,0,0,0)     — confirmed available in this version
	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
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

	// Attach the window so clicking the tray icon toggles it.
	// WindowOffset(5) leaves a small gap between icon and panel edge.
	tray.AttachWindow(window).WindowOffset(5)

	// ── Run ─────────────────────────────────────────────────────────────────────
	if err := wailsApp.Run(); err != nil {
		log.Fatalf("[pager] fatal: %v", err)
	}
}
