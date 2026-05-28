package main

import (
	"context"
	"log"

	"pager/internal/event"
	"pager/internal/notify"
	"pager/internal/registry"
	"pager/internal/server"
	"pager/internal/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App manages the core Pager state: session registry + HTTP server.
// It is registered as a Wails service so it participates in the app lifecycle.
//
// API adaptations from PRD:
//   - PRD assumed OnStartup(ctx, options) — real Wails v3 interface is ServiceStartup(ctx, options)
//   - PRD assumed a.app.EmitEvent(...) — real API is application.Get().Event.Emit(...)
//   - tray icon update is done via a stored *application.SystemTray reference (set from main.go)
type App struct {
	reg  *registry.Registry
	srv  *server.Server
	tray *application.SystemTray // set via SetTray() after tray is created in main
}

// NewApp constructs the App and wires up the registry onChange callback.
//
// The onChange callback safely uses application.Get() to reach the Wails event bus —
// application.Get() returns nil until application.New() is called, but onChange is only
// invoked when an AgentEvent arrives over HTTP (i.e. after the server is running and the
// Wails app is already live), so the race does not occur in practice.
func NewApp() *App {
	a := &App{}

	a.reg = registry.New(func(sessions []*registry.Session) {
		wailsApp := application.Get()
		if wailsApp == nil {
			return
		}

		// 1. Push full session list to the React frontend.
		//    The frontend subscribes via Events.On("sessions-updated") in its zustand store.
		wailsApp.Event.Emit("sessions-updated", sessions)

		// 2. Show a macOS system notification for the triggering event.
		//    notify.Show internally filters: only pre_tool_use / stop / error fire a notification.
		if len(sessions) > 0 && sessions[0].LastEvent != nil {
			cfg, _ := settings.LoadFrom(settings.DefaultPath())
			notify.ShowFull(sessions[0].LastEvent, cfg.NotificationLevel, cfg.Language)
		}

		// 3. Update the tray icon to reflect aggregate session status.
		if a.tray != nil {
			a.updateTrayIcon(sessions)
		}
	})

	a.srv = server.New(a.reg)
	return a
}

// ServiceStartup implements application.ServiceStartup.
// Called by Wails after the application is ready — we start the HTTP server here
// so it is guaranteed to be running before the UI becomes interactive.
func (a *App) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	log.Println("[pager] starting HTTP server on :7421")
	a.srv.Start()
	return nil
}

// ServiceShutdown implements application.ServiceShutdown.
// Called by Wails during orderly shutdown (reverse registration order).
func (a *App) ServiceShutdown() error {
	// v1.5: flush in-memory registry to SQLite here.
	return nil
}

// Registry returns the session registry so other services (SessionService) can share it.
func (a *App) registry() *registry.Registry {
	return a.reg
}

// SetTray stores the system tray reference so the onChange callback can update its icon.
// Must be called from main.go before app.Run().
func (a *App) setTray(tray *application.SystemTray) {
	a.tray = tray
}

// updateTrayIcon picks an icon based on the highest-priority status across all sessions:
//
//	waiting  → red  template icon  (agent blocked, needs user attention)
//	active   → blue template icon  (work is happening)
//	idle     → gray template icon  (no active sessions)
//
// macOS template icons are monochrome PNGs; the OS tints them automatically.
// For now we use the same Wails default template icon for all states and add a
// TODO for custom per-state icons (requires embedding separate PNG assets).
func (a *App) updateTrayIcon(sessions []*registry.Session) {
	hasWaiting := false
	hasActive := false

	for _, s := range sessions {
		switch s.Status {
		case event.StatusWaiting:
			hasWaiting = true
		case event.StatusActive:
			hasActive = true
		}
	}

	// TODO(v1.5): embed separate red/blue/gray 22×22 template PNG assets and swap here.
	// For MVP the same template icon is used; the badge count on "waiting" is sufficient
	// to communicate urgency on macOS.
	switch {
	case hasWaiting:
		// Red-state: a waiting (blocked) session exists — most urgent.
		// Setting a label is a lightweight way to signal state until custom icons land.
		a.tray.SetLabel("●")
	case hasActive:
		// Blue-state: at least one session is actively executing tools.
		a.tray.SetLabel("")
	default:
		// Gray-state: all sessions finished or no sessions.
		a.tray.SetLabel("")
	}
}
