package terminal

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type Authorizer interface {
	CanWrite(context.Context, string, string) error
}

type RootResolver interface {
	Root(context.Context, string, string) (string, error)
}

type Handler struct {
	factory    Factory
	resolver   RootResolver
	authorizer Authorizer
	upgrader   websocket.Upgrader
}

func NewHandler(factory Factory, resolver RootResolver, authorizer Authorizer) *Handler {
	return &Handler{factory: factory, resolver: resolver, authorizer: authorizer, upgrader: websocket.Upgrader{
		ReadBufferSize:  4 * 1024,
		WriteBufferSize: 16 * 1024,
		CheckOrigin:     sameOrigin,
	}}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/workspaces/{workspaceID}/resources/{repositoryID}/terminal", h.serve)
	router.Get("/workspaces/{workspaceID}/repositories/{repositoryID}/terminal", h.serve)
}

func (h *Handler) serve(writer http.ResponseWriter, request *http.Request) {
	workspaceID := chi.URLParam(request, "workspaceID")
	repositoryID := chi.URLParam(request, "repositoryID")
	userID := terminalUserID(request)
	if h.invalidAuthorization(request, workspaceID, userID) {
		http.Error(writer, "workspace authorization required", http.StatusUnauthorized)
		return
	}
	if err := h.authorizer.CanWrite(request.Context(), workspaceID, userID); err != nil {
		http.Error(writer, err.Error(), http.StatusForbidden)
		return
	}
	root, err := h.resolver.Root(request.Context(), workspaceID, repositoryID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	connection, err := h.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	defer func() { _ = connection.Close() }()
	connection.SetReadLimit(maxInputBytes + 1024)
	command := request.URL.Query().Get("command")
	if strings.TrimSpace(command) == "" {
		command = "sh"
	}
	session, err := h.factory.Open(request.Context(), OpenRequest{WorkspaceID: workspaceID, RepositoryID: repositoryID, Root: root, Command: command})
	if err != nil {
		_ = connection.WriteJSON(frame{Type: "error", Data: err.Error()})
		return
	}
	defer func() { _ = session.Close() }()
	h.stream(connection, session)
}

func (h *Handler) invalidAuthorization(request *http.Request, workspaceID, userID string) bool {
	return h.factory == nil || h.resolver == nil || h.authorizer == nil || workspaceID == "" || userID == ""
}

func (h *Handler) stream(connection *websocket.Conn, session Session) {
	inputs := make(chan []byte)
	readErrors := make(chan error, 1)
	go readInputs(connection, inputs, readErrors)
	for {
		select {
		case data, open := <-inputs:
			if !open {
				return
			}
			if !writeInput(connection, session, data) {
				return
			}
		case data, open := <-session.Output():
			if !open {
				return
			}
			if !writeOutput(connection, data) {
				return
			}
		case <-readErrors:
			return
		}
	}
}

func writeInput(connection *websocket.Conn, session Session, data []byte) bool {
	if err := session.Input(data); err != nil {
		_ = connection.WriteJSON(frame{Type: "error", Data: err.Error()})
		return false
	}
	return true
}

func writeOutput(connection *websocket.Conn, data []byte) bool {
	return connection.WriteJSON(frame{Type: "output", Data: string(data)}) == nil
}

func readInputs(connection *websocket.Conn, inputs chan<- []byte, errors chan<- error) {
	defer close(inputs)
	for {
		kind, payload, err := connection.ReadMessage()
		if err != nil {
			errors <- err
			return
		}
		if kind != websocket.TextMessage && kind != websocket.BinaryMessage {
			continue
		}
		inputs <- decodeInput(payload)
	}
}

type frame struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func decodeInput(payload []byte) []byte {
	var item frame
	if json.Unmarshal(payload, &item) == nil && item.Type == "input" {
		return []byte(item.Data)
	}
	return payload
}

func terminalUserID(request *http.Request) string {
	if userID := strings.TrimSpace(request.Header.Get("X-User-ID")); userID != "" {
		return userID
	}
	return strings.TrimSpace(request.URL.Query().Get("user_id"))
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	return origin == "" || strings.EqualFold(origin, "http://"+request.Host) || strings.EqualFold(origin, "https://"+request.Host)
}
