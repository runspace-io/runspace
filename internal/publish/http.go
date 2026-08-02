package publish

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/sandbox"
	"github.com/runspace/runspace/internal/workspace"
)

type Authorizer interface {
	CanWrite(context.Context, string, string) error
}
type RepositoryCatalog interface {
	ListRepositories(context.Context, string, string) ([]workspace.Repository, error)
}

type Handler struct {
	service    *Service
	authorizer Authorizer
	workspaces RepositoryCatalog
	resolver   sandbox.RootResolver
}

func NewHandler(service *Service, authorizer Authorizer, catalog RepositoryCatalog, resolver sandbox.RootResolver) *Handler {
	return &Handler{service: service, authorizer: authorizer, workspaces: catalog, resolver: resolver}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/workspaces/{workspaceID}/runs/{runID}/publish", h.publish)
}

type requestBody struct {
	RepositoryID  string `json:"repository_id"`
	Branch        string `json:"branch"`
	Base          string `json:"base"`
	CommitMessage string `json:"commit_message"`
	Title         string `json:"title"`
	Body          string `json:"body"`
}

func (h *Handler) publish(writer http.ResponseWriter, request *http.Request) {
	workspaceID := chi.URLParam(request, "workspaceID")
	userID := auth.UserID(request)
	if h.authorizer == nil {
		writePublishError(writer, errors.New("authorization required"))
		return
	}
	if err := h.authorizer.CanWrite(request.Context(), workspaceID, userID); err != nil {
		writePublishError(writer, err)
		return
	}
	var body requestBody
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writePublishError(writer, errors.New("invalid publish request"))
		return
	}
	path, repository, err := h.resolveRepository(request.Context(), userID, workspaceID, body.RepositoryID)
	if err != nil {
		writePublishError(writer, err)
		return
	}
	result, err := h.service.Publish(request.Context(), Request{ID: chi.URLParam(request, "runID"), RepositoryPath: path, Repository: repository.FullName, Branch: body.Branch, Base: body.Base, CommitMessage: body.CommitMessage, Title: body.Title, Body: body.Body})
	if err != nil {
		writePublishError(writer, err)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(result)
}

func (h *Handler) resolveRepository(ctx context.Context, userID, workspaceID, repositoryID string) (string, workspace.Repository, error) {
	if h.workspaces == nil || h.resolver == nil {
		return "", workspace.Repository{}, errors.New("repository resolution is unavailable")
	}
	repositories, err := h.workspaces.ListRepositories(ctx, userID, workspaceID)
	if err != nil {
		return "", workspace.Repository{}, err
	}
	for _, repository := range repositories {
		if repository.ID == repositoryID {
			path, err := h.resolver.Root(ctx, workspaceID, repository.ID)
			return path, repository, err
		}
	}
	return "", workspace.Repository{}, errors.New("repository is not connected to workspace")
}

func writePublishError(writer http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if strings.Contains(err.Error(), "authorization") {
		status = http.StatusUnauthorized
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": err.Error()})
}
