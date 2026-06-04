package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	Language           string              `json:"language"`
	Theme              string              `json:"theme"`
	Opacity            int                 `json:"opacity"`
	HotkeyToggle       string              `json:"hotkey_toggle"`
	NotificationLevel  string              `json:"notification_level"`
	PopupWidth         int                 `json:"popup_width"`
	PopupPinned        bool                `json:"popup_pinned"`
	SessionLoadHours   int                 `json:"session_load_hours"`
	NotificationEvents map[string][]string `json:"notification_events"`
	CollapsedProjects  []string            `json:"collapsed_projects"`
}

func Defaults() Settings {
	return Settings{
		Language:          "zh",
		Theme:             "system",
		Opacity:           75,
		HotkeyToggle:     "Alt+E",
		NotificationLevel: "attention_only",
		PopupWidth:        380,
		PopupPinned:       false,
		SessionLoadHours:  24,
		NotificationEvents: map[string][]string{
			"CC": {
				"StopFailure",
				"Notification",
				"PermissionRequest",
				"PostToolUseFailure",
				"Elicitation",
			},
			"CC-INT": {
				"StopFailure",
				"Notification",
			},
			"CodeBuddy": {
				"StopFailure",
				"Notification",
				"PermissionRequest",
				"PostToolUseFailure",
				"Elicitation",
			},
		},
		CollapsedProjects: []string{},
	}
}

// BaseDir returns the directory under which all VibeCoding Pager runtime data
// (settings.json, pager.db, WAL files) lives. Resolution order:
//
//  1. PAGER_CONFIG_DIR env var (highest priority — used by `make dev`,
//     `make run`, and tests for isolation)
//  2. ~/Library/Application Support/VibeCoding Pager/ (macOS-native production default)
//
// The directory is NOT created here; callers should MkdirAll on first write.
func BaseDir() string {
	if envDir := os.Getenv("PAGER_CONFIG_DIR"); envDir != "" {
		return envDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "VibeCoding Pager")
}

// DefaultPath returns the absolute path to settings.json under BaseDir().
func DefaultPath() string {
	return filepath.Join(BaseDir(), "settings.json")
}

// DefaultDBPath returns the absolute path to pager.db under BaseDir().
func DefaultDBPath() string {
	return filepath.Join(BaseDir(), "pager.db")
}

func LoadFrom(path string) (Settings, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Defaults(), err
	}
	return cfg, nil
}

func SaveTo(path string, cfg Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
