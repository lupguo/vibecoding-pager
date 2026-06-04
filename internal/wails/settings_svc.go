package wails

import (
	"github.com/lupguo/vibecoding-pager/internal/infra/config"
	"github.com/lupguo/vibecoding-pager/internal/infra/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// SettingsBinding exposes user settings to the React frontend via Wails bindings.
type SettingsBinding struct {
	path     string
	onChange func(config.Settings)
	store    store.EventStore
}

// NewSettingsBinding creates a SettingsBinding.
func NewSettingsBinding(onChange func(config.Settings), eventStore store.EventStore) *SettingsBinding {
	return &SettingsBinding{
		path:     config.DefaultPath(),
		onChange: onChange,
		store:    eventStore,
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

// DataStats holds database statistics for display in settings UI.
type DataStats struct {
	DBSizeBytes int64 `json:"db_size_bytes"`
	EventCount  int64 `json:"event_count"`
}

// GetDataStats returns current database size and event count.
func (s *SettingsBinding) GetDataStats() DataStats {
	if s.store == nil {
		return DataStats{}
	}
	size, count, _ := s.store.Stats()
	return DataStats{DBSizeBytes: size, EventCount: count}
}

// PurgeData physically deletes events and sessions older than N days.
// Pass 0 to delete all data.
func (s *SettingsBinding) PurgeData(days int) (int64, error) {
	if s.store == nil {
		return 0, nil
	}
	if days == 0 {
		return s.store.PurgeAll()
	}
	return s.store.PurgeOlderThan(days)
}
