package resourcegraph

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (h *MCPHandler) uiTool(
	request *http.Request, userID string, scope mcpScope, name string, args map[string]any,
) (any, error) {
	switch name {
	case "ui.list_components":
		return map[string]any{
			"version": uiVersion, "layouts": UILayouts(), "components": UIComponents(),
		}, nil
	case "ui.get_component_schema":
		return uiComponentSchema(stringArg(args, "component"))
	case "ui.create_artifact":
		return h.createUIArtifact(request, userID, scope, args)
	case "ui.update_artifact":
		return h.updateUIArtifact(request, userID, scope, args)
	case "ui.request_action":
		return h.requestUIAction(request, userID, scope, args)
	default:
		return nil, fmt.Errorf("%w: unknown UI tool", ErrInvalid)
	}
}

func uiComponentSchema(name string) (UIComponentDefinition, error) {
	for _, component := range uiComponents {
		if component.Name == name {
			return component, nil
		}
	}
	return UIComponentDefinition{}, ErrNotFound
}

func (h *MCPHandler) createUIArtifact(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (Node, error) {
	document, err := uiDocumentArg(args)
	if err != nil {
		return Node{}, err
	}
	entityID := newMCPID("ui")
	node, err := h.service.UpsertNode(request.Context(), userID, Node{
		ID: "artifact:" + entityID, WorkspaceID: scope.WorkspaceID,
		Kind: KindArtifact, Type: "ui_artifact", Title: document.Title,
		Summary: "Interactive Runspace UI artifact", OwnerID: userID,
		Metadata: map[string]any{
			"entity_id": entityID, "thread_id": scopedArg(args, "thread_id", scope.ThreadID),
			"channel_id": scope.ChannelID, "ui_document": document,
		},
	})
	if err != nil {
		return Node{}, err
	}
	if h.messages != nil && scope.ThreadID != "" && scope.AgentID != "" {
		_, _ = h.messages.RecordOutput(
			request.Context(), userID, scope.WorkspaceID, scope.ThreadID,
			scope.AgentID, "[[ui:"+node.ID+"]]",
		)
	}
	return node, nil
}

func (h *MCPHandler) updateUIArtifact(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (Node, error) {
	nodeID := stringArg(args, "node_id")
	context, err := h.service.GetContext(request.Context(), userID, scope.WorkspaceID, nodeID)
	if err != nil {
		return Node{}, err
	}
	if context.Node.Type != "ui_artifact" || context.Node.OwnerID != userID {
		return Node{}, ErrForbidden
	}
	document, err := uiDocumentArg(args)
	if err != nil {
		return Node{}, err
	}
	context.Node.Title = document.Title
	context.Node.Metadata["ui_document"] = document
	return h.service.UpsertNode(request.Context(), userID, context.Node)
}

func (h *MCPHandler) requestUIAction(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (Node, error) {
	operation := stringArg(args, "operation")
	if operation == "" || stringArg(args, "resource") == "" {
		return Node{}, ErrInvalid
	}
	entityID := newMCPID("action")
	return h.service.UpsertNode(request.Context(), userID, Node{
		ID: "policy:" + entityID, WorkspaceID: scope.WorkspaceID,
		Kind: KindPolicy, Type: "ui_action_request",
		Title: "Action requested: " + operation, Summary: stringArg(args, "reason"),
		OwnerID: userID,
		Metadata: map[string]any{
			"entity_id": entityID, "operation": operation,
			"resource": stringArg(args, "resource"), "status": "pending",
			"thread_id": scope.ThreadID, "channel_id": scope.ChannelID,
		},
	})
}

func uiDocumentArg(args map[string]any) (UIDocument, error) {
	encoded, err := json.Marshal(args["document"])
	if err != nil {
		return UIDocument{}, ErrInvalid
	}
	var document UIDocument
	if json.Unmarshal(encoded, &document) != nil || ValidateUIDocument(document) != nil {
		return UIDocument{}, ErrInvalid
	}
	return document, nil
}
