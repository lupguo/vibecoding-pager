package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lupguo/vibecoding-pager/internal/adapter/httpapi"
	"github.com/lupguo/vibecoding-pager/internal/domain/session"
	infralog "github.com/lupguo/vibecoding-pager/internal/infra/log"
)

// Standalone test server — runs HTTP server + tracker without Wails GUI.
// Used for integration testing the bridge → server → tracker pipeline.
func main() {
	infralog.Init(slog.LevelInfo)
	log := infralog.Module("testserver")

	tracker := session.NewTracker(func(sessions []*session.Session) {
		for _, s := range sessions {
			log.Info("session changed",
				"session_key", s.Key,
				"status", s.Status,
				"tool", s.LastEvent.ToolName,
				"content", s.LastEvent.Content,
			)
		}
		log.Debug("tracker snapshot", "sessions", sessions)
	})

	srv := httpapi.New(tracker)
	srv.Start()

	log.Info("test-server ready", "addr", "127.0.0.1:7421",
		"endpoints", "POST /event, GET /sessions")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info("test-server shutting down")
}
