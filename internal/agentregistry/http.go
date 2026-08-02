package agentregistry

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/auth"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/users/me/agents", h.list)
	router.Put("/users/me/agents/{agentID}", h.upsert)
	router.Get("/workspaces/{workspaceID}/agents", h.directory)
	router.Post("/threads/{threadID}/agent-messages", h.recordOutput)
	router.Post("/threads/{threadID}/agent-activity", h.recordActivity)
	router.Get("/agent-tasks/{taskID}/grants", h.listTaskGrants)
	router.Put("/agent-tasks/{taskID}/grants/{principalID}", h.grantTaskAccess)
	router.Put("/agent-tasks/{taskID}", h.upsertTask)
	router.Get("/workspaces/{workspaceID}/agent-tasks", h.listTasks)
	router.Post("/agent-tasks/{taskID}/input", h.inputTask)
	router.Post("/agent-tasks/{taskID}/cancel", h.cancelTask)
	router.Post("/agent-tasks/{taskID}/artifacts", h.shareTaskArtifact)
	router.Get("/agent-tasks/{taskID}/messages", h.listTaskMessages)
	router.Get("/agent-tasks/{taskID}/questions", h.listTaskQuestions)
	router.Post("/agent-tasks/{taskID}/questions/{questionID}/answer", h.answerTaskQuestion)
	router.Post("/users/me/agents/presence", h.presence)
	router.Post("/users/me/agent-tasks/{taskID}/events", h.recordTaskStream)
}

func (h *Handler) presence(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Agents []Installation `json:"agents"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	for _, item := range payload.Agents {
		if _, err := h.service.Upsert(request.Context(), userID(request), item); err != nil {
			writeError(writer, err)
			return
		}
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "online"})
}

func (h *Handler) recordOutput(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		WorkspaceID string `json:"workspace_id"`
		AgentID     string `json:"agent_id"`
		Body        string `json:"body"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	message, err := h.service.RecordOutput(
		request.Context(), userID(request), payload.WorkspaceID,
		chi.URLParam(request, "threadID"), payload.AgentID, payload.Body,
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, message)
}

func (h *Handler) recordActivity(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		WorkspaceID string         `json:"workspace_id"`
		AgentID     string         `json:"agent_id"`
		Status      ActivityStatus `json:"status"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	message, err := h.service.RecordActivity(
		request.Context(), userID(request), payload.WorkspaceID,
		chi.URLParam(request, "threadID"), payload.AgentID, payload.Status,
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, message)
}

func (h *Handler) grantTaskAccess(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		WorkspaceID string     `json:"workspace_id"`
		AgentID     string     `json:"agent_id"`
		Role        string     `json:"role"`
		ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	grant, err := h.service.GrantTaskAccess(request.Context(), userID(request), TaskGrant{
		TaskID: chi.URLParam(request, "taskID"), WorkspaceID: payload.WorkspaceID,
		AgentID: payload.AgentID, PrincipalID: chi.URLParam(request, "principalID"),
		Role: payload.Role, ExpiresAt: payload.ExpiresAt,
	})
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, grant)
}

func (h *Handler) listTaskGrants(writer http.ResponseWriter, request *http.Request) {
	items, err := h.service.ListTaskGrants(
		request.Context(), userID(request), request.URL.Query().Get("workspace_id"),
		chi.URLParam(request, "taskID"), request.URL.Query().Get("agent_id"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"grants": items})
}

func (h *Handler) upsertTask(writer http.ResponseWriter, request *http.Request) {
	var task AgentTask
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&task) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	task.ID = chi.URLParam(request, "taskID")
	saved, err := h.service.UpsertTask(request.Context(), userID(request), task)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, saved)
}

func (h *Handler) listTasks(writer http.ResponseWriter, request *http.Request) {
	items, err := h.service.ListTasks(
		request.Context(), userID(request), chi.URLParam(request, "workspaceID"),
		request.URL.Query().Get("thread_id"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"tasks": items})
}

func (h *Handler) inputTask(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Input string `json:"input"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	outputs, err := h.service.InputTask(
		request.Context(), userID(request), chi.URLParam(request, "taskID"), payload.Input,
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"outputs": outputs})
}

func (h *Handler) cancelTask(writer http.ResponseWriter, request *http.Request) {
	if err := h.service.CancelTask(
		request.Context(), userID(request), chi.URLParam(request, "taskID"),
	); err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) shareTaskArtifact(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	message, err := h.service.ShareTaskArtifact(
		request.Context(), userID(request), chi.URLParam(request, "taskID"), payload.Body,
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, message)
}

func (h *Handler) directory(writer http.ResponseWriter, request *http.Request) {
	items, err := h.service.Directory(
		request.Context(), userID(request), chi.URLParam(request, "workspaceID"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"agents": items})
}

func (h *Handler) list(writer http.ResponseWriter, request *http.Request) {
	items, err := h.service.List(request.Context(), userID(request))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"agents": items})
}

func (h *Handler) upsert(writer http.ResponseWriter, request *http.Request) {
	var item Installation
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&item) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	item.ID = chi.URLParam(request, "agentID")
	saved, err := h.service.Upsert(request.Context(), userID(request), item)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, saved)
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
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, ErrTaskUnavailable):
		status = http.StatusNotFound
	case errors.Is(err, ErrQuestionResolved):
		status = http.StatusConflict
	}
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}
