package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	acpruntime "github.com/runspace/runspace/internal/runtime"
)

type fakeLocalACP struct {
	notices chan acpruntime.ACPNotification
}

func (fake *fakeLocalACP) Initialize(context.Context) error { return nil }
func (fake *fakeLocalACP) NewSession(context.Context, string) (string, error) {
	return "private-native-session", nil
}
func (fake *fakeLocalACP) ResumeSession(context.Context, string, string) error {
	return errors.New("resume unsupported")
}
func (fake *fakeLocalACP) SetSessionModel(context.Context, string, string) error { return nil }
func (fake *fakeLocalACP) Prompt(_ context.Context, sessionID, _ string) error {
	fake.notices <- acpruntime.ACPNotification{
		SessionID: sessionID, Kind: "agent_message_chunk", Text: "HOST_ACP_OK",
	}
	return nil
}
func (fake *fakeLocalACP) Cancel(context.Context, string) error { return nil }
func (fake *fakeLocalACP) Notifications() <-chan acpruntime.ACPNotification {
	return fake.notices
}
func (fake *fakeLocalACP) Close() error { return nil }

func TestLocalAgentPromptUsesUserResourceAndHidesNativeSession(t *testing.T) {
	server, err := NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	server.configFile = t.TempDir() + "/runspace-local.json"
	server.mirrors[scopedResourceKey("nahid", "resource-1")] = t.TempDir()
	server.agentLaunch["local_agent_test"] = agentLaunch{
		registryID: "opencode", command: "private-command",
	}
	server.newACPClient = func(
		context.Context, acpruntime.StdioOptions,
	) (acpruntime.ACPClient, error) {
		return &fakeLocalACP{notices: make(chan acpruntime.ACPNotification, 2)}, nil
	}
	body, _ := json.Marshal(agentPromptRequest{
		ResourceID: "resource-1", ThreadID: "thread-1", Prompt: "hello",
	})
	request := httptest.NewRequest(
		http.MethodPost, "/v1/agents/local_agent_test/prompt", bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "nahid")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("private-native-session")) {
		t.Fatal("native ACP session ID leaked through the HTTP response")
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("HOST_ACP_OK")) {
		t.Fatalf("normalized output missing: %s", response.Body.String())
	}
	historyRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/agents/local_agent_test/session?resource_id=resource-1&thread_id=thread-1",
		nil,
	)
	historyRequest.Header.Set("X-User-ID", "nahid")
	historyResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(historyResponse, historyRequest)
	if historyResponse.Code != http.StatusOK {
		t.Fatalf("history status=%d body=%s", historyResponse.Code, historyResponse.Body.String())
	}
	var history localSessionView
	if err := json.Unmarshal(historyResponse.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if history.Status != "completed" || len(history.Messages) != 2 {
		t.Fatalf("private task history was not persisted: %#v", history)
	}
	if bytes.Contains(historyResponse.Body.Bytes(), []byte("private-native-session")) {
		t.Fatal("native session leaked through private task history")
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/v1/agent-chats", nil)
	listRequest.Header.Set("X-User-ID", "nahid")
	listResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("chat list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	if !bytes.Contains(listResponse.Body.Bytes(), []byte(`"title":"hello"`)) {
		t.Fatalf("human-readable chat title missing: %s", listResponse.Body.String())
	}
	for _, private := range [][]byte{
		[]byte("private-native-session"), []byte("HOST_ACP_OK"), []byte(`"messages"`),
	} {
		if bytes.Contains(listResponse.Body.Bytes(), private) {
			t.Fatalf("private chat content leaked through catalog: %s", listResponse.Body.String())
		}
	}
}

func TestLocalAgentSessionAutomaticallyInjectsRunspaceMCP(t *testing.T) {
	server, err := NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	server.configFile = t.TempDir() + "/runspace-local.json"
	resourcePath := t.TempDir()
	binding := LocalResourceBinding{
		Path: resourcePath, GatewayURL: "http://localhost:3000/gateway",
		WorkspaceID: "ws_1",
	}
	binding.Resource.ID = "resource-1"
	server.config.Users["nahid"] = &LocalUserConfig{
		Resources: map[string]LocalResourceBinding{"resource-1": binding},
		Agents:    map[string]LocalAgentPreference{},
		Sessions:  map[string]LocalACPSession{},
	}
	server.mirrors[scopedResourceKey("nahid", "resource-1")] = resourcePath
	server.agentLaunch["local_agent_test"] = agentLaunch{
		registryID: "opencode", command: "private-command",
	}
	var options acpruntime.StdioOptions
	server.newACPClient = func(
		_ context.Context, received acpruntime.StdioOptions,
	) (acpruntime.ACPClient, error) {
		options = received
		return &fakeLocalACP{notices: make(chan acpruntime.ACPNotification, 1)}, nil
	}
	if _, err := server.localAgentSession(
		"nahid", "local_agent_test", "resource-1", "thread-1", "",
	); err != nil {
		t.Fatal(err)
	}
	if len(options.MCPServers) != 1 {
		t.Fatalf("MCP servers=%#v", options.MCPServers)
	}
	serverConfig := options.MCPServers[0]
	if serverConfig.Name != "Runspace" || serverConfig.Command == "" {
		t.Fatalf("invalid Runspace MCP config: %#v", serverConfig)
	}
	joined := strings.Join(serverConfig.Args, " ")
	if !strings.Contains(joined, "mcp-proxy") ||
		!strings.Contains(joined, "workspaces/ws_1/mcp?") ||
		!strings.Contains(joined, "thread_id=thread-1") ||
		!strings.Contains(joined, "agent_id=local_agent_test") ||
		!strings.Contains(joined, "--user-id nahid") {
		t.Fatalf("MCP args=%q", joined)
	}
}
