package filesync

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/workspace"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/workspaces/{workspaceID}/resources/{repositoryID}/sync", h.register)
	router.Get("/workspaces/{workspaceID}/resources/{repositoryID}/sync", h.status)
	router.Post("/workspaces/{workspaceID}/repositories/{repositoryID}/sync", h.register)
	router.Get("/workspaces/{workspaceID}/repositories/{repositoryID}/sync", h.status)
}

func (h *Handler) register(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		DeviceID   string   `json:"device_id"`
		DeviceName string   `json:"device_name"`
		Addresses  []string `json:"addresses"`
		Branch     string   `json:"branch"`
		Git        bool     `json:"git"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20)).Decode(&body); err != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	session, err := h.service.Register(request.Context(), RegisterRequest{
		UserID:       userID(request),
		WorkspaceID:  chi.URLParam(request, "workspaceID"),
		RepositoryID: chi.URLParam(request, "repositoryID"),
		DeviceID:     body.DeviceID,
		DeviceName:   body.DeviceName,
		Addresses:    body.Addresses,
		Branch:       body.Branch,
		Git:          body.Git,
	})
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, session)
}

func (h *Handler) status(writer http.ResponseWriter, request *http.Request) {
	session, err := h.service.Status(
		request.Context(),
		userID(request),
		chi.URLParam(request, "workspaceID"),
		chi.URLParam(request, "repositoryID"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, session)
}

func userID(request *http.Request) string {
	return auth.UserID(request)
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, ErrNotFound), errors.Is(err, workspace.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, workspace.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, workspace.ErrForbidden):
		status = http.StatusForbidden
	}
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}
