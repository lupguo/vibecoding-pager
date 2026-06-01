package wails

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"pager/internal/adapter/httpapi"
	"pager/internal/adapter/notify"
	"pager/internal/domain/entity"
	"pager/internal/domain/session"
	"pager/internal/infra/config"
	"pager/internal/infra/store"
)

//go:embed assets/tray-icon@2x.png
var trayIconData []byte

// PagerApp orchestrates the full Wails application lifecycle.
type PagerApp struct {
	tracker *session.Tracker
	srv     *httpapi.Server
	store   store.EventStore
	tray    *application.SystemTray
}

// defaultDBPath returns the SQLite database file path.
func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pager", "pager.db")
}

// NewPagerApp assembles and returns a runnable Wails application.
func NewPagerApp(assets embed.FS) *application.App {
	logger := slog.Default().With("module", "wails")

	p := &PagerApp{}

	// ── Load config ─────────────────────────────────────────────────────────
	initialCfg, _ := config.LoadFrom(config.DefaultPath())

	// ── SQLite EventStore ───────────────────────────────────────────────────
	eventStore, err := store.NewSQLiteStore(defaultDBPath())
	if err != nil {
		slog.Error("failed to open event store", "error", err)
		// Continue without persistence — graceful degradation
	}
	p.store = eventStore

	// ── SessionTracker ──────────────────────────────────────────────────────
	p.tracker = session.NewTracker(func(sessions []*session.Session) {
		wailsApp := application.Get()
		if wailsApp == nil {
			return
		}
		wailsApp.Event.Emit("sessions-updated", sessions)

		if len(sessions) > 0 && sessions[0].LastEvent != nil {
			cfg, _ := config.LoadFrom(config.DefaultPath())
			e := sessions[0].LastEvent
			if notify.ShouldNotifyByConfig(e, cfg.NotificationEvents) {
				notify.ShowEvent(e, cfg.Language)
			}
		}

		if p.tray != nil {
			p.updateTrayIcon(sessions)
		}
	})

	// ── Replay from SQLite ──────────────────────────────────────────────────
	if eventStore != nil {
		loadHours := initialCfg.SessionLoadHours
		if loadHours <= 0 {
			loadHours = 24
		}
		replayEvents, err := eventStore.LoadRecentSessions(loadHours)
		if err != nil {
			slog.Error("failed to load recent sessions", "error", err)
		} else if len(replayEvents) > 0 {
			logger.Info("replaying events from SQLite", "count", len(replayEvents))
			p.tracker.Replay(replayEvents)
		}
	}

	// ── HTTP Server ─────────────────────────────────────────────────────────
	var serverOpts []httpapi.ServerOption
	if eventStore != nil {
		serverOpts = append(serverOpts, httpapi.WithStore(eventStore))
	}
	p.srv = httpapi.New(p.tracker, serverOpts...)

	// ── Services ────────────────────────────────────────────────────────────
	sessionBinding := &SessionBinding{tracker: p.tracker, store: eventStore}

	var popupWindow *application.WebviewWindow
	var settingsWindow *application.WebviewWindow

	settingsBinding := NewSettingsBinding(func(cfg config.Settings) {
		RegisterHotkey(popupWindow, cfg.HotkeyToggle)
	}, eventStore)

	// WindowBinding will be connected to windows after creation
	windowBinding := &WindowBinding{configPath: config.DefaultPath()}

	// ── Wails app ───────────────────────────────────────────────────────────
	wailsApp := application.New(application.Options{
		Name:        "Pager",
		Description: "AI coding agents 状态感知层",
		Services: []application.Service{
			application.NewService(p),
			application.NewService(sessionBinding),
			application.NewService(settingsBinding),
			application.NewService(windowBinding),
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
		Width:            initialCfg.PopupWidth,
		Height:           520,
		MinWidth:         300,
		MaxWidth:         600,
		MinHeight:        200,
		MaxHeight:        800,
		Hidden:           true,
		Frameless:        true,
		AlwaysOnTop:      initialCfg.PopupPinned,
		DisableResize:    false,
		HideOnFocusLost:  false, // Managed manually via WindowLostFocus hook
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		Mac: application.MacWindow{
			Backdrop:           application.MacBackdropTransparent,
			CollectionBehavior: application.MacWindowCollectionBehaviorMoveToActiveSpace,
		},
	})

	// Manual hide-on-focus-lost: only hide when NOT pinned
	popupWindow.RegisterHook(events.Common.WindowLostFocus, func(e *application.WindowEvent) {
		cfg, _ := config.LoadFrom(config.DefaultPath())
		if !cfg.PopupPinned {
			popupWindow.Hide()
		}
	})

	// ── Settings window ─────────────────────────────────────────────────────
	settingsWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "Pager 设置",
		Name:          "pager-settings",
		Width:         720,
		Height:        520,
		Hidden:        true,
		Frameless:     true,
		DisableResize: true,
		URL:           "#/settings",
	})

	// B1 fix: Intercept window close → hide instead of destroy.
	settingsWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		settingsWindow.Hide()
	})

	// Connect WindowBinding to created windows
	windowBinding.popup = popupWindow
	windowBinding.settings = settingsWindow

	// ── Tray menu ───────────────────────────────────────────────────────────
	trayMenu := wailsApp.NewMenu()
	trayMenu.Add("偏好设置...").
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
	if p.store != nil {
		slog.Info("closing event store", "module", "wails")
		return p.store.Close()
	}
	return nil
}

func (p *PagerApp) updateTrayIcon(sessions []*session.Session) {
	hasWaiting := false
	hasWorking := false

	for _, s := range sessions {
		switch s.Status {
		case entity.StatusWaiting:
			hasWaiting = true
		case entity.StatusWorking:
			hasWorking = true
		}
	}

	switch {
	case hasWaiting:
		p.tray.SetLabel("●")
	case hasWorking:
		p.tray.SetLabel("")
	default:
		p.tray.SetLabel("")
	}
}
