package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/pflag"

	infralog "github.com/lupguo/vibecoding-pager/internal/infra/log"
)

var log *slog.Logger

// main runs the installer pipeline:
//
// For each target in `targets`, fetch its hook list via
// `pager-bridge --print-hooks --agent <Label>`, merge into the agent's
// settings.json with append+idempotent semantics, and write it back.
//
// Flags:
//
//	--agent <label>  only operate on one agent (default: all)
//	--bridge <path>  pager-bridge binary path (auto-resolves to absolute)
//	--dry-run        print resulting JSON to stdout, do not write
//	--uninstall      remove pager entries from each target's settings.json
//	--verbose        bumps slog level to DEBUG (default INFO)
func main() {
	var (
		agentFilter string
		dryRun      bool
		uninstall   bool
		bridgePath  string
		verbose     bool
	)
	pflag.StringVar(&agentFilter, "agent", "", "only install for this agent label (default: all)")
	pflag.BoolVar(&dryRun, "dry-run", false, "show what would change, do not write")
	pflag.BoolVar(&uninstall, "uninstall", false, "remove pager hook entries from all settings files")
	pflag.StringVar(&bridgePath, "bridge", defaultBridgePath(), "path to pager-bridge binary")
	pflag.BoolVar(&verbose, "verbose", false, "bump log level to DEBUG (default INFO)")
	pflag.Parse()

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	infralog.Init(level)
	log = infralog.Module("installhooks")

	// Resolve bridgePath to an absolute path. Hook configs are read by agents
	// from arbitrary working directories (Codex from project_grace, CC from
	// the user's repo, etc.), so a relative `command` in hooks.json fires
	// `exit 127` and the event silently disappears. Defensive: even if the
	// caller (Makefile, user, CI) passes a relative path, we make sure the
	// JSON we write always has an absolute one.
	if abs, err := filepath.Abs(bridgePath); err == nil {
		bridgePath = abs
	}

	for _, t := range targets {
		if agentFilter != "" && t.AgentLabel != agentFilter {
			continue
		}
		if err := processTarget(t, bridgePath, uninstall, dryRun); err != nil {
			log.Error("install failed", "agent", t.AgentLabel, "err", err)
			os.Exit(1)
		}
	}
}

func defaultBridgePath() string {
	self, err := os.Executable()
	if err != nil {
		return "pager-bridge"
	}
	return filepath.Join(filepath.Dir(self), "pager-bridge")
}

func processTarget(t Target, bridgePath string, uninstall, dryRun bool) error {
	path := expandPath(t.SettingsFile)

	var specs []HookSpec
	if !uninstall {
		s, err := fetchSpecs(bridgePath, t.AgentLabel)
		if err != nil {
			return fmt.Errorf("fetch specs: %w", err)
		}
		specs = s
	}

	settings, err := readSettings(path, t.Format)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	if uninstall {
		settings = removeAllPagerHooks(settings)
	} else {
		settings = upsertHooks(settings, t.AgentLabel, specs, bridgePath)
	}

	if dryRun {
		buf, _ := json.MarshalIndent(settings, "", "  ")
		// dry-run output is the artifact itself (the user pipes / inspects it),
		// so it goes to stdout as-is rather than through slog.
		fmt.Printf("// %s would write to %s:\n%s\n", t.AgentLabel, path, string(buf))
		return nil
	}
	if err := writeSettings(path, t.Format, settings); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	op := "installed"
	if uninstall {
		op = "uninstalled"
	}
	log.Info("hooks "+op,
		"agent", t.AgentLabel,
		"events", len(specs),
		"path", path,
	)
	return nil
}

func fetchSpecs(bridgePath, agentLabel string) ([]HookSpec, error) {
	cmd := exec.Command(bridgePath, "--print-hooks", "--agent", agentLabel)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var specs []HookSpec
	if err := json.Unmarshal(bytes.TrimSpace(out), &specs); err != nil {
		return nil, err
	}
	return specs, nil
}

func readSettings(path, format string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, err
	}
	_ = format // retained for symmetry with writeSettings; current installer
	// only emits the "settings" shape ({"hooks": {<events>}}). CC, CodeBuddy
	// and Codex (~/.codex/hooks.json) all use this top-level shape — see
	// codex-rs/config/src/hook_config.rs::HooksFile.
	return top, nil
}

func writeSettings(path, format string, settings map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	_ = format
	buf, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}

func removeAllPagerHooks(settings map[string]any) map[string]any {
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return settings
	}
	for event, raw := range hooks {
		arr, _ := raw.([]any)
		filtered := make([]any, 0, len(arr))
		for _, item := range arr {
			g, err := decodeGroupFromAny(item)
			if err == nil && isPagerHookGroup(g) {
				continue
			}
			filtered = append(filtered, item)
		}
		if len(filtered) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = filtered
		}
	}
	settings["hooks"] = hooks
	return settings
}
