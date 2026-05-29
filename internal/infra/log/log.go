package log

import (
	"log/slog"
	"os"
)

var defaultLevel = &slog.LevelVar{}

// Init sets up the global slog handler with text output to stderr.
func Init(level slog.Level) {
	defaultLevel.Set(level)
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: defaultLevel,
	})
	slog.SetDefault(slog.New(handler))
}

// Module returns a logger with a "module" attribute for filtering.
func Module(name string) *slog.Logger {
	return slog.Default().With("module", name)
}
