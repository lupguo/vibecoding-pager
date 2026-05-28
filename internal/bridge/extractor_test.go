package bridge

import (
	"encoding/json"
	"testing"
)

func TestExtractContent_Bash(t *testing.T) {
	input := json.RawMessage(`{"command":"go build ./..."}`)
	raw, content := ExtractContent("Bash", input)
	if raw != "go build ./..." {
		t.Errorf("raw = %q, want %q", raw, "go build ./...")
	}
	if content != "go build ./..." {
		t.Errorf("content = %q, want %q", content, "go build ./...")
	}
}

func TestExtractContent_Edit(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/src/main.go"}`)
	raw, content := ExtractContent("Edit", input)
	if raw != "编辑 /src/main.go" {
		t.Errorf("raw = %q, want %q", raw, "编辑 /src/main.go")
	}
	if content != "编辑 /src/main.go" {
		t.Errorf("content = %q, want %q", content, "编辑 /src/main.go")
	}
}

func TestExtractContent_Write(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/tmp/out.txt"}`)
	raw, content := ExtractContent("Write", input)
	if raw != "写入 /tmp/out.txt" {
		t.Errorf("raw = %q, want %q", raw, "写入 /tmp/out.txt")
	}
	if content != "写入 /tmp/out.txt" {
		t.Errorf("content = %q, want %q", content, "写入 /tmp/out.txt")
	}
}

func TestExtractContent_Read(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/etc/hosts"}`)
	raw, content := ExtractContent("Read", input)
	if raw != "读取 /etc/hosts" {
		t.Errorf("raw = %q, want %q", raw, "读取 /etc/hosts")
	}
	if content != "读取 /etc/hosts" {
		t.Errorf("content = %q, want %q", content, "读取 /etc/hosts")
	}
}

func TestExtractContent_Glob(t *testing.T) {
	input := json.RawMessage(`{"pattern":"**/*.go"}`)
	raw, _ := ExtractContent("Glob", input)
	if raw != "查找 **/*.go" {
		t.Errorf("raw = %q, want %q", raw, "查找 **/*.go")
	}
}

func TestExtractContent_Grep(t *testing.T) {
	input := json.RawMessage(`{"pattern":"TODO"}`)
	raw, _ := ExtractContent("Grep", input)
	if raw != "搜索 TODO" {
		t.Errorf("raw = %q, want %q", raw, "搜索 TODO")
	}
}

func TestExtractContent_WebFetch(t *testing.T) {
	input := json.RawMessage(`{"url":"https://example.com"}`)
	raw, _ := ExtractContent("WebFetch", input)
	if raw != "抓取 https://example.com" {
		t.Errorf("raw = %q, want %q", raw, "抓取 https://example.com")
	}
}

func TestExtractContent_WebSearch(t *testing.T) {
	input := json.RawMessage(`{"query":"golang wails v3"}`)
	raw, _ := ExtractContent("WebSearch", input)
	if raw != "搜索 golang wails v3" {
		t.Errorf("raw = %q, want %q", raw, "搜索 golang wails v3")
	}
}

func TestExtractContent_Task(t *testing.T) {
	input := json.RawMessage(`{"description":"Run linter"}`)
	raw, _ := ExtractContent("Task", input)
	if raw != "子任务: Run linter" {
		t.Errorf("raw = %q, want %q", raw, "子任务: Run linter")
	}
}

func TestExtractContent_MCP(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("mcp__github__create_pr", input)
	if raw != "MCP: create_pr" {
		t.Errorf("raw = %q, want %q", raw, "MCP: create_pr")
	}
}

func TestExtractContent_Unknown(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("SomeNewTool", input)
	if raw != "SomeNewTool" {
		t.Errorf("raw = %q, want %q", raw, "SomeNewTool")
	}
}

func TestExtractContent_Truncation(t *testing.T) {
	longCmd := "echo 'this is a very long command that definitely exceeds sixty characters limit for display'"
	input, _ := json.Marshal(BashInput{Command: longCmd})
	raw, content := ExtractContent("Bash", json.RawMessage(input))
	if raw != longCmd {
		t.Errorf("raw should be untruncated")
	}
	runes := []rune(content)
	if len(runes) != 61 {
		t.Errorf("content rune len = %d, want 61 (60 + ellipsis)", len(runes))
	}
	if string(runes[60]) != "…" {
		t.Errorf("last rune should be ellipsis, got %q", string(runes[60]))
	}
}

func TestExtractContent_EmptyInput(t *testing.T) {
	input := json.RawMessage(`{}`)
	raw, _ := ExtractContent("Bash", input)
	if raw != "Bash" {
		t.Errorf("raw = %q, want %q", raw, "Bash")
	}
}
