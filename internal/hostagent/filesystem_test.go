package hostagent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectorySuggestionsCompleteFoldersOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "beta"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifact.txt"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	suggestions := directorySuggestions(filepath.Join(root, "al"), 12)
	if len(suggestions) != 1 || filepath.Base(suggestions[0]) != "alpha" {
		t.Fatalf("suggestions=%v", suggestions)
	}
}

func TestApprovedRepositoryFilesystemIsLazyAndBounded(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "src", "index.ts"),
		[]byte("export const ready = true;\n"),
		0o640,
	); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	server.mirrors[scopedResourceKey("admin", "repository-1")] = root
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	response, err := http.Get(httpServer.URL + "/v1/repositories/repository-1/tree?user_id=admin")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var tree struct {
		Entries []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(response.Body).Decode(&tree); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK ||
		len(tree.Entries) != 1 ||
		tree.Entries[0].Path != "src" {
		t.Fatalf("status=%d tree=%+v", response.StatusCode, tree)
	}

	fileResponse, err := http.Get(
		httpServer.URL + "/v1/repositories/repository-1/file?user_id=admin&path=src%2Findex.ts",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fileResponse.Body.Close() }()
	var file struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(fileResponse.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if fileResponse.StatusCode != http.StatusOK || file.Content != "export const ready = true;\n" {
		t.Fatalf("status=%d file=%+v", fileResponse.StatusCode, file)
	}

	traversal, err := http.Get(
		httpServer.URL + "/v1/repositories/repository-1/file?user_id=admin&path=..%2Fsecret",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = traversal.Body.Close() }()
	if traversal.StatusCode != http.StatusBadRequest {
		t.Fatalf("traversal status=%d", traversal.StatusCode)
	}
}
