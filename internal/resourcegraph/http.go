package resourcegraph

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
	mcp     *MCPHandler
}

func NewHandler(service *Service, discussions ...DiscussionReader) *Handler {
	return &Handler{service: service, mcp: NewMCPHandler(service, discussions...)}
}

func (h *Handler) SetAgentMessageWriter(writer AgentMessageWriter) {
	h.mcp.messages = writer
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/ui/components", h.listUIComponents)
	router.Get("/ui/components/{component}", h.getUIComponent)
	router.Get("/workspaces/{workspaceID}/graph/nodes", h.listNodes)
	router.Post("/workspaces/{workspaceID}/graph/nodes", h.upsertNode)
	router.Get("/workspaces/{workspaceID}/graph/nodes/{nodeID}", h.getContext)
	router.Post("/workspaces/{workspaceID}/graph/nodes/{nodeID}/query", h.queryCapability)
	router.Get("/workspaces/{workspaceID}/graph/nodes/{nodeID}/availability", h.capabilityAvailability)
	router.Post("/workspaces/{workspaceID}/ui/artifacts", h.createUIArtifact)
	router.Post("/workspaces/{workspaceID}/ui/actions", h.requestUIAction)
	router.Post("/workspaces/{workspaceID}/graph/edges", h.createEdge)
	router.Handle("/workspaces/{workspaceID}/mcp", h.mcp)
	router.Handle("/workspaces/{workspaceID}/channels/{channelID}/mcp", h.mcp)
}

func (h *Handler) listNodes(writer http.ResponseWriter, request *http.Request) {
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	nodes, err := h.service.ListNodes(
		request.Context(), graphUserID(request), chi.URLParam(request, "workspaceID"),
		Query{
			Kind: Kind(request.URL.Query().Get("kind")), Type: request.URL.Query().Get("type"),
			Text: request.URL.Query().Get("q"), ThreadID: request.URL.Query().Get("thread_id"),
			Limit: limit,
		},
	)
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	writeGraphJSON(writer, http.StatusOK, map[string]any{"nodes": nodes})
}

func (h *Handler) getContext(writer http.ResponseWriter, request *http.Request) {
	result, err := h.service.GetContext(
		request.Context(), graphUserID(request), chi.URLParam(request, "workspaceID"),
		graphNodeID(request),
	)
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	writeGraphJSON(writer, http.StatusOK, result)
}

func graphNodeID(request *http.Request) string {
	value, err := url.PathUnescape(chi.URLParam(request, "nodeID"))
	if err != nil {
		return ""
	}
	return value
}

func (h *Handler) upsertNode(writer http.ResponseWriter, request *http.Request) {
	var node Node
	if err := decodeGraphBody(request, &node); err != nil {
		writeGraphError(writer, err)
		return
	}
	node.WorkspaceID = chi.URLParam(request, "workspaceID")
	saved, err := h.service.UpsertNode(request.Context(), graphUserID(request), node)
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	writeGraphJSON(writer, http.StatusCreated, saved)
}

func (h *Handler) createEdge(writer http.ResponseWriter, request *http.Request) {
	var edge Edge
	if err := decodeGraphBody(request, &edge); err != nil {
		writeGraphError(writer, err)
		return
	}
	edge.WorkspaceID = chi.URLParam(request, "workspaceID")
	saved, err := h.service.CreateEdge(request.Context(), graphUserID(request), edge)
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	writeGraphJSON(writer, http.StatusCreated, saved)
}

func graphUserID(request *http.Request) string {
	return strings.TrimSpace(request.Header.Get("X-User-ID"))
}

func decodeGraphBody(request *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalid
	}
	return nil
}

func writeGraphJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeGraphError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrInvalid):
		status = http.StatusBadRequest
	}
	writeGraphJSON(writer, status, map[string]string{"error": err.Error()})
}
