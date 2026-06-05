package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildBinary compiles cmd/pager-bridge into a temp binary for use in tests.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "pager-bridge")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestPrintHooks_CC(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "--print-hooks", "--agent", "CC").Output()
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	var hooks []struct {
		Event   string `json:"event"`
		Matcher string `json:"matcher"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &hooks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(hooks) != 23 {
		t.Errorf("hooks count = %d, want 23", len(hooks))
	}
}

func TestPrintHooks_UnknownAgent(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--print-hooks", "--agent", "Nonsense")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err == nil {
		t.Error("expected non-zero exit for unknown agent")
	}
}
