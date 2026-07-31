package resourcegraph

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type MCPHandler struct {
	service      *Service
	discussions  DiscussionReader
	messages     AgentMessageWriter
	capabilities CapabilityQuerier
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *mcpError `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewMCPHandler(service *Service, discussions ...DiscussionReader) *MCPHandler {
	handler := &MCPHandler{service: service}
	if len(discussions) > 0 {
		handler.discussions = discussions[0]
	}
	return handler
}

func (h *MCPHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		writer.Header().Set("Allow", "POST")
		writeGraphJSON(writer, http.StatusMethodNotAllowed, map[string]string{
			"error": "this stateless MCP endpoint accepts POST requests",
		})
		return
	}
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var rpc mcpRequest
	if err := decodeGraphBody(request, &rpc); err != nil || rpc.JSONRPC != "2.0" {
		h.writeRPC(writer, mcpResponse{
			JSONRPC: "2.0", ID: rpc.ID,
			Error: &mcpError{Code: -32600, Message: "invalid JSON-RPC request"},
		})
		return
	}
	response := h.dispatch(request, rpc)
	if rpc.ID == nil {
		writer.WriteHeader(http.StatusAccepted)
		return
	}
	h.writeRPC(writer, response)
}

func (h *MCPHandler) dispatch(request *http.Request, rpc mcpRequest) mcpResponse {
	scope := requestMCPScope(request)
	switch rpc.Method {
	case "initialize":
		return mcpResult(rpc.ID, initializeResult(rpc.Params, scope))
	case "ping":
		return mcpResult(rpc.ID, map[string]any{})
	case "tools/list":
		return mcpResult(rpc.ID, map[string]any{"tools": graphTools()})
	case "tools/call":
		result, err := h.callTool(request, scope, rpc.Params)
		if err != nil {
			return mcpResult(rpc.ID, toolError(err))
		}
		return mcpResult(rpc.ID, toolResult(result))
	default:
		return mcpResponse{
			JSONRPC: "2.0", ID: rpc.ID,
			Error: &mcpError{Code: -32601, Message: "method not found"},
		}
	}
}

func (h *MCPHandler) callTool(
	request *http.Request, scope mcpScope, raw json.RawMessage,
) (any, error) {
	var call struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if json.Unmarshal(raw, &call) != nil {
		return nil, ErrInvalid
	}
	userID := graphUserID(request)
	switch call.Name {
	case "search_resources":
		return h.search(request, userID, scope, call.Arguments)
	case "query_resource":
		return h.queryCapability(
			request, userID, scope.WorkspaceID, stringArg(call.Arguments, "node_id"),
			CapabilityQuery{
				Capability: stringArg(call.Arguments, "capability"),
				Query:      stringArg(call.Arguments, "query"), Limit: intArg(call.Arguments, "limit"),
			},
		)
	case "read_context":
		return h.service.GetContext(
			request.Context(), userID, scope.WorkspaceID, stringArg(call.Arguments, "node_id"),
		)
	case "read_discussion":
		return h.readDiscussion(request, userID, scope, call.Arguments)
	case "send_message":
		return h.sendMessage(request, userID, scope, call.Arguments)
	case "list_tasks":
		return h.service.ListNodes(request.Context(), userID, scope.WorkspaceID, Query{
			Kind: KindTask, ThreadID: scopedArg(call.Arguments, "thread_id", scope.ThreadID),
			Limit: 100,
		})
	case "create_task":
		return h.createTask(request, userID, scope, call.Arguments)
	case "publish_artifact":
		return h.publishArtifact(request, userID, scope, call.Arguments)
	case "request_access":
		return h.requestAccess(request, userID, scope, call.Arguments)
	default:
		return h.uiTool(request, userID, scope, call.Name, call.Arguments)
	}
}

func (h *MCPHandler) search(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (any, error) {
	limit := intArg(args, "limit")
	kind := Kind(stringArg(args, "kind"))
	return h.service.ListNodes(request.Context(), userID, scope.WorkspaceID, Query{
		Kind: kind, Text: stringArg(args, "query"),
		ThreadID: scopedSearchThread(kind, args, scope), Limit: limit,
	})
}

func (h *MCPHandler) createTask(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (any, error) {
	entityID := newMCPID("task")
	node, err := h.service.UpsertNode(request.Context(), userID, Node{
		ID: "task:" + entityID, WorkspaceID: scope.WorkspaceID, Kind: KindTask,
		Type: "workspace_task", Title: stringArg(args, "title"),
		Summary: stringArg(args, "summary"), OwnerID: userID,
		Metadata: map[string]any{
			"entity_id":  entityID,
			"thread_id":  scopedArg(args, "thread_id", scope.ThreadID),
			"channel_id": scope.ChannelID,
			"status":     "ready",
		},
	})
	if err != nil {
		return nil, err
	}
	return node, h.linkOptional(
		request, userID, scope.WorkspaceID, node.ID, args, "discussion_id", "discussed_in",
	)
}

func (h *MCPHandler) publishArtifact(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (any, error) {
	entityID := newMCPID("artifact")
	node, err := h.service.UpsertNode(request.Context(), userID, Node{
		ID: "artifact:" + entityID, WorkspaceID: scope.WorkspaceID, Kind: KindArtifact,
		Type:  defaultString(stringArg(args, "type"), "document"),
		Title: stringArg(args, "title"), Summary: stringArg(args, "summary"),
		ExternalRef: stringArg(args, "external_ref"), OwnerID: userID,
		Metadata: map[string]any{
			"entity_id":  entityID,
			"thread_id":  scopedArg(args, "thread_id", scope.ThreadID),
			"channel_id": scope.ChannelID,
		},
	})
	if err != nil {
		return nil, err
	}
	return node, h.linkOptional(
		request, userID, scope.WorkspaceID, node.ID, args, "task_id", "produced_by",
	)
}

func (h *MCPHandler) requestAccess(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (any, error) {
	entityID := newMCPID("access")
	return h.service.UpsertNode(request.Context(), userID, Node{
		ID: "policy:" + entityID, WorkspaceID: scope.WorkspaceID, Kind: KindPolicy,
		Type: "access_request", Title: "Access requested: " + stringArg(args, "capability"),
		Summary: stringArg(args, "reason"), OwnerID: userID,
		Metadata: map[string]any{
			"entity_id": entityID, "resource_id": stringArg(args, "resource_id"),
			"capability": stringArg(args, "capability"), "status": "pending",
			"thread_id": scope.ThreadID, "channel_id": scope.ChannelID,
		},
	})
}

func (h *MCPHandler) linkOptional(
	request *http.Request, userID, workspaceID, fromID string, args map[string]any,
	key, relation string,
) error {
	target := stringArg(args, key)
	if target == "" {
		return nil
	}
	_, err := h.service.CreateEdge(request.Context(), userID, Edge{
		WorkspaceID: workspaceID, FromID: fromID, ToID: target,
		Relation: relation, CreatedBy: userID,
	})
	return err
}

func (h *MCPHandler) writeRPC(writer http.ResponseWriter, response mcpResponse) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

func mcpResult(id, result any) mcpResponse {
	return mcpResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func initializeResult(raw json.RawMessage, scope mcpScope) map[string]any {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(raw, &params)
	version := defaultString(params.ProtocolVersion, "2025-11-25")
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]string{"name": "runspace-resource-graph", "version": "0.1.0"},
		"instructions":    scopeInstructions(scope),
	}
}

func toolResult(value any) map[string]any {
	encoded, _ := json.Marshal(value)
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": string(encoded)}},
		"structuredContent": value,
	}
}

func toolError(err error) map[string]any {
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": err.Error()}},
		"isError": true,
	}
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func intArg(args map[string]any, key string) int {
	value, _ := args[key].(float64)
	return int(value)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func newMCPID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
}
