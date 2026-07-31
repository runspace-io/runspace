package runs

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/collaboration"
	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/sandbox"
	"github.com/runspace/runspace/internal/workspace"
)

type Handler struct {
	service    *Service
	authorizer *workspace.MemoryService
	chat       collaboration.Service
	resolver   sandbox.RootResolver
}

func NewHandler(
	service *Service,
	authorizer *workspace.MemoryService,
	chat collaboration.Service,
	resolver sandbox.RootResolver,
) *Handler {
	return &Handler{service: service, authorizer: authorizer, chat: chat, resolver: resolver}
}

func (h *Handler) Routes() chi.Router {
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	return router
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/threads/{threadID}/runs", h.create)
	router.Get("/threads/{threadID}/runs", h.list)
	router.Get("/runs/{runID}", h.get)
	router.Get("/runs/{runID}/outputs", h.outputs)
	router.Post("/runs/{runID}/start", h.start)
	router.Post("/runs/{runID}/stop", h.stop)
	router.Post("/runs/{runID}/input", h.input)
	router.Post("/runs/{runID}/retry", h.retry)
}

type createRequest struct {
	RunID       string `json:"run_id"`
	WorkspaceID string `json:"workspace_id"`
	Repository  string `json:"repository"`
	Prompt      string `json:"prompt"`
}

func (h *Handler) create(writer http.ResponseWriter, request *http.Request) {
	var payload createRequest
	if err := decodeRunJSON(request, &payload); err != nil {
		writeRunError(writer, err)
		return
	}
	if err := h.authorize(request, payload.WorkspaceID, true); err != nil {
		writeRunError(writer, err)
		return
	}
	spawn := contracts.SpawnRequest{
		RunID: payload.RunID, ThreadID: chi.URLParam(request, "threadID"),
		WorkspaceID: payload.WorkspaceID, Repository: payload.Repository, Prompt: payload.Prompt,
	}
	userID := strings.TrimSpace(request.Header.Get("X-User-ID"))
	if err := h.resolveSpawnContext(request.Context(), userID, &spawn); err != nil {
		writeRunError(writer, err)
		return
	}
	if err := h.assignWorkingDirectory(request.Context(), userID, &spawn); err != nil {
		writeRunError(writer, err)
		return
	}
	run, err := h.service.Create(request.Context(), spawn)
	if err != nil {
		writeRunError(writer, err)
		return
	}
	writeRunJSON(writer, http.StatusAccepted, run)
}

func (h *Handler) list(writer http.ResponseWriter, request *http.Request) {
	items, err := h.service.ListRuns(request.Context(), chi.URLParam(request, "threadID"))
	if err != nil {
		writeRunError(writer, err)
		return
	}
	if len(items) > 0 {
		if err := h.authorize(request, items[0].WorkspaceID, false); err != nil {
			writeRunError(writer, err)
			return
		}
	} else if h.chat != nil {
		userID := strings.TrimSpace(request.Header.Get("X-User-ID"))
		threads, err := h.chat.ListThreads(request.Context(), userID, strings.TrimSpace(request.URL.Query().Get("workspace_id")))
		if err != nil {
			writeRunError(writer, err)
			return
		}
		found := false
		for _, thread := range threads {
			if thread.ID == chi.URLParam(request, "threadID") {
				if err := h.authorize(request, thread.WorkspaceID, false); err != nil {
					writeRunError(writer, err)
					return
				}
				found = true
				break
			}
		}
		if !found {
			writeRunError(writer, errors.New("thread not found"))
			return
		}
	}
	writeRunJSON(writer, http.StatusOK, map[string]any{"runs": items})
}

func (h *Handler) get(writer http.ResponseWriter, request *http.Request) {
	run, err := h.service.Get(request.Context(), chi.URLParam(request, "runID"))
	if err != nil {
		writeRunError(writer, err)
		return
	}
	if err := h.authorize(request, run.WorkspaceID, false); err != nil {
		writeRunError(writer, err)
		return
	}
	writeRunJSON(writer, http.StatusOK, run)
}

func (h *Handler) outputs(writer http.ResponseWriter, request *http.Request) {
	run, err := h.service.Get(request.Context(), chi.URLParam(request, "runID"))
	if err != nil {
		writeRunError(writer, err)
		return
	}
	if err := h.authorize(request, run.WorkspaceID, false); err != nil {
		writeRunError(writer, err)
		return
	}
	items, err := h.service.ListOutputs(request.Context(), run.ID)
	if err != nil {
		writeRunError(writer, err)
		return
	}
	writeRunJSON(writer, http.StatusOK, map[string]any{"outputs": items})
}

func (h *Handler) start(writer http.ResponseWriter, request *http.Request) {
	if err := h.authorizeExisting(request, false); err != nil {
		writeRunError(writer, err)
		return
	}
	run, err := h.service.Start(request.Context(), chi.URLParam(request, "runID"))
	h.writeMutation(writer, run, err)
}

func (h *Handler) stop(writer http.ResponseWriter, request *http.Request) {
	if err := h.authorizeExisting(request, true); err != nil {
		writeRunError(writer, err)
		return
	}
	run, err := h.service.Stop(request.Context(), chi.URLParam(request, "runID"))
	h.writeMutation(writer, run, err)
}

func (h *Handler) retry(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		RunID string `json:"run_id"`
	}
	if err := decodeRunJSON(request, &payload); err != nil {
		writeRunError(writer, err)
		return
	}
	old, err := h.service.Get(request.Context(), chi.URLParam(request, "runID"))
	if err != nil {
		writeRunError(writer, err)
		return
	}
	if err := h.authorize(request, old.WorkspaceID, true); err != nil {
		writeRunError(writer, err)
		return
	}
	spawn := retrySpawnRequest(old, payload.RunID)
	userID := strings.TrimSpace(request.Header.Get("X-User-ID"))
	if err := h.assignWorkingDirectory(request.Context(), userID, &spawn); err != nil {
		writeRunError(writer, err)
		return
	}
	run, err := h.service.Create(request.Context(), spawn)
	h.writeMutation(writer, run, err)
}

func (h *Handler) input(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := decodeRunJSON(request, &payload); err != nil {
		writeRunError(writer, err)
		return
	}
	if err := h.authorizeExisting(request, true); err != nil {
		writeRunError(writer, err)
		return
	}
	if err := h.service.Input(request.Context(), chi.URLParam(request, "runID"), payload.Text); err != nil {
		writeRunError(writer, err)
		return
	}
	writeRunJSON(writer, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *Handler) writeMutation(writer http.ResponseWriter, run Run, err error) {
	if err != nil {
		writeRunError(writer, err)
		return
	}
	writeRunJSON(writer, http.StatusAccepted, run)
}

func (h *Handler) authorizeExisting(request *http.Request, write bool) error {
	run, err := h.service.Get(request.Context(), chi.URLParam(request, "runID"))
	if err != nil {
		return err
	}
	return h.authorize(request, run.WorkspaceID, write)
}

func (h *Handler) authorize(request *http.Request, workspaceID string, write bool) error {
	if h.authorizer == nil {
		return workspace.ErrUnauthorized
	}
	userID := strings.TrimSpace(request.Header.Get("X-User-ID"))
	if write {
		return h.authorizer.CanWrite(request.Context(), workspaceID, userID)
	}
	return h.authorizer.CanRead(request.Context(), workspaceID, userID)
}

func decodeRunJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid run request")
	}
	return nil
}

func writeRunJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeRunError(writer http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, workspace.ErrUnauthorized) {
		status = http.StatusUnauthorized
	}
	if errors.Is(err, workspace.ErrForbidden) {
		status = http.StatusForbidden
	}
	writeRunJSON(writer, status, map[string]string{"error": err.Error()})
}
