package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsDefaultsWhenFileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Language != "zh" {
		t.Errorf("expected default language 'zh', got '%s'", cfg.Language)
	}
	if cfg.Theme != "system" {
		t.Errorf("expected default theme 'system', got '%s'", cfg.Theme)
	}
	if cfg.Opacity != 75 {
		t.Errorf("expected default opacity 75, got %d", cfg.Opacity)
	}
	if cfg.HotkeyToggle != "Alt+E" {
		t.Errorf("expected default hotkey 'Alt+E', got '%s'", cfg.HotkeyToggle)
	}
	if cfg.NotificationLevel != "attention_only" {
		t.Errorf("expected default notification_level 'attention_only', got '%s'", cfg.NotificationLevel)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to not be created on Load")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "settings.json")

	cfg := Settings{
		Language:          "en",
		Theme:             "dark",
		Opacity:           50,
		HotkeyToggle:     "Ctrl+Shift+P",
		NotificationLevel: "all",
	}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	if loaded != cfg {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", loaded, cfg)
	}
}

func TestLoadFromCorruptedFileReturnsDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	os.WriteFile(path, []byte("{invalid json"), 0o644)

	cfg, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for corrupted JSON")
	}
	if cfg != Defaults() {
		t.Errorf("expected defaults on corruption, got %+v", cfg)
	}
}
