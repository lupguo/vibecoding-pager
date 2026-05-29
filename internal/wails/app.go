package wails

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"pager/internal/adapter/httpapi"
	"pager/internal/adapter/notify"
	"pager/internal/domain/entity"
	"pager/internal/domain/session"
	"pager/internal/infra/config"
)

//go:embed assets/tray-icon@2x.png
var trayIconData []byte

// PagerApp orchestrates the full Wails application lifecycle.
type PagerApp struct {
	reg  *session.Registry
	srv  *httpapi.Server
	tray *application.SystemTray
}

// NewPagerApp assembles and returns a runnable Wails application.
func NewPagerApp(assets embed.FS) *application.App {
	logger := slog.Default().With("module", "wails")

	p := &PagerApp{}

	// ── Registry + HTTP server ──────────────────────────────────────────────
	p.reg = session.New(func(sessions []*session.Session) {
		wailsApp := application.Get()
		if wailsApp == nil {
			return
		}
		wailsApp.Event.Emit("sessions-updated", sessions)

		if len(sessions) > 0 && sessions[0].LastEvent != nil {
			cfg, _ := config.LoadFrom(config.DefaultPath())
			notify.ShowFull(sessions[0].LastEvent, cfg.NotificationLevel, cfg.Language)
		}

		if p.tray != nil {
			p.updateTrayIcon(sessions)
		}
	})

	p.srv = httpapi.New(p.reg)

	// ── Services ────────────────────────────────────────────────────────────
	sessionBinding := &SessionBinding{reg: p.reg}

	var popupWindow *application.WebviewWindow

	settingsBinding := NewSettingsBinding(func(cfg config.Settings) {
		RegisterHotkey(popupWindow, cfg.HotkeyToggle)
	})

	// ── Wails app ───────────────────────────────────────────────────────────
	wailsApp := application.New(application.Options{
		Name:        "Pager",
		Description: "AI coding agents 状态感知层",
		Services: []application.Service{
			application.NewService(p),
			application.NewService(sessionBinding),
			application.NewService(settingsBinding),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	// ── System tray ─────────────────────────────────────────────────────────
	tray := wailsApp.SystemTray.New()
	tray.SetTemplateIcon(trayIconData)
	tray.SetTooltip("Pager — AI agent monitor")
	p.tray = tray

	// ── Popup window ────────────────────────────────────────────────────────
	popupWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Pager",
		Name:             "pager-panel",
		Width:            400,
		Height:           600,
		Hidden:           true,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	})

	// ── Settings window ─────────────────────────────────────────────────────
	settingsWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "Pager 设置",
		Name:          "pager-settings",
		Width:         720,
		Height:        520,
		Hidden:        true,
		DisableResize: true,
		URL:           "#/settings",
	})

	// ── Tray menu ───────────────────────────────────────────────────────────
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

	// Attach popup to tray icon click
	tray.AttachWindow(popupWindow).WindowOffset(5)

	// ── Global hotkey (deferred — needs RunLoop active) ─────────────────────
	go func() {
		time.Sleep(500 * time.Millisecond)
		initialCfg, _ := config.LoadFrom(config.DefaultPath())
		RegisterHotkey(popupWindow, initialCfg.HotkeyToggle)
	}()

	logger.Info("app assembled")
	return wailsApp
}

// ServiceStartup implements application.ServiceStartup.
func (p *PagerApp) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	slog.Info("starting HTTP server", "module", "wails", "addr", httpapi.ListenAddr)
	p.srv.Start()
	return nil
}

// ServiceShutdown implements application.ServiceShutdown.
func (p *PagerApp) ServiceShutdown() error {
	return nil
}

func (p *PagerApp) updateTrayIcon(sessions []*session.Session) {
	hasWaiting := false
	hasActive := false

	for _, s := range sessions {
		switch s.Status {
		case entity.StatusWaiting:
			hasWaiting = true
		case entity.StatusActive:
			hasActive = true
		}
	}

	switch {
	case hasWaiting:
		p.tray.SetLabel("●")
	case hasActive:
		p.tray.SetLabel("")
	default:
		p.tray.SetLabel("")
	}
}
