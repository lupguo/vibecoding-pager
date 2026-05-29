package wails

import (
	"pager/internal/infra/config"
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

// UpdateSettings saves new settings and triggers onChange.
func (s *SettingsBinding) UpdateSettings(cfg config.Settings) error {
	if err := config.SaveTo(s.path, cfg); err != nil {
		return err
	}
	if s.onChange != nil {
		s.onChange(cfg)
	}
	return nil
}
