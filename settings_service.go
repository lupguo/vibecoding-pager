package main

import (
	"pager/internal/settings"
)

// SettingsService exposes user settings to the React frontend via Wails bindings.
type SettingsService struct {
	path     string
	onChange func(settings.Settings)
}

func NewSettingsService(onChange func(settings.Settings)) *SettingsService {
	return &SettingsService{
		path:     settings.DefaultPath(),
		onChange: onChange,
	}
}

func (s *SettingsService) GetSettings() settings.Settings {
	cfg, _ := settings.LoadFrom(s.path)
	return cfg
}

func (s *SettingsService) UpdateSettings(cfg settings.Settings) error {
	if err := settings.SaveTo(s.path, cfg); err != nil {
		return err
	}
	if s.onChange != nil {
		s.onChange(cfg)
	}
	return nil
}
