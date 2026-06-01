package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"pager/internal/domain/entity"
	"pager/internal/domain/session"
	"pager/internal/infra/store"
)

const ListenAddr = "127.0.0.1:7421"

// Server handles HTTP requests from bridge processes.
type Server struct {
	tracker *session.Tracker
	store   store.EventStore
	httpSrv *http.Server
}

// New creates a Server with the given tracker and optional event store.
func New(tracker *session.Tracker, opts ...ServerOption) *Server {
	s := &Server{tracker: tracker}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServerOption configures the Server.
type ServerOption func(*Server)

// WithStore sets the EventStore for persistent event recording.
func WithStore(es store.EventStore) ServerOption {
	return func(s *Server) { s.store = es }
}

// Start launches the HTTP server in a background goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/event", s.HandleEvent)
	mux.HandleFunc("/sessions", s.HandleSessions)
	mux.HandleFunc("/debug-log", s.HandleDebugLog)
	s.httpSrv = &http.Server{Addr: ListenAddr, Handler: mux}
	go func() {
		log.Printf("[pager-server] listening on %s", ListenAddr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[pager-server] error: %v", err)
		}
	}()
}

// Stop gracefully shuts down the HTTP server with a 2-second deadline.
// Safe to call multiple times; no-op if Start was never called.
func (s *Server) Stop() error {
	if s.httpSrv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(ctx)
}

// HandleEvent processes incoming AgentEvent POST requests.
func (s *Server) HandleEvent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var e entity.AgentEvent
	if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.tracker.TrackEvent(&e)

	if s.store != nil {
		s.store.Record(&e)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// HandleSessions returns all sessions as JSON (debug endpoint).
func (s *Server) HandleSessions(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.tracker.ListByRecent())
}

// HandleDebugLog receives debug messages from the frontend and logs them.
func (s *Server) HandleDebugLog(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(req.Body)
	log.Printf("[frontend-debug] %s", string(body))
	w.WriteHeader(http.StatusOK)
}
