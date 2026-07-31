package resourceplugin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/workspace"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/resource-plugins", h.listPlugins)
	router.Get("/workspaces/{workspaceID}/resource-connections", h.listConnections)
	router.Post("/workspaces/{workspaceID}/resource-connections", h.connect)
}

func (h *Handler) listPlugins(writer http.ResponseWriter, request *http.Request) {
	if callerID(request) == "" {
		writeError(writer, workspace.ErrUnauthorized)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"plugins": Manifests()})
}

func (h *Handler) listConnections(writer http.ResponseWriter, request *http.Request) {
	items, err := h.service.List(
		request.Context(), callerID(request), chi.URLParam(request, "workspaceID"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"connections": items})
}

func (h *Handler) connect(writer http.ResponseWriter, request *http.Request) {
	var input ConnectRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		writeError(writer, ErrInvalid)
		return
	}
	item, err := h.service.Connect(
		request.Context(), callerID(request), chi.URLParam(request, "workspaceID"), input,
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, item)
}

func callerID(request *http.Request) string {
	return strings.TrimSpace(request.Header.Get("X-User-ID"))
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, workspace.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, workspace.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, workspace.ErrNotFound), errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, workspace.ErrInvalidInput), errors.Is(err, ErrInvalid):
		status = http.StatusBadRequest
	}
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}
