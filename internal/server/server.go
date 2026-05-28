package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"pager/internal/event"
	"pager/internal/registry"
)

const ListenAddr = "127.0.0.1:7421"

// Server handles HTTP requests from bridge processes.
type Server struct {
	reg *registry.Registry
}

// New creates a Server with the given registry.
func New(reg *registry.Registry) *Server {
	return &Server{reg: reg}
}

// Start launches the HTTP server in a background goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/event", s.HandleEvent)
	mux.HandleFunc("/sessions", s.HandleSessions)
	go func() {
		log.Printf("[pager-server] listening on %s", ListenAddr)
		if err := http.ListenAndServe(ListenAddr, mux); err != nil {
			log.Printf("[pager-server] error: %v", err)
		}
	}()
}

// HandleEvent processes incoming AgentEvent POST requests.
func (s *Server) HandleEvent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var e event.AgentEvent
	if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.reg.Apply(&e)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// HandleSessions returns all sessions as JSON (debug endpoint).
func (s *Server) HandleSessions(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.reg.ListSorted())
}
