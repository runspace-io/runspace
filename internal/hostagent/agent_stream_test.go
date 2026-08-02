package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/runspace/runspace/internal/agentregistry"
	acpruntime "github.com/runspace/runspace/internal/runtime"
)

type chunkingACP struct {
	notices chan acpruntime.ACPNotification
	chunks  []string
}

func (fake *chunkingACP) Initialize(context.Context) error { return nil }
func (fake *chunkingACP) NewSession(context.Context, string) (string, error) {
	return "native-session", nil
}
func (fake *chunkingACP) ResumeSession(context.Context, string, string) error { return nil }
func (fake *chunkingACP) SetSessionModel(context.Context, string, string) error {
	return nil
}
func (fake *chunkingACP) Prompt(_ context.Context, sessionID, _ string) error {
	for _, chunk := range fake.chunks {
		fake.notices <- acpruntime.ACPNotification{
			SessionID: sessionID, Kind: "agent_message_chunk", Text: chunk,
		}
	}
	return nil
}
func (fake *chunkingACP) Cancel(context.Context, string) error { return nil }
func (fake *chunkingACP) AnswerPermission(context.Context, string, string) error {
	return nil
}
func (fake *chunkingACP) Notifications() <-chan acpruntime.ACPNotification {
	return fake.notices
}
func (fake *chunkingACP) Close() error { return nil }

type capturedPush struct {
	taskID string
	userID string
	update taskStreamUpdate
}

// gatewayRecorder decodes pushes exactly the way the real gateway handler does —
// into agentregistry.TaskStreamUpdate with unknown fields rejected — so a drift
// between the two struct definitions fails here instead of in production.
func gatewayRecorder(t *testing.T) (*httptest.Server, func() []capturedPush) {
	t.Helper()
	var mu sync.Mutex
	var pushes []capturedPush
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			var wire agentregistry.TaskStreamUpdate
			strict := json.NewDecoder(bytes.NewReader(body))
			strict.DisallowUnknownFields()
			if err := strict.Decode(&wire); err != nil {
				t.Errorf("gateway rejected the host agent push: %v (%s)", err, body)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			var update taskStreamUpdate
			_ = json.Unmarshal(body, &update)
			mu.Lock()
			pushes = append(pushes, capturedPush{
				taskID: request.URL.Path, userID: request.Header.Get("X-User-ID"), update: update,
			})
			mu.Unlock()
			writer.WriteHeader(http.StatusAccepted)
		}))
	t.Cleanup(server.Close)
	return server, func() []capturedPush {
		mu.Lock()
		defer mu.Unlock()
		return append([]capturedPush(nil), pushes...)
	}
}

func streamingServer(t *testing.T, gatewayURL string, chunks []string) *Server {
	t.Helper()
	server, err := NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	server.configFile = t.TempDir() + "/runspace-local.json"
	resourcePath := t.TempDir()
	binding := LocalResourceBinding{
		Path: resourcePath, GatewayURL: gatewayURL, WorkspaceID: "ws_1",
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
	server.newACPClient = func(
		context.Context, acpruntime.StdioOptions,
	) (acpruntime.ACPClient, error) {
		return &chunkingACP{
			notices: make(chan acpruntime.ACPNotification, len(chunks)), chunks: chunks,
		}, nil
	}
	return server
}

func promptOnce(t *testing.T, server *Server, prompt string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(agentPromptRequest{
		ResourceID: "resource-1", ThreadID: "thread-1", Prompt: prompt,
	})
	request := httptest.NewRequest(
		http.MethodPost, "/v1/agents/local_agent_test/prompt", bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "nahid")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

// Every streamed chunk must reach the gateway with its own message identity;
// that is what lets a grantee and a reloaded browser rebuild the transcript.
func TestPromptStreamsEachChunkToGateway(t *testing.T) {
	gateway, pushes := gatewayRecorder(t)
	server := streamingServer(t, gateway.URL, []string{"Reading main.go", "Found the bug"})
	if response := promptOnce(t, server, "investigate"); response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var bodies []string
	seen := map[string]bool{}
	for _, push := range pushes() {
		if push.userID != "nahid" || push.update.WorkspaceID != "ws_1" ||
			push.update.ThreadID != "thread-1" || push.update.AgentID != "local_agent_test" {
			t.Fatalf("push lost task identity: %#v", push)
		}
		for _, message := range push.update.Messages {
			if seen[message.ID] {
				t.Fatalf("duplicate message ID %q across pushes", message.ID)
			}
			seen[message.ID] = true
			bodies = append(bodies, message.Body)
		}
	}
	if len(bodies) != 3 || bodies[0] != "investigate" ||
		bodies[1] != "Reading main.go" || bodies[2] != "Found the bug" {
		t.Fatalf("streamed transcript incomplete or out of order: %#v", bodies)
	}
}

func TestPromptReportsTerminalStatusToGateway(t *testing.T) {
	gateway, pushes := gatewayRecorder(t)
	server := streamingServer(t, gateway.URL, []string{"done"})
	if response := promptOnce(t, server, "investigate"); response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	captured := pushes()
	if len(captured) == 0 {
		t.Fatal("no pushes reached the gateway")
	}
	if final := captured[len(captured)-1].update; final.Status != "completed" {
		t.Fatalf("final push status=%q", final.Status)
	}
}

// A device that cannot reach its gateway must still run its own agent.
func TestPromptSucceedsWhenGatewayIsUnreachable(t *testing.T) {
	server := streamingServer(t, "http://127.0.0.1:1", []string{"done"})
	response := promptOnce(t, server, "investigate")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("done")) {
		t.Fatalf("local output lost: %s", response.Body.String())
	}
}
