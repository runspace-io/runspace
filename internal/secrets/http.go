package secrets

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/collaboration"
)

type Handler struct{ store Store }

func NewHandler(store Store) *Handler { return &Handler{store: store} }
func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/channels/{channelID}/secrets", h.list)
	router.Put("/channels/{channelID}/secrets/{name}", h.set)
	router.Delete("/channels/{channelID}/secrets/{name}", h.delete)
}

type request struct {
	Value string `json:"value"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context(), userID(r), chi.URLParam(r, "channelID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": items})
}
func (h *Handler) set(w http.ResponseWriter, r *http.Request) {
	var payload request
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&payload) != nil {
		writeError(w, ErrInvalid)
		return
	}
	err := h.store.Set(r.Context(), userID(r), chi.URLParam(r, "channelID"), chi.URLParam(r, "name"), payload.Value)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Delete(r.Context(), userID(r), chi.URLParam(r, "channelID"), chi.URLParam(r, "name")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func userID(r *http.Request) string { return auth.UserID(r) }
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, collaboration.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, collaboration.ErrNotFound), errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrInvalid):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
