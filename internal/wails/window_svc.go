package wails

import (
	"pager/internal/infra/config"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// WindowBinding exposes popup window controls to the frontend.
type WindowBinding struct {
	popup      *application.WebviewWindow
	settings   *application.WebviewWindow
	configPath string
}

func NewWindowBinding(popup, settings *application.WebviewWindow) *WindowBinding {
	return &WindowBinding{
		popup:      popup,
		settings:   settings,
		configPath: config.DefaultPath(),
	}
}

// SetPinned toggles always-on-top and persists state.
func (w *WindowBinding) SetPinned(pinned bool) error {
	w.popup.SetAlwaysOnTop(pinned)

	cfg, _ := config.LoadFrom(w.configPath)
	cfg.PopupPinned = pinned
	return config.SaveTo(w.configPath, cfg)
}

// SetPopupWidth persists the user-adjusted width.
func (w *WindowBinding) SetPopupWidth(width int) error {
	if width < 300 {
		width = 300
	}
	if width > 600 {
		width = 600
	}
	cfg, _ := config.LoadFrom(w.configPath)
	cfg.PopupWidth = width
	return config.SaveTo(w.configPath, cfg)
}

// OpenSettings shows the settings window.
func (w *WindowBinding) OpenSettings() {
	w.settings.Show()
	w.settings.Focus()
}
