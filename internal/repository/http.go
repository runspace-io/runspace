package repository

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/workspaces/{workspaceID}/resources/{repositoryID}/clone", h.clone)
	router.Post("/workspaces/{workspaceID}/repositories/{repositoryID}/clone", h.clone)
}
func (h *Handler) clone(writer http.ResponseWriter, request *http.Request) {
	result, err := h.service.Clone(request.Context(), strings.TrimSpace(request.Header.Get("X-User-ID")), chi.URLParam(request, "workspaceID"), chi.URLParam(request, "repositoryID"))
	if err != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]string{"error": err.Error()})
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(result)
}
