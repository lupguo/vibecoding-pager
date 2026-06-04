package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lupguo/vibecoding-pager/internal/domain/session"
	"github.com/lupguo/vibecoding-pager/internal/adapter/httpapi"
)

// Standalone test server — runs HTTP server + tracker without Wails GUI.
// Used for integration testing the bridge → server → tracker pipeline.
func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	tracker := session.NewTracker(func(sessions []*session.Session) {
		// Log state changes
		for _, s := range sessions {
			log.Printf("[onChange] session=%s status=%s tool=%s content=%q",
				s.Key, s.Status, s.LastEvent.ToolName, s.LastEvent.Content)
		}
		// Also dump full state as JSON
		data, _ := json.MarshalIndent(sessions, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- Current Sessions ---\n%s\n---\n\n", string(data))
	})

	srv := httpapi.New(tracker)
	srv.Start()

	log.Println("[test-server] Ready. Listening on 127.0.0.1:7421")
	log.Println("[test-server] Endpoints: POST /event, GET /sessions")
	log.Println("[test-server] Press Ctrl+C to stop")

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("[test-server] Shutting down")
}
