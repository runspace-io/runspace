package hostagent

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runspace/runspace/internal/workspace"
)

func TestRunspaceMCPEndpointCarriesWorkspaceAndThreadScope(t *testing.T) {
	binding := LocalResourceBinding{
		GatewayURL:  "http://localhost:3000/gateway/",
		WorkspaceID: "ws 1",
		Resource:    workspace.Resource{ID: "resource-1"},
	}
	endpoint := runspaceMCPEndpoint(binding, "thread/1", "agent/1")
	expected := "http://localhost:3000/gateway/workspaces/ws%201/mcp?agent_id=agent%2F1&thread_id=thread%2F1"
	if endpoint != expected {
		t.Fatalf("endpoint=%q want=%q", endpoint, expected)
	}
}

func TestMCPProxyInjectsUserAndRelaysRequests(t *testing.T) {
	var receivedUser string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedUser = request.Header.Get("X-User-ID")
		body, _ := io.ReadAll(request.Body)
		if bytes.Contains(body, []byte(`"id":1`)) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
			return
		}
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	input := strings.NewReader(
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n" +
			"{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n",
	)
	var output bytes.Buffer
	if err := RunMCPProxy(
		context.Background(), input, &output, server.URL, "nahid", server.Client(),
	); err != nil {
		t.Fatal(err)
	}
	if receivedUser != "nahid" {
		t.Fatalf("user header=%q", receivedUser)
	}
	if strings.TrimSpace(output.String()) != `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}` {
		t.Fatalf("proxy output=%q", output.String())
	}
}
