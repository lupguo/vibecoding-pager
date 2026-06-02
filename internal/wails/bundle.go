package wails

import (
	"os"
	"strings"
)

// runningInBundle reports whether the current process is launched from inside a
// macOS .app bundle (e.g. `open build/bin/Pager.app`). Returns false when the
// binary runs unbundled — typically `wails dev` or `make run`. Used to gate the
// Wails NotificationService, which calls into UNUserNotificationCenter and
// hard-fails on Startup with "notifications require a valid bundle identifier"
// when [NSBundle mainBundle].bundleIdentifier is nil.
//
// On any error (Executable() failure, non-darwin OS), conservatively returns
// false so the unbundled-mode skip path is taken — never the panicking path.
func runningInBundle() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return isBundlePath(exe)
}

// isBundlePath is the pure half of runningInBundle, split out for testing.
// A canonical bundled binary lives at `<some path>/<Name>.app/Contents/MacOS/<Name>`,
// so the substring `.app/Contents/MacOS/` uniquely identifies the bundle layout.
func isBundlePath(exePath string) bool {
	return strings.Contains(exePath, ".app/Contents/MacOS/")
}
