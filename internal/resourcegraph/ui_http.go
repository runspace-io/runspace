package resourcegraph

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type uiArtifactRequest struct {
	Document  UIDocument `json:"document"`
	ThreadID  string     `json:"thread_id"`
	ChannelID string     `json:"channel_id"`
}

type uiActionRequest struct {
	Operation string `json:"operation"`
	Resource  string `json:"resource"`
	Reason    string `json:"reason"`
	ThreadID  string `json:"thread_id"`
	ChannelID string `json:"channel_id"`
}

func (h *Handler) listUIComponents(writer http.ResponseWriter, _ *http.Request) {
	writeGraphJSON(writer, http.StatusOK, map[string]any{
		"version": uiVersion, "layouts": UILayouts(), "components": UIComponents(),
	})
}

func (h *Handler) getUIComponent(writer http.ResponseWriter, request *http.Request) {
	component, err := uiComponentSchema(chi.URLParam(request, "component"))
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	writeGraphJSON(writer, http.StatusOK, component)
}

func (h *Handler) createUIArtifact(writer http.ResponseWriter, request *http.Request) {
	var input uiArtifactRequest
	if decodeGraphBody(request, &input) != nil || ValidateUIDocument(input.Document) != nil {
		writeGraphError(writer, ErrInvalid)
		return
	}
	userID, entityID := graphUserID(request), newMCPID("ui")
	node, err := h.service.UpsertNode(request.Context(), userID, Node{
		ID: "artifact:" + entityID, WorkspaceID: chi.URLParam(request, "workspaceID"),
		Kind: KindArtifact, Type: "ui_artifact", Title: input.Document.Title,
		Summary: "Interactive Runspace UI artifact", OwnerID: userID,
		Metadata: map[string]any{
			"entity_id": entityID, "thread_id": input.ThreadID,
			"channel_id": input.ChannelID, "ui_document": input.Document,
		},
	})
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	writeGraphJSON(writer, http.StatusCreated, node)
}

func (h *Handler) requestUIAction(writer http.ResponseWriter, request *http.Request) {
	var input uiActionRequest
	if decodeGraphBody(request, &input) != nil || input.Operation == "" || input.Resource == "" {
		writeGraphError(writer, ErrInvalid)
		return
	}
	userID, entityID := graphUserID(request), newMCPID("action")
	node, err := h.service.UpsertNode(request.Context(), userID, Node{
		ID: "policy:" + entityID, WorkspaceID: chi.URLParam(request, "workspaceID"),
		Kind: KindPolicy, Type: "ui_action_request",
		Title: "Action requested: " + input.Operation, Summary: input.Reason, OwnerID: userID,
		Metadata: map[string]any{
			"entity_id": entityID, "operation": input.Operation,
			"resource": input.Resource, "status": "pending",
			"thread_id": input.ThreadID, "channel_id": input.ChannelID,
		},
	})
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	writeGraphJSON(writer, http.StatusAccepted, node)
}
