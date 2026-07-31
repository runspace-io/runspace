package realtime

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/runspace/runspace/internal/contracts"
)

type Authorizer interface {
	CanRead(context.Context, string, string) error
}

type Handler struct {
	hub        Hub
	authorizer Authorizer
	upgrader   websocket.Upgrader
	replayer   interface {
		Replay(context.Context, string, string) ([]contracts.EventEnvelope, error)
	}
}

func NewHandler(hub Hub, authorizer Authorizer, replayers ...interface {
	Replay(context.Context, string, string) ([]contracts.EventEnvelope, error)
}) *Handler {
	var replayer interface {
		Replay(context.Context, string, string) ([]contracts.EventEnvelope, error)
	}
	if len(replayers) > 0 {
		replayer = replayers[0]
	}
	return &Handler{
		hub:        hub,
		authorizer: authorizer,
		replayer:   replayer,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4 * 1024,
			WriteBufferSize: 16 * 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	workspaceID := strings.TrimSpace(request.URL.Query().Get("workspace_id"))
	userID := realtimeUserID(request)
	if !validSubscription(workspaceID, userID, h.authorizer) {
		http.Error(writer, "workspace authorization required", http.StatusUnauthorized)
		return
	}
	if err := h.authorizer.CanRead(request.Context(), workspaceID, userID); err != nil {
		http.Error(writer, err.Error(), http.StatusForbidden)
		return
	}
	lastEventID := strings.TrimSpace(request.URL.Query().Get("last_event_id"))
	if h.replayer != nil {
		replayed, err := h.replayer.Replay(request.Context(), workspaceID, lastEventID)
		if err != nil {
			http.Error(writer, "realtime replay unavailable", http.StatusServiceUnavailable)
			return
		}
		if sink, ok := h.hub.(HistorySink); ok {
			sink.AddHistory(replayed)
		}
	}
	connection, err := h.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	defer func() { _ = connection.Close() }()
	events, unsubscribe, err := h.hub.SubscribeSince(request.Context(), workspaceID, lastEventID)
	if err != nil {
		return
	}
	defer unsubscribe()
	h.stream(request, connection, events)
}

// Browser WebSocket clients cannot set arbitrary headers; the query fallback
// keeps the transport usable while the production session middleware evolves.
func realtimeUserID(request *http.Request) string {
	if userID := strings.TrimSpace(request.Header.Get("X-User-ID")); userID != "" {
		return userID
	}
	return strings.TrimSpace(request.URL.Query().Get("user_id"))
}

func validSubscription(workspaceID, userID string, authorizer Authorizer) bool {
	return workspaceID != "" && userID != "" && authorizer != nil
}

func (h *Handler) stream(request *http.Request, connection *websocket.Conn, events <-chan contracts.EventEnvelope) {
	for {
		select {
		case <-request.Context().Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			if err := connection.WriteJSON(eventFrame{Type: "event", Event: event}); err != nil {
				return
			}
		}
	}
}

type eventFrame struct {
	Type  string                  `json:"type"`
	Event contracts.EventEnvelope `json:"event"`
}
