package bridge

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/lupguo/vibecoding-pager/internal/domain/entity"
	infralog "github.com/lupguo/vibecoding-pager/internal/infra/log"
)

const serverURL = "http://127.0.0.1:7421/event"

var posterLog = infralog.Module("bridge.poster")

// PostEvent posts an AgentEvent to the in-process Pager HTTP server.
// Fire-and-forget by design: hooks must never block the agent or affect
// its decisions. 1s timeout, all failures (network, marshal, 4xx/5xx)
// are logged via slog (module=bridge.poster) but never returned.
func PostEvent(e entity.AgentEvent) {
	body, err := json.Marshal(e)
	if err != nil {
		posterLog.Warn("marshal event", "err", err, "event_type", e.EventType, "tool", e.ToolName)
		return
	}
	client := &http.Client{Timeout: 1 * time.Second}
	req, err := http.NewRequest(http.MethodPost, serverURL, bytes.NewReader(body))
	if err != nil {
		posterLog.Warn("build request", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		posterLog.Warn("post event", "err", err, "event_type", e.EventType)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		posterLog.Warn("server rejected event",
			"status", resp.StatusCode, "event_type", e.EventType)
	}
}
