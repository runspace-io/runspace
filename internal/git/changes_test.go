package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProviderChangedFilesAndContents(t *testing.T) {
	repository := changeFixture(t)
	writeFixture(t, repository, "modified file.txt", "modified\n")
	if err := os.Remove(filepath.Join(repository, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repository, "mv", "rename source.txt", "renamed target.txt")
	writeFixture(t, repository, "untracked file.txt", "untracked\n")

	provider := NewProvider()
	changes, err := provider.ChangedFiles(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]Change, len(changes))
	for _, change := range changes {
		byPath[change.Path] = change
	}
	assertChange(t, byPath, "modified file.txt", "modified", "")
	assertChange(t, byPath, "deleted.txt", "deleted", "")
	assertChange(t, byPath, "renamed target.txt", "renamed", "rename source.txt")
	assertChange(t, byPath, "untracked file.txt", "untracked", "")

	assertContents(t, provider, repository, "modified file.txt", "base\n", "modified\n")
	assertContents(t, provider, repository, "deleted.txt", "delete\n", "")
	assertContents(t, provider, repository, "renamed target.txt", "rename\n", "rename\n")
	assertContents(t, provider, repository, "untracked file.txt", "", "untracked\n")
}

func TestProviderFileContentsRejectsUnsafeAndUnchangedPaths(t *testing.T) {
	repository := changeFixture(t)
	provider := NewProvider()
	for _, path := range []string{"", "../secret", "/absolute", "nested/../../secret"} {
		_, _, err := provider.FileContents(context.Background(), repository, path)
		if !errors.Is(err, ErrInvalidDiffPath) {
			t.Fatalf("path %q error = %v", path, err)
		}
	}
	_, _, err := provider.FileContents(context.Background(), repository, "modified file.txt")
	if !errors.Is(err, ErrChangeNotFound) {
		t.Fatalf("unchanged path error = %v", err)
	}
}

func TestParseChangesRejectsMalformedRename(t *testing.T) {
	if _, err := parseChanges("R  destination\x00"); err == nil {
		t.Fatal("expected malformed rename error")
	}
	if _, err := parseChanges("x"); err == nil {
		t.Fatal("expected short status record error")
	}
}

func changeFixture(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	gitRun(t, repository, "init", "--initial-branch=main")
	gitRun(t, repository, "config", "user.email", "test@example.com")
	gitRun(t, repository, "config", "user.name", "Test")
	writeFixture(t, repository, "modified file.txt", "base\n")
	writeFixture(t, repository, "deleted.txt", "delete\n")
	writeFixture(t, repository, "rename source.txt", "rename\n")
	gitRun(t, repository, "add", ".")
	gitRun(t, repository, "commit", "-m", "base")
	return repository
}

func writeFixture(t *testing.T, repository, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repository, path), []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
}

func assertChange(
	t *testing.T,
	changes map[string]Change,
	path string,
	status string,
	previousPath string,
) {
	t.Helper()
	change, ok := changes[path]
	if !ok {
		t.Fatalf("change %q missing: %+v", path, changes)
	}
	if change.Status != status || change.PreviousPath != previousPath {
		t.Fatalf("change %q = %+v", path, change)
	}
}

func assertContents(
	t *testing.T,
	provider Provider,
	repository string,
	path string,
	wantOriginal string,
	wantModified string,
) {
	t.Helper()
	original, modified, err := provider.FileContents(context.Background(), repository, path)
	if err != nil {
		t.Fatal(err)
	}
	if original != wantOriginal || modified != wantModified {
		t.Fatalf("%s contents = %q -> %q", path, original, modified)
	}
}
