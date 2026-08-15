package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AgentEvent is a single event pushed to listeners over SSE.
type AgentEvent struct {
	SessionID  string    `json:"session_id"`
	Type       string    `json:"type"` // "start" | "step" | "patch" | "done" | "error"
	Message    string    `json:"message"`
	Package    string    `json:"package,omitempty"`
	Version    string    `json:"version,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// agentBus is the in-process fan-out bus for agent session events.
var agentBus = &eventBus{subs: make(map[chan AgentEvent]struct{})}

type eventBus struct {
	mu   sync.RWMutex
	subs map[chan AgentEvent]struct{}
}

func (b *eventBus) subscribe() chan AgentEvent {
	ch := make(chan AgentEvent, 32)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *eventBus) unsubscribe(ch chan AgentEvent) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

// Publish sends an event to all connected SSE listeners.
// Called by fg-agent or the patch CLI command via POST /api/v1/agent/events.
func (b *eventBus) publish(e AgentEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default: // subscriber too slow — drop rather than block
		}
	}
}

// AgentStream streams agent session events to the caller via Server-Sent Events.
// GET /api/v1/agent/stream
//
// Clients connect with EventSource('/api/v1/agent/stream') and receive JSON-encoded
// AgentEvent objects. The connection is kept open until the client disconnects.
func (h *Handler) AgentStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // disable nginx buffering

	ch := agentBus.subscribe()
	defer agentBus.unsubscribe(ch)

	// Send a connected heartbeat immediately so the client knows the stream is live.
	connected := AgentEvent{Type: "connected", Message: "stream connected", OccurredAt: time.Now()}
	if b, err := json.Marshal(connected); err == nil {
		fmt.Fprintf(c.Writer, "data: %s\n\n", b)
		c.Writer.Flush()
	}

	// Keepalive ticker — prevents proxies from closing idle connections.
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": keepalive\n\n")
			c.Writer.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", b)
			c.Writer.Flush()
		}
	}
}

// PublishAgentEvent accepts an agent event from fg-agent and fans it out to all SSE listeners.
// POST /api/v1/agent/events   body: AgentEvent JSON
func (h *Handler) PublishAgentEvent(c *gin.Context) {
	var ev AgentEvent
	if err := c.ShouldBindJSON(&ev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now()
	}
	agentBus.publish(ev)
	c.JSON(http.StatusOK, gin.H{"ok": true, "subscribers": len(agentBus.subs)})
}
