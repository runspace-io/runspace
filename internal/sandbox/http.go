package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/git"
	"github.com/runspace/runspace/internal/workspace"
)

type Authorizer interface {
	CanRead(context.Context, string, string) error
}
type RepositoryCatalog interface {
	ListRepositories(context.Context, string, string) ([]workspace.Repository, error)
}
type DiffProvider interface {
	Diff(context.Context, string) (string, error)
	ChangedFiles(context.Context, string) ([]git.Change, error)
	FileContents(context.Context, string, string) (string, string, error)
}

type Handler struct {
	service    *Service
	authorizer Authorizer
	git        DiffProvider
	catalog    RepositoryCatalog
}

func NewHandler(service *Service, authorizer Authorizer, catalog RepositoryCatalog, provider DiffProvider) *Handler {
	return &Handler{service: service, authorizer: authorizer, catalog: catalog, git: provider}
}

func (h *Handler) Routes() chi.Router {
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	return router
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/workspaces/{workspaceID}/resources/{repositoryID}/tree", h.tree)
	router.Get("/workspaces/{workspaceID}/resources/{repositoryID}/file", h.file)
	router.Get("/workspaces/{workspaceID}/resources/{repositoryID}/diff", h.diff)
	router.Get("/workspaces/{workspaceID}/resources/{repositoryID}/changes", h.changes)
	router.Get("/workspaces/{workspaceID}/repositories/{repositoryID}/tree", h.tree)
	router.Get("/workspaces/{workspaceID}/repositories/{repositoryID}/file", h.file)
	router.Get("/workspaces/{workspaceID}/repositories/{repositoryID}/diff", h.diff)
	router.Get("/workspaces/{workspaceID}/repositories/{repositoryID}/changes", h.changes)
}

func (h *Handler) diff(writer http.ResponseWriter, request *http.Request) {
	if err := h.authorize(request, chi.URLParam(request, "workspaceID")); err != nil {
		writeError(writer, err)
		return
	}
	root, err := h.diffRoot(request)
	if err != nil {
		writeError(writer, err)
		return
	}
	path := request.URL.Query().Get("path")
	diff, err := h.git.Diff(request.Context(), root)
	if path != "" {
		original, modified, contentErr := h.git.FileContents(request.Context(), root, path)
		if contentErr != nil {
			writeError(writer, contentErr)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]string{"path": path, "original": original, "modified": modified})
		return
	}
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"path": path, "original": "", "modified": diff})
}

func (h *Handler) diffRoot(request *http.Request) (string, error) {
	if h.git == nil || h.catalog == nil {
		return "", errors.New("change provider unavailable")
	}
	workspaceID, repositoryID := chi.URLParam(request, "workspaceID"), chi.URLParam(request, "repositoryID")
	repositories, err := h.catalog.ListRepositories(request.Context(), auth.UserID(request), workspaceID)
	if err != nil {
		return "", err
	}
	for _, repository := range repositories {
		if repository.ID == repositoryID {
			return h.service.resolver.Root(request.Context(), workspaceID, repositoryID)
		}
	}
	return "", errors.New("resource is not connected to workspace")
}

func (h *Handler) changes(writer http.ResponseWriter, request *http.Request) {
	if err := h.authorize(request, chi.URLParam(request, "workspaceID")); err != nil {
		writeError(writer, err)
		return
	}
	if h.git == nil || h.catalog == nil {
		writeError(writer, errors.New("change provider unavailable"))
		return
	}
	repositories, err := h.catalog.ListRepositories(request.Context(), auth.UserID(request), chi.URLParam(request, "workspaceID"))
	if err != nil {
		writeError(writer, err)
		return
	}
	connected := false
	for _, repository := range repositories {
		if repository.ID == chi.URLParam(request, "repositoryID") {
			connected = true
			break
		}
	}
	if !connected {
		writeError(writer, errors.New("resource is not connected to workspace"))
		return
	}
	root, err := h.service.resolver.Root(request.Context(), chi.URLParam(request, "workspaceID"), chi.URLParam(request, "repositoryID"))
	if err != nil {
		writeError(writer, err)
		return
	}
	items, err := h.git.ChangedFiles(request.Context(), root)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"changes": items})
}

func (h *Handler) tree(writer http.ResponseWriter, request *http.Request) {
	if err := h.authorize(request, chi.URLParam(request, "workspaceID")); err != nil {
		writeError(writer, err)
		return
	}
	items, err := h.service.Tree(request.Context(), chi.URLParam(request, "workspaceID"), chi.URLParam(request, "repositoryID"), request.URL.Query().Get("path"))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"entries": items})
}

func (h *Handler) file(writer http.ResponseWriter, request *http.Request) {
	if err := h.authorize(request, chi.URLParam(request, "workspaceID")); err != nil {
		writeError(writer, err)
		return
	}
	item, err := h.service.Read(request.Context(), chi.URLParam(request, "workspaceID"), chi.URLParam(request, "repositoryID"), request.URL.Query().Get("path"))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, item)
}

func (h *Handler) authorize(request *http.Request, workspaceID string) error {
	if h.authorizer == nil {
		return workspace.ErrUnauthorized
	}
	return h.authorizer.CanRead(request.Context(), workspaceID, auth.UserID(request))
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
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrInvalidPath), errors.Is(err, ErrBinary), errors.Is(err, ErrTooLarge), errors.Is(err, ErrSymlink):
		status = http.StatusBadRequest
	case errors.Is(err, git.ErrInvalidDiffPath), errors.Is(err, git.ErrDiffTooLarge),
		errors.Is(err, git.ErrBinaryDiff), errors.Is(err, git.ErrChangeNotFound):
		status = http.StatusBadRequest
	}
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}
