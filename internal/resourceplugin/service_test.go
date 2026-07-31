package resourceplugin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/resourcegraph"
)

type allowAll struct{}

func (allowAll) CanRead(context.Context, string, string) error  { return nil }
func (allowAll) CanWrite(context.Context, string, string) error { return nil }

type graphCapture struct{ node resourcegraph.Node }

func (capture *graphCapture) UpsertNode(
	_ context.Context, _ string, node resourcegraph.Node,
) (resourcegraph.Node, error) {
	capture.node = node
	return node, nil
}

func TestNativeGitHubConnectionKeepsCredentialOutOfGraphAndQueriesAPI(t *testing.T) {
	requests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatal("missing provider authorization")
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/user" {
			_, _ = writer.Write([]byte(`{"login":"nahid"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"items":[{"full_name":"runspace/app","description":"Workspace","html_url":"https://github.com/runspace/app","visibility":"private"}]}`))
	}))
	defer provider.Close()
	graph := &graphCapture{}
	service, err := New(allowAll{}, graph, []byte("01234567890123456789012345678901"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	service.providers.githubURL = provider.URL
	connection, err := service.Connect(context.Background(), "nahid", "ws_1", ConnectRequest{
		PluginID: "github", Title: "Engineering GitHub", Placement: "runspace",
		AuthMethod: "token", AccessMode: "read", Credential: "secret-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(connection.Secret) != 0 || strings.Contains(stringify(graph.node.Metadata), "secret-token") {
		t.Fatal("credential escaped into public connection metadata")
	}
	for range 2 {
		status, statusErr := service.Availability(
			context.Background(), "nahid", connection.ID, "runspace",
		)
		if statusErr != nil || status.(Availability).Status != "available" {
			t.Fatalf("availability failed: %#v %v", status, statusErr)
		}
	}
	if requests != 1 {
		t.Fatalf("expected lazy cached health check, got %d provider requests", requests)
	}
	result, err := service.Query(
		context.Background(), "nahid", connection.ID, "runspace",
		resourcegraph.CapabilityQuery{Capability: "github.repositories.query", Query: "runspace"},
	)
	if err != nil {
		t.Fatal(err)
	}
	matches := result.(map[string]any)["matches"].([]resourcegraph.CapabilityMatch)
	if len(matches) != 1 || matches[0].Title != "runspace/app" {
		t.Fatalf("unexpected GitHub result: %#v", matches)
	}
}

func TestDigitalOceanNativeQueryReturnsSanitizedDroplets(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"droplets":[{"id":"42","name":"api-prod","status":"active","region":{"slug":"nyc3"},"networks":{"private":[{"ip_address":"10.0.0.4"}]}}]}`))
	}))
	defer provider.Close()
	service, _ := New(
		allowAll{}, &graphCapture{}, []byte("01234567890123456789012345678901"), time.Now,
	)
	service.providers.digitalURL = provider.URL
	connection, err := service.Connect(context.Background(), "nahid", "ws_1", ConnectRequest{
		PluginID: "digitalocean", Title: "Production", Placement: "runspace",
		AuthMethod: "token", AccessMode: "read", Credential: "dop-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Query(
		context.Background(), "nahid", connection.ID, "runspace",
		resourcegraph.CapabilityQuery{
			Capability: "digitalocean.droplets.query", Query: "api",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	match := result.(map[string]any)["matches"].([]resourcegraph.CapabilityMatch)[0]
	if match.Title != "api-prod" || strings.Contains(stringify(match), "10.0.0.4") {
		t.Fatalf("unsafe DigitalOcean result: %#v", match)
	}
}

func TestPostgreSQLPluginAdvertisesFixedSchemaCapability(t *testing.T) {
	plugin, ok := manifest("postgresql")
	if !ok || len(plugin.Capabilities) != 1 ||
		plugin.Capabilities[0].ID != "postgres.schema.query" {
		t.Fatalf("unexpected PostgreSQL plugin: %#v", plugin)
	}
}

func stringify(value any) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(
		fmt.Sprintf("%#v", value), "\n", " ",
	), "\t", " "))
}
