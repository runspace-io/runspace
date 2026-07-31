package hostagent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runspace/runspace/internal/workspace"
)

func TestLocalResourcesAreUserScoped(t *testing.T) {
	server, err := NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	server.configFile = t.TempDir() + "/runspace-local.json"
	binding := LocalResourceBinding{
		Path: t.TempDir(), WorkspaceID: "ws_1",
		Resource: workspace.Resource{ID: "resource-1"},
	}
	if err := server.approveMirror("nahid", binding); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	response, err := http.Get(httpServer.URL + "/v1/resources/resource-1/tree?user_id=admin")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("another user accessed local resource: %d", response.StatusCode)
	}
}
