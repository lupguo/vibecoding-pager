package wails

import (
	"pager/internal/infra/config"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// SettingsBinding exposes user settings to the React frontend via Wails bindings.
type SettingsBinding struct {
	path     string
	onChange func(config.Settings)
}

// NewSettingsBinding creates a SettingsBinding.
func NewSettingsBinding(onChange func(config.Settings)) *SettingsBinding {
	return &SettingsBinding{
		path:     config.DefaultPath(),
		onChange: onChange,
	}
}

// GetSettings returns the current settings.
func (s *SettingsBinding) GetSettings() config.Settings {
	cfg, _ := config.LoadFrom(s.path)
	return cfg
}

// UpdateSettings saves new settings, broadcasts change event, and triggers onChange.
func (s *SettingsBinding) UpdateSettings(cfg config.Settings) error {
	if err := config.SaveTo(s.path, cfg); err != nil {
		return err
	}
	// Broadcast to all windows for cross-window sync
	if app := application.Get(); app != nil {
		app.Event.Emit("settings-changed", cfg)
	}
	if s.onChange != nil {
		s.onChange(cfg)
	}
	return nil
}
