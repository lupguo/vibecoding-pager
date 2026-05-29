package terminal

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// JumpRequest contains the info needed to activate a terminal tab.
type JumpRequest struct {
	TTY            string
	TermProgram    string
	ITermSessionID string
}

// Jump activates the terminal tab matching the given request.
func Jump(req JumpRequest) error {
	switch {
	case strings.Contains(req.TermProgram, "iTerm"):
		return jumpITerm(req)
	case req.TermProgram == "Apple_Terminal":
		return jumpAppleTerminal(req.TTY)
	case strings.Contains(req.TermProgram, "WezTerm"):
		return jumpWezTerm(req.TTY)
	default:
		return activateApp(req.TermProgram)
	}
}

func jumpITerm(req JumpRequest) error {
	var script string
	if req.ITermSessionID != "" {
		// ITERM_SESSION_ID format is "w0t0p0:GUID" but iTerm2's AppleScript
		// "unique id" is just the GUID part. Strip the prefix.
		guid := req.ITermSessionID
		if idx := strings.Index(guid, ":"); idx >= 0 {
			guid = guid[idx+1:]
		}
		script = fmt.Sprintf(`
tell application "iTerm2"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      repeat with s in sessions of t
        if (unique id of s) is "%s" then
          select w
          tell t to select
          tell s to select
          return
        end if
      end repeat
    end repeat
  end repeat
end tell`, guid)
	} else {
		script = fmt.Sprintf(`
tell application "iTerm2"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      repeat with s in sessions of t
        if (tty of s) is "%s" then
          select w
          tell t to select
          tell s to select
          return
        end if
      end repeat
    end repeat
  end repeat
end tell`, req.TTY)
	}
	return runOsa(script)
}

func jumpAppleTerminal(tty string) error {
	script := fmt.Sprintf(`
tell application "Terminal"
  activate
  repeat with w in windows
    repeat with t in tabs of w
      if (tty of t) is "%s" then
        set selected of t to true
        set index of w to 1
        return
      end if
    end repeat
  end repeat
end tell`, tty)
	return runOsa(script)
}

func jumpWezTerm(_ string) error {
	return activateApp("WezTerm")
}

func activateApp(termProgram string) error {
	app := strings.TrimSuffix(termProgram, ".app")
	if app == "" {
		app = "Terminal"
	}
	return runOsa(fmt.Sprintf(`tell application "%s" to activate`, app))
}

func runOsa(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[terminal] osascript error: %v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return err
}
