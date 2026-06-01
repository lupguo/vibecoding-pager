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

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pager", "settings.json")
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
