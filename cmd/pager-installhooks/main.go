package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/pflag"
)

// main runs the installer pipeline:
//
//  1. (install path) legacy cleanup — sweep ~/.<agent>/settings.local.json
//     etc. for stale pager entries left by earlier installer versions.
//  2. For each target in `targets`, fetch its hook list via
//     `pager-bridge --print-hooks --agent <Label>`, merge into the agent's
//     settings.json with append+idempotent semantics, and write it back.
//
// Flags:
//
//	--agent <label>  only operate on one agent (default: all)
//	--bridge <path>  pager-bridge binary path (auto-resolves to absolute)
//	--dry-run        print resulting JSON, do not write
//	--uninstall      remove all pager entries (also sweeps legacy paths)
//	--verbose        per-target log line
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
	pflag.BoolVar(&verbose, "verbose", false, "print per-target operation log")
	pflag.Parse()

	// Resolve bridgePath to an absolute path. Hook configs are read by agents
	// from arbitrary working directories (Codex from project_grace, CC from
	// the user's repo, etc.), so a relative `command` in hooks.json fires
	// `exit 127` and the event silently disappears. Defensive: even if the
	// caller (Makefile, user, CI) passes a relative path, we make sure the
	// JSON we write always has an absolute one.
	if abs, err := filepath.Abs(bridgePath); err == nil {
		bridgePath = abs
	}

	// Legacy cleanup phase: scan primary settings.json files for any pager
	// entries left over from earlier versions (which wrote there directly),
	// and remove them. User's non-pager entries are preserved.
	// Skipped on --dry-run (don't mutate disk).
	if !uninstall && !dryRun {
		for _, t := range legacyCleanupTargets {
			path := expandPath(t.SettingsFile)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				continue
			}
			settings, err := readSettings(path, t.Format)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s legacy] read error: %v\n", t.AgentLabel, err)
				continue
			}
			cleaned := removeAllPagerHooks(settings)
			if err := writeSettings(path, t.Format, cleaned); err != nil {
				fmt.Fprintf(os.Stderr, "[%s legacy] write error: %v\n", t.AgentLabel, err)
				continue
			}
			if verbose {
				fmt.Printf("[%s legacy] cleaned pager entries in %s\n", t.AgentLabel, path)
			}
		}
	}

	// Uninstall: also clean legacy paths so old entries don't linger after removal.
	if uninstall && !dryRun {
		for _, t := range legacyCleanupTargets {
			path := expandPath(t.SettingsFile)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				continue
			}
			settings, err := readSettings(path, t.Format)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s legacy] read error: %v\n", t.AgentLabel, err)
				continue
			}
			cleaned := removeAllPagerHooks(settings)
			if err := writeSettings(path, t.Format, cleaned); err != nil {
				fmt.Fprintf(os.Stderr, "[%s legacy] write error: %v\n", t.AgentLabel, err)
				continue
			}
			if verbose {
				fmt.Printf("[%s legacy] uninstalled pager entries from %s\n", t.AgentLabel, path)
			}
		}
	}

	for _, t := range targets {
		if agentFilter != "" && t.AgentLabel != agentFilter {
			continue
		}
		if err := processTarget(t, bridgePath, uninstall, dryRun, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] error: %v\n", t.AgentLabel, err)
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

func processTarget(t Target, bridgePath string, uninstall, dryRun, verbose bool) error {
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
		fmt.Printf("[%s] would write to %s:\n%s\n", t.AgentLabel, path, string(buf))
		return nil
	}
	if err := writeSettings(path, t.Format, settings); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if verbose {
		op := "installed"
		if uninstall {
			op = "uninstalled"
		}
		fmt.Printf("[%s] %s %d events at %s\n", t.AgentLabel, op, len(specs), path)
	}
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
