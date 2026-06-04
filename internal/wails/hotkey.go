package wails

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.design/x/hotkey"

	"github.com/lupguo/vibecoding-pager/internal/infra/log"
)

var (
	currentHotkey  *hotkey.Hotkey
	goroutineAlive atomic.Bool
	hotkeyLogger   = log.Module("hotkey")
	// hotkeyMu serializes RegisterHotkey across the boot timer, settings save
	// callback, and the ApplicationDidBecomeActive recovery hook so that two
	// concurrent callers cannot both Unregister + Register the listener,
	// which would leak the second goroutine.
	hotkeyMu sync.Mutex
)

var keyMap = map[string]hotkey.Key{
	"A": hotkey.KeyA, "B": hotkey.KeyB, "C": hotkey.KeyC, "D": hotkey.KeyD,
	"E": hotkey.KeyE, "F": hotkey.KeyF, "G": hotkey.KeyG, "H": hotkey.KeyH,
	"I": hotkey.KeyI, "J": hotkey.KeyJ, "K": hotkey.KeyK, "L": hotkey.KeyL,
	"M": hotkey.KeyM, "N": hotkey.KeyN, "O": hotkey.KeyO, "P": hotkey.KeyP,
	"Q": hotkey.KeyQ, "R": hotkey.KeyR, "S": hotkey.KeyS, "T": hotkey.KeyT,
	"U": hotkey.KeyU, "V": hotkey.KeyV, "W": hotkey.KeyW, "X": hotkey.KeyX,
	"Y": hotkey.KeyY, "Z": hotkey.KeyZ,
	"0": hotkey.Key0, "1": hotkey.Key1, "2": hotkey.Key2, "3": hotkey.Key3,
	"4": hotkey.Key4, "5": hotkey.Key5, "6": hotkey.Key6, "7": hotkey.Key7,
	"8": hotkey.Key8, "9": hotkey.Key9,
	"SPACE": hotkey.KeySpace, "RETURN": hotkey.KeyReturn,
	"ESCAPE": hotkey.KeyEscape, "DELETE": hotkey.KeyDelete, "TAB": hotkey.KeyTab,
	"LEFT": hotkey.KeyLeft, "RIGHT": hotkey.KeyRight,
	"UP": hotkey.KeyUp, "DOWN": hotkey.KeyDown,
	"F1": hotkey.KeyF1, "F2": hotkey.KeyF2, "F3": hotkey.KeyF3,
	"F4": hotkey.KeyF4, "F5": hotkey.KeyF5, "F6": hotkey.KeyF6,
	"F7": hotkey.KeyF7, "F8": hotkey.KeyF8, "F9": hotkey.KeyF9,
	"F10": hotkey.KeyF10, "F11": hotkey.KeyF11, "F12": hotkey.KeyF12,
}

// IsHotkeyHealthy reports whether the global hotkey listener goroutine is alive.
// Used by the app-active hook to detect silent failure modes.
func IsHotkeyHealthy() bool {
	return currentHotkey != nil && goroutineAlive.Load()
}

// RegisterHotkey registers a global hotkey that toggles the popup window.
// Safe to call repeatedly; the previous registration is unwound first.
// Concurrent callers are serialized via hotkeyMu.
func RegisterHotkey(window *application.WebviewWindow, hotkeyStr string) {
	hotkeyMu.Lock()
	defer hotkeyMu.Unlock()

	if currentHotkey != nil {
		currentHotkey.Unregister()
		hotkeyLogger.Info("hotkey unregistered")
		currentHotkey = nil
		goroutineAlive.Store(false)
	}

	mods, key, ok := ParseHotkey(hotkeyStr)
	if !ok {
		hotkeyLogger.Warn("invalid hotkey string", "key", hotkeyStr)
		return
	}

	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		hotkeyLogger.Error("hotkey register failed", "err", err, "key", hotkeyStr)
		return
	}
	currentHotkey = hk
	hotkeyLogger.Info("hotkey registered", "key", hotkeyStr)

	go func() {
		goroutineAlive.Store(true)
		defer goroutineAlive.Store(false)
		for range hk.Keydown() {
			visible := window.IsVisible()
			hotkeyLogger.Debug("hotkey fired", "visible", visible)
			if visible {
				window.Hide()
			} else {
				window.Show()
				window.Focus()
			}
		}
		hotkeyLogger.Warn("hotkey goroutine exited; channel closed", "key", hotkeyStr)
	}()
}

// ParseHotkey converts "Alt+E" format to hotkey modifiers and key.
func ParseHotkey(s string) ([]hotkey.Modifier, hotkey.Key, bool) {
	parts := strings.Split(s, "+")
	if len(parts) < 2 {
		return nil, 0, false
	}

	var mods []hotkey.Modifier
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "alt", "option":
			mods = append(mods, hotkey.ModOption)
		case "ctrl", "control":
			mods = append(mods, hotkey.ModCtrl)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "cmd", "command", "meta":
			mods = append(mods, hotkey.ModCmd)
		}
	}

	keyStr := strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
	key, ok := keyMap[keyStr]
	if !ok {
		return nil, 0, false
	}

	return mods, key, true
}
