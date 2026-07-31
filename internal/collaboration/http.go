package collaboration

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/workspace"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) Routes() chi.Router {
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	return router
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/workspaces/{workspaceID}/threads", h.listThreads)
	router.Post("/workspaces/{workspaceID}/threads", h.createThread)
	router.Get("/workspaces/{workspaceID}/channels", h.listChannels)
	router.Post("/workspaces/{workspaceID}/channels", h.createChannel)
	router.Patch("/workspaces/{workspaceID}/channels/{channelID}", h.updateChannel)
	router.Get("/threads/{threadID}/messages", h.listMessages)
	router.Post("/threads/{threadID}/messages", h.createMessage)
}

type threadRequest struct {
	Title     string `json:"title"`
	ChannelID string `json:"channel_id"`
}

type channelRequest struct {
	Name          string         `json:"name"`
	ParentID      string         `json:"parent_id"`
	ResourceID    string         `json:"resource_id"`
	ResourceIDs   []string       `json:"resource_ids"`
	RepositoryID  string         `json:"repository_id"`
	RepositoryIDs []string       `json:"repository_ids"`
	Config        map[string]any `json:"config"`
}

type channelPatchRequest = ChannelPatch

type messageRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Body        string `json:"body"`
	ActorType   string `json:"actor_type"`
}

func collaborationUserID(request *http.Request) string {
	return strings.TrimSpace(request.Header.Get("X-User-ID"))
}

func decodeCollaborationBody(request *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalid
	}
	return nil
}

func writeCollaborationJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeCollaborationError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrUnauthorized), errors.Is(err, workspace.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, ErrNotFound), errors.Is(err, workspace.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrInvalid), errors.Is(err, workspace.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, workspace.ErrForbidden):
		status = http.StatusForbidden
	}
	writeCollaborationJSON(writer, status, map[string]string{"error": err.Error()})
}

func (h *Handler) listThreads(writer http.ResponseWriter, request *http.Request) {
	threads, err := h.service.ListThreads(request.Context(), collaborationUserID(request), chi.URLParam(request, "workspaceID"))
	if err != nil {
		writeCollaborationError(writer, err)
		return
	}
	writeCollaborationJSON(writer, http.StatusOK, map[string]any{"threads": threads})
}

func (h *Handler) listChannels(writer http.ResponseWriter, request *http.Request) {
	channels, err := h.service.ListChannels(request.Context(), collaborationUserID(request), chi.URLParam(request, "workspaceID"))
	if err != nil {
		writeCollaborationError(writer, err)
		return
	}
	writeCollaborationJSON(writer, http.StatusOK, map[string]any{"channels": channels})
}

func (h *Handler) createChannel(writer http.ResponseWriter, request *http.Request) {
	var payload channelRequest
	if err := decodeCollaborationBody(request, &payload); err != nil {
		writeCollaborationError(writer, err)
		return
	}
	ids := payload.ResourceIDs
	if len(ids) == 0 && payload.ResourceID != "" {
		ids = []string{payload.ResourceID}
	}
	if len(ids) == 0 {
		ids = payload.RepositoryIDs
	}
	if len(ids) == 0 && payload.RepositoryID != "" {
		ids = []string{payload.RepositoryID}
	}
	channel, err := h.service.CreateChannelWithRepositories(request.Context(), collaborationUserID(request), chi.URLParam(request, "workspaceID"), payload.Name, payload.ParentID, ids, payload.Config)
	if err != nil {
		writeCollaborationError(writer, err)
		return
	}
	writeCollaborationJSON(writer, http.StatusCreated, channel)
}

func (h *Handler) updateChannel(writer http.ResponseWriter, request *http.Request) {
	var payload channelPatchRequest
	if err := decodeCollaborationBody(request, &payload); err != nil {
		writeCollaborationError(writer, err)
		return
	}
	channel, err := h.service.UpdateChannel(request.Context(), collaborationUserID(request), chi.URLParam(request, "workspaceID"), chi.URLParam(request, "channelID"), payload)
	if err != nil {
		writeCollaborationError(writer, err)
		return
	}
	writeCollaborationJSON(writer, http.StatusOK, channel)
}

func (h *Handler) createThread(writer http.ResponseWriter, request *http.Request) {
	var payload threadRequest
	if err := decodeCollaborationBody(request, &payload); err != nil {
		writeCollaborationError(writer, err)
		return
	}
	thread, err := h.service.CreateThread(request.Context(), collaborationUserID(request), chi.URLParam(request, "workspaceID"), payload.Title, payload.ChannelID)
	if err != nil {
		writeCollaborationError(writer, err)
		return
	}
	writeCollaborationJSON(writer, http.StatusCreated, thread)
}

func (h *Handler) listMessages(writer http.ResponseWriter, request *http.Request) {
	messages, err := h.service.ListMessages(request.Context(), collaborationUserID(request), request.URL.Query().Get("workspace_id"), chi.URLParam(request, "threadID"))
	if err != nil {
		writeCollaborationError(writer, err)
		return
	}
	writeCollaborationJSON(writer, http.StatusOK, map[string]any{"messages": messages})
}

func (h *Handler) createMessage(writer http.ResponseWriter, request *http.Request) {
	var payload messageRequest
	if err := decodeCollaborationBody(request, &payload); err != nil {
		writeCollaborationError(writer, err)
		return
	}
	message, err := h.service.CreateMessage(request.Context(), collaborationUserID(request), payload.WorkspaceID, chi.URLParam(request, "threadID"), payload.ActorType, payload.Body)
	if err != nil {
		writeCollaborationError(writer, err)
		return
	}
	writeCollaborationJSON(writer, http.StatusCreated, message)
}
