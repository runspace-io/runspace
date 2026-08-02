package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"

	"github.com/runspace/runspace/internal/auth"
)

// Handler exposes the workspace service over the versioned REST API. The
// caller's identity comes from the verified token the auth middleware puts on
// the request context, never from a header the client controls.
type Handler struct{ service Service }

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

func userID(r *http.Request) string { return auth.UserID(r) }
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
	summaries, err := h.workspaceSummaries(r.Context(), userID(r), items)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": summaries})
}

// workspaceSummaries counts each workspace's resources concurrently.
//
// This ran one resource-count query per workspace, sequentially, on every
// workspace list load — an N+1 that costs roughly N round trips of wall time.
// It only shows up at scale: a person with a handful of workspaces never
// notices, a workspace accumulating a hundred does. Running the same calls
// concurrently costs one round trip instead of N, without touching the
// storage layer or its authorization checks.
func (h *Handler) workspaceSummaries(
	ctx context.Context, caller string, items []Workspace,
) ([]workspaceSummary, error) {
	summaries := make([]workspaceSummary, len(items))
	var group errgroup.Group
	for index, item := range items {
		index, item := index, item
		group.Go(func() error {
			resources, err := h.service.ListResources(ctx, caller, item.ID)
			if err != nil {
				return err
			}
			summaries[index] = workspaceSummary{
				Workspace: item, ResourceCount: len(resources), RepositoryCount: len(resources),
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return summaries, nil
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
