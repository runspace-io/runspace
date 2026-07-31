package git

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runspace/runspace/internal/contracts"
)

func TestGitHubRemoteOpenPRAndValidateAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing bearer authorization")
		}
		if request.Method == http.MethodGet {
			writer.WriteHeader(http.StatusOK)
			return
		}
		if request.URL.Path != "/repos/acme/project/pulls" {
			t.Errorf("path=%q", request.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"number": 42, "html_url": "https://github.com/acme/project/pull/42"})
	}))
	defer server.Close()

	remote := GitHubRemote{BaseURL: server.URL, Token: "secret", Client: server.Client()}
	if err := remote.ValidateAccess(context.Background(), "acme/project"); err != nil {
		t.Fatal(err)
	}
	result, err := remote.OpenPR(context.Background(), contracts.PullRequest{Repository: "acme/project", Title: "Ship", Head: "feat/ship", Base: "main"})
	if err != nil || result.Number != 42 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
