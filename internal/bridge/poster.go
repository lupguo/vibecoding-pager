package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"pager/internal/event"
)

const serverURL = "http://127.0.0.1:7421/event"

// PostEvent sends an AgentEvent to the Pager server.
// Timeout is 1 second. Failures are silent (stderr warning only).
func PostEvent(e event.AgentEvent) {
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 1 * time.Second}
	req, err := http.NewRequest(http.MethodPost, serverURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[pager-bridge] warn: %v\n", err)
		return
	}
	defer resp.Body.Close()
}
