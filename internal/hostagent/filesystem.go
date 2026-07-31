package hostagent

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/sandbox"
)

type approvedMirrorResolver struct{ server *Server }

func (r approvedMirrorResolver) Root(_ context.Context, userID string, repositoryID string) (string, error) {
	if r.server == nil {
		return "", sandbox.ErrNotFound
	}
	r.server.mu.RLock()
	path := r.server.mirrors[scopedResourceKey(userID, repositoryID)]
	r.server.mu.RUnlock()
	if path == "" {
		return "", sandbox.ErrNotFound
	}
	return path, nil
}

func (s *Server) repositoryTree(writer http.ResponseWriter, request *http.Request) {
	if s.browser == nil {
		writeError(writer, http.StatusServiceUnavailable, "host filesystem is unavailable")
		return
	}
	entries, err := s.browser.Tree(
		request.Context(),
		localUserID(request),
		chi.URLParam(request, "repositoryID"),
		request.URL.Query().Get("path"),
	)
	if err != nil {
		writeFilesystemError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) repositoryFile(writer http.ResponseWriter, request *http.Request) {
	if s.browser == nil {
		writeError(writer, http.StatusServiceUnavailable, "host filesystem is unavailable")
		return
	}
	file, err := s.browser.Read(
		request.Context(),
		localUserID(request),
		chi.URLParam(request, "repositoryID"),
		request.URL.Query().Get("path"),
	)
	if err != nil {
		writeFilesystemError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, file)
}

func writeFilesystemError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, sandbox.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, sandbox.ErrInvalidPath),
		errors.Is(err, sandbox.ErrBinary),
		errors.Is(err, sandbox.ErrTooLarge),
		errors.Is(err, sandbox.ErrSymlink):
		status = http.StatusBadRequest
	}
	writeError(writer, status, err.Error())
}

func (s *Server) suggestDirectories(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeRequest(writer, request, &body); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"paths": directorySuggestions(body.Path, 12),
	})
}

func directorySuggestions(input string, limit int) []string {
	input = expandHome(strings.TrimSpace(input))
	if input == "" {
		home, _ := os.UserHomeDir()
		workingDirectory, _ := os.Getwd()
		return uniquePaths([]string{home, workingDirectory})
	}
	base, fragment := input, ""
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		base, fragment = filepath.Dir(input), filepath.Base(input)
	}
	if absolute, err := filepath.Abs(base); err == nil {
		base = absolute
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	paths := make([]string, 0, min(limit, len(entries)))
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(strings.ToLower(entry.Name()), strings.ToLower(fragment)) {
			continue
		}
		paths = append(paths, filepath.Join(base, entry.Name()))
		if len(paths) == limit {
			break
		}
	}
	sort.Strings(paths)
	return paths
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimLeft(path[1:], `/\`))
}

func uniquePaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := make(map[string]struct{})
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}
