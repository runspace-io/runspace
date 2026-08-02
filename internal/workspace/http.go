package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes the workspace service over the versioned REST API. The
// authenticated user is supplied by the auth middleware through X-User-ID in
// production middleware should set the same request context value.
type Handler struct{ service Service }

type contextKey struct{}

// WithUserID lets authentication middleware pass the verified identity without
// exposing it as a client-controlled header.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, strings.TrimSpace(id))
}

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/workspaces", h.listWorkspaces)
	r.Post("/workspaces", h.createWorkspace)
	r.Get("/workspaces/{workspaceID}", h.getWorkspace)
	r.Get("/workspaces/{workspaceID}/resources", h.listResources)
	r.Post("/workspaces/{workspaceID}/resources", h.connectResource)
	// Deprecated route aliases retained for existing clients.
	r.Get("/workspaces/{workspaceID}/repositories", h.listRepositories)
	r.Post("/workspaces/{workspaceID}/repositories", h.connectRepository)
	r.Post("/workspaces/{workspaceID}/members", h.addMember)
	r.Get("/workspaces/{workspaceID}/members", h.listMembers)
	h.registerInvitationRoutes(r)
}

type workspaceRequest = CreateWorkspaceRequest
type resourceRequest = ConnectResourceRequest
type repositoryRequest = ConnectRepositoryRequest
type memberRequest struct {
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
}

func userID(r *http.Request) string {
	if id, ok := r.Context().Value(contextKey{}).(string); ok && id != "" {
		return id
	}
	return strings.TrimSpace(r.Header.Get("X-User-ID"))
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, ErrAlreadyExists):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
func decodeBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrInvalidInput
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return ErrInvalidInput
	}
	return nil
}

func (h *Handler) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListWorkspaces(r.Context(), userID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	summaries := make([]workspaceSummary, 0, len(items))
	for _, item := range items {
		resources, resourceErr := h.service.ListResources(r.Context(), userID(r), item.ID)
		if resourceErr != nil {
			writeError(w, resourceErr)
			return
		}
		summaries = append(summaries, workspaceSummary{
			Workspace: item, ResourceCount: len(resources), RepositoryCount: len(resources),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": summaries})
}

type workspaceSummary struct {
	Workspace
	ResourceCount   int `json:"resource_count"`
	RepositoryCount int `json:"repository_count"`
}

func (h *Handler) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var req workspaceRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	ws, err := h.service.CreateWorkspace(r.Context(), userID(r), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}
func (h *Handler) getWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, err := h.service.GetWorkspace(r.Context(), userID(r), chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}
func (h *Handler) listRepositories(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListRepositories(r.Context(), userID(r), chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": items})
}
func (h *Handler) listResources(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListResources(r.Context(), userID(r), chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"resources": items})
}
func (h *Handler) connectResource(w http.ResponseWriter, r *http.Request) {
	var req resourceRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	resource, err := h.service.ConnectResource(r.Context(), userID(r), chi.URLParam(r, "workspaceID"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resource)
}
func (h *Handler) connectRepository(w http.ResponseWriter, r *http.Request) {
	var req repositoryRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	repo, err := h.service.ConnectRepository(r.Context(), userID(r), chi.URLParam(r, "workspaceID"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	var req memberRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	member, err := h.service.AddMember(r.Context(), userID(r), chi.URLParam(r, "workspaceID"), req.UserID, req.Role)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.service.ListMembers(r.Context(), userID(r), chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}
