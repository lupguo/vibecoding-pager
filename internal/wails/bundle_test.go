package wails

import "testing"

func TestIsBundlePath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"installed app bundle", "/Applications/Pager.app/Contents/MacOS/Pager", true},
		{"build-dir app bundle", "/Users/me/repo/pager/bin/Pager.app/Contents/MacOS/Pager", true},
		{"wails dev temp binary", "/var/folders/x0/abc/T/wails-dev/pager", false},
		{"raw make-run binary", "/Users/me/repo/pager/bin/pager", false},
		{"empty path", "", false},
		{"unrelated path with .app substring", "/Users/foo/.app-template/something/MacOS/x", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBundlePath(tc.path); got != tc.want {
				t.Errorf("isBundlePath(%q) = %v; want %v", tc.path, got, tc.want)
			}
		})
	}
}
