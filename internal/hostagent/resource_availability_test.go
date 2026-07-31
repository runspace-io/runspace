package hostagent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCapabilityAvailabilityIsUserScopedAndCached(t *testing.T) {
	server, err := NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	server.config.Users["nahid"] = &LocalUserConfig{
		Resources: make(map[string]LocalResourceBinding),
		Agents:    make(map[string]LocalAgentPreference),
		Sessions:  make(map[string]LocalACPSession),
		Capabilities: map[string]LocalCapabilityResource{
			"resource-1": {ID: "resource-1", AdapterID: "github-cli"},
		},
	}
	lookups := 0
	server.lookPath = func(string) (string, error) {
		lookups++
		return "/usr/bin/gh", nil
	}
	handler := server.Handler()
	for range 2 {
		request := httptest.NewRequest(
			http.MethodGet, "/v1/capability-resources/resource-1/availability", nil,
		)
		request.Header.Set("X-User-ID", "nahid")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("availability returned %d: %s", response.Code, response.Body.String())
		}
	}
	if lookups != 1 {
		t.Fatalf("expected one lazy executable check, got %d", lookups)
	}
	request := httptest.NewRequest(
		http.MethodGet, "/v1/capability-resources/resource-1/availability", nil,
	)
	request.Header.Set("X-User-ID", "another-user")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("another user received availability: %d", response.Code)
	}
}
