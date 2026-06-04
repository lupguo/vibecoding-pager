package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	if cfg.PopupWidth != 380 {
		t.Errorf("expected default popup_width 380, got %d", cfg.PopupWidth)
	}
	if cfg.PopupPinned != false {
		t.Errorf("expected default popup_pinned false, got %v", cfg.PopupPinned)
	}
	if cfg.PopupWidth != 380 {
		t.Errorf("expected default popup_width 380, got %d", cfg.PopupWidth)
	}
	if cfg.PopupPinned != false {
		t.Errorf("expected default popup_pinned false, got %v", cfg.PopupPinned)
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
		PopupWidth:        450,
		PopupPinned:       true,
	}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	if !reflect.DeepEqual(loaded, cfg) {
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
	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("expected defaults on corruption, got %+v", cfg)
	}
}

func TestDefaults_NotificationEvents(t *testing.T) {
	cfg := Defaults()
	if cfg.NotificationEvents == nil {
		t.Fatal("NotificationEvents should not be nil")
	}
	ccEvents, ok := cfg.NotificationEvents["CC"]
	if !ok {
		t.Fatal("expected CC key in NotificationEvents")
	}
	found := false
	for _, e := range ccEvents {
		if e == "StopFailure" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("CC events should include StopFailure, got: %v", ccEvents)
	}
}

func TestLoadFrom_WithNotificationEvents(t *testing.T) {
	tmp := t.TempDir() + "/settings.json"
	data := []byte(`{"notification_events":{"CC":["Stop","Notification"]}}`)
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NotificationEvents["CC"]) != 2 {
		t.Errorf("expected 2 events, got %d", len(cfg.NotificationEvents["CC"]))
	}
}

func TestCollapsedProjectsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	cfg := Defaults()
	cfg.CollapsedProjects = []string{"foo", "bar"}
	if err := SaveTo(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.CollapsedProjects) != 2 || loaded.CollapsedProjects[0] != "foo" {
		t.Errorf("CollapsedProjects = %v; want [foo bar]", loaded.CollapsedProjects)
	}
}

func TestBaseDir_Override(t *testing.T) {
	t.Setenv("PAGER_CONFIG_DIR", "/tmp/pager-test")
	if got := BaseDir(); got != "/tmp/pager-test" {
		t.Errorf("BaseDir() = %q, want %q", got, "/tmp/pager-test")
	}
}

func TestBaseDir_Default(t *testing.T) {
	t.Setenv("PAGER_CONFIG_DIR", "")
	got := BaseDir()
	if !strings.HasSuffix(got, "/Library/Application Support/VibeCoding Pager") {
		t.Errorf("BaseDir() = %q, want path ending in '/Library/Application Support/VibeCoding Pager'", got)
	}
}

func TestDefaultPath_UsesBaseDir(t *testing.T) {
	t.Setenv("PAGER_CONFIG_DIR", "/tmp/pager-test")
	if got := DefaultPath(); got != "/tmp/pager-test/settings.json" {
		t.Errorf("DefaultPath() = %q, want %q", got, "/tmp/pager-test/settings.json")
	}
}

func TestDefaultDBPath_UsesBaseDir(t *testing.T) {
	t.Setenv("PAGER_CONFIG_DIR", "/tmp/pager-test")
	if got := DefaultDBPath(); got != "/tmp/pager-test/pager.db" {
		t.Errorf("DefaultDBPath() = %q, want %q", got, "/tmp/pager-test/pager.db")
	}
}
