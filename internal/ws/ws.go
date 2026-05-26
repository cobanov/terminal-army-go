// Package ws is a minimal event hub. The Python port polls instead of using
// websockets, so this package starts as a thin no-op that the rest of the
// codebase can grow into when push events become useful.
package ws

import (
	"context"
	"net/http"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

// Hub fan-outs broadcast events to subscribed users. The MVP implementation
// drops events on the floor; clients keep polling for state changes.
type Hub struct{}

func NewHub() *Hub { return &Hub{} }

// Run is the placeholder event loop. It blocks until ctx is cancelled so the
// caller can `go hub.Run(ctx)` next to the HTTP server.
func (h *Hub) Run(ctx context.Context) {
	<-ctx.Done()
}

// Broadcast satisfies svc.EventSink.
func (h *Hub) Broadcast(userID int64, event string, payload any) {}

// Handler returns an HTTP handler for /api/v1/ws. The MVP responds 501 since
// no client uses it yet, but the route remains so we can swap in a real
// implementation without changing the route table.
func Handler(hub *Hub, app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"websocket not enabled in this build"}`, http.StatusNotImplemented)
	}
}
