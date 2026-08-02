package workspace

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// InvitationService is optional: a Service that cannot issue links simply does
// not expose the routes, which keeps the core Service interface unchanged.
type InvitationService interface {
	CreateInvitation(
		context.Context, string, string, Role, time.Duration,
	) (Invitation, string, error)
	PreviewInvitation(context.Context, string) (InvitationPreview, error)
	AcceptInvitation(context.Context, string, string) (Member, error)
	ListInvitations(context.Context, string, string) ([]Invitation, error)
	RevokeInvitation(context.Context, string, string, string) error
}

func (h *Handler) registerInvitationRoutes(r chi.Router) {
	if _, ok := h.service.(InvitationService); !ok {
		return
	}
	r.Post("/workspaces/{workspaceID}/invitations", h.createInvitation)
	r.Get("/workspaces/{workspaceID}/invitations", h.listInvitations)
	r.Delete("/workspaces/{workspaceID}/invitations/{invitationID}", h.revokeInvitation)
	// Tokens travel in the body rather than the path so they stay out of access
	// logs; the link puts the token in the browser URL, and the page posts it.
	r.Post("/invitations/preview", h.previewInvitation)
	r.Post("/invitations/accept", h.acceptInvitation)
}

func (h *Handler) invitations() InvitationService {
	service, _ := h.service.(InvitationService)
	return service
}

type invitationRequest struct {
	Role       Role `json:"role"`
	TTLSeconds int  `json:"ttl_seconds,omitempty"`
}

type tokenRequest struct {
	Token string `json:"token"`
}

func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	var body invitationRequest
	if err := decodeBody(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.Role == "" {
		body.Role = RoleMember
	}
	invitation, token, err := h.invitations().CreateInvitation(
		r.Context(), userID(r), chi.URLParam(r, "workspaceID"),
		body.Role, time.Duration(body.TTLSeconds)*time.Second,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	// The token is returned exactly once; only its hash is stored.
	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation": invitation, "token": token,
	})
}

func (h *Handler) listInvitations(w http.ResponseWriter, r *http.Request) {
	items, err := h.invitations().ListInvitations(
		r.Context(), userID(r), chi.URLParam(r, "workspaceID"),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []Invitation{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitations": items})
}

func (h *Handler) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	err := h.invitations().RevokeInvitation(
		r.Context(), userID(r), chi.URLParam(r, "workspaceID"),
		chi.URLParam(r, "invitationID"),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) previewInvitation(w http.ResponseWriter, r *http.Request) {
	var body tokenRequest
	if err := decodeBody(r, &body); err != nil {
		writeError(w, err)
		return
	}
	preview, err := h.invitations().PreviewInvitation(r.Context(), body.Token)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	var body tokenRequest
	if err := decodeBody(r, &body); err != nil {
		writeError(w, err)
		return
	}
	member, err := h.invitations().AcceptInvitation(r.Context(), userID(r), body.Token)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, member)
}
