package agentregistry

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) recordTaskStream(writer http.ResponseWriter, request *http.Request) {
	var update TaskStreamUpdate
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&update) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	if err := h.service.RecordTaskStream(
		request.Context(), userID(request), chi.URLParam(request, "taskID"), update,
	); err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusAccepted, map[string]string{"status": "recorded"})
}

func (h *Handler) listTaskMessages(writer http.ResponseWriter, request *http.Request) {
	messages, err := h.service.ListTaskMessages(
		request.Context(), userID(request), chi.URLParam(request, "taskID"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"messages": messages})
}

func (h *Handler) listTaskQuestions(writer http.ResponseWriter, request *http.Request) {
	questions, answerable, err := h.service.ListTaskQuestions(
		request.Context(), userID(request), chi.URLParam(request, "taskID"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"questions": questions, "can_answer": answerable,
	})
}

func (h *Handler) answerTaskQuestion(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		OptionID string `json:"option_id"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil {
		writeError(writer, ErrInvalidInput)
		return
	}
	question, err := h.service.AnswerTaskQuestion(
		request.Context(), userID(request), chi.URLParam(request, "taskID"),
		chi.URLParam(request, "questionID"), payload.OptionID,
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, question)
}
