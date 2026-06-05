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
	if format == "hooks-only" {
		return map[string]any{"hooks": top}, nil
	}
	return top, nil
}

func writeSettings(path, format string, settings map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var top any = settings
	if format == "hooks-only" {
		top = settings["hooks"]
		if top == nil {
			top = map[string]any{}
		}
	}
	buf, err := json.MarshalIndent(top, "", "  ")
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
