package resourcegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/collaboration"
)

func TestMCPListsRunspaceGraphTools(t *testing.T) {
	service := New(testAuthorizer{}, nil)
	router := chi.NewRouter()
	NewHandler(service).RegisterRoutes(router)
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	request := httptest.NewRequest(
		http.MethodPost, "/workspaces/ws_1/mcp", bytes.NewReader(body),
	)
	request = request.WithContext(auth.WithUserID(request.Context(), "nahid"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Result.Tools) != 14 {
		t.Fatalf("expected fourteen graph tools, got %#v", response.Result.Tools)
	}
}

func TestGraphContextAcceptsEncodedNamespacedNodeID(t *testing.T) {
	service := New(testAuthorizer{}, nil)
	_, err := service.UpsertNode(context.Background(), "nahid", Node{
		ID: "artifact:ui_1", WorkspaceID: "ws_1", Kind: KindArtifact,
		Type: "ui_artifact", Title: "Artifact", OwnerID: "nahid",
	})
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	NewHandler(service).RegisterRoutes(router)
	request := httptest.NewRequest(
		http.MethodGet, "/workspaces/ws_1/graph/nodes/artifact%3Aui_1", nil,
	)
	request = request.WithContext(auth.WithUserID(request.Context(), "nahid"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "artifact:ui_1") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

type testAgentMessageWriter struct {
	userID, workspaceID, threadID, agentID, body string
}

func (writer *testAgentMessageWriter) RecordOutput(
	_ context.Context, userID, workspaceID, threadID, agentID, body string,
) (collaboration.Message, error) {
	writer.userID, writer.workspaceID = userID, workspaceID
	writer.threadID, writer.agentID, writer.body = threadID, agentID, body
	return collaboration.Message{
		ID: "message_2", ThreadID: threadID, ActorID: agentID,
		ActorType: "agent", Body: body,
	}, nil
}

func TestMCPSendsMessageAsConnectedAgent(t *testing.T) {
	service := New(testAuthorizer{}, nil)
	messages := &testAgentMessageWriter{}
	handler := NewHandler(service)
	handler.SetAgentMessageWriter(messages)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	body := []byte(`{
		"jsonrpc":"2.0","id":4,"method":"tools/call",
		"params":{"name":"send_message","arguments":{"body":"Done through MCP"}}
	}`)
	request := httptest.NewRequest(
		http.MethodPost,
		"/workspaces/ws_1/mcp?thread_id=thread_1&agent_id=local_agent_codex",
		bytes.NewReader(body),
	)
	request = request.WithContext(auth.WithUserID(request.Context(), "nahid"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || messages.agentID != "local_agent_codex" ||
		messages.userID != "nahid" || messages.body != "Done through MCP" {
		t.Fatalf("status=%d writer=%#v body=%s", response.Code, messages, response.Body.String())
	}
}

func TestMCPValidatesAndSharesInteractiveArtifact(t *testing.T) {
	service := New(testAuthorizer{}, nil)
	messages := &testAgentMessageWriter{}
	handler := NewHandler(service)
	handler.SetAgentMessageWriter(messages)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	body := []byte(`{
		"jsonrpc":"2.0","id":5,"method":"tools/call",
		"params":{"name":"ui.create_artifact","arguments":{"document":{
			"version":"runspace.ui/v1","title":"Release readiness",
			"layout":{"type":"MetricGroup","props":{"items":[
				{"label":"Passing","value":"92%"}
			]}}
		}}}
	}`)
	request := httptest.NewRequest(
		http.MethodPost,
		"/workspaces/ws_1/mcp?thread_id=thread_1&agent_id=local_agent_codex",
		bytes.NewReader(body),
	)
	request = request.WithContext(auth.WithUserID(request.Context(), "nahid"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.HasPrefix(messages.body, "[[ui:artifact:") {
		t.Fatalf("status=%d writer=%#v body=%s", response.Code, messages, response.Body.String())
	}
	nodes, err := service.ListNodes(context.Background(), "nahid", "ws_1", Query{
		Kind: KindArtifact, ThreadID: "thread_1",
	})
	if err != nil || len(nodes) != 1 || nodes[0].Type != "ui_artifact" {
		t.Fatalf("interactive artifact was not persisted: %#v %v", nodes, err)
	}
}

type testDiscussionReader struct {
	messages []collaboration.Message
}

func (reader testDiscussionReader) ListMessages(
	_ context.Context, _, _, _ string,
) ([]collaboration.Message, error) {
	return reader.messages, nil
}

func TestMCPReadsCurrentDiscussionMessages(t *testing.T) {
	service := New(testAuthorizer{}, nil)
	discussions := testDiscussionReader{messages: []collaboration.Message{{
		ID: "message_1", ThreadID: "thread_1", ActorID: "nahid",
		ActorType: "user", Body: "Ship the MCP bridge",
	}}}
	router := chi.NewRouter()
	NewHandler(service, discussions).RegisterRoutes(router)
	body := []byte(`{
		"jsonrpc":"2.0","id":3,"method":"tools/call",
		"params":{"name":"read_discussion","arguments":{}}
	}`)
	request := httptest.NewRequest(
		http.MethodPost, "/workspaces/ws_1/mcp?thread_id=thread_1", bytes.NewReader(body),
	)
	request = request.WithContext(auth.WithUserID(request.Context(), "nahid"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "Ship the MCP bridge") ||
		!strings.Contains(response.Body.String(), "thread_1") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMCPThreadScopeDefaultsCreatedWork(t *testing.T) {
	service := New(testAuthorizer{}, nil)
	router := chi.NewRouter()
	NewHandler(service).RegisterRoutes(router)
	body := []byte(`{
		"jsonrpc":"2.0","id":2,"method":"tools/call",
		"params":{"name":"create_task","arguments":{"title":"Scoped work"}}
	}`)
	request := httptest.NewRequest(
		http.MethodPost, "/workspaces/ws_1/mcp?thread_id=thread_1", bytes.NewReader(body),
	)
	request = request.WithContext(auth.WithUserID(request.Context(), "nahid"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	nodes, err := service.ListNodes(context.Background(), "nahid", "ws_1", Query{
		Kind: KindTask, ThreadID: "thread_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Title != "Scoped work" {
		t.Fatalf("thread-scoped task missing: %#v", nodes)
	}
}
