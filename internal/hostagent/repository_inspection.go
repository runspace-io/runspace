package hostagent

import (
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
)

func (s *Server) inspectRepository(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeRequest(writer, request, &body); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	status, err := s.InspectRepository(request.Context(), body.Path)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, status)
}

func (s *Server) initializeRepository(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Path   string `json:"path"`
		Branch string `json:"branch"`
	}
	if err := decodeRequest(writer, request, &body); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	path, err := existingDirectory(body.Path)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	branch := fallback(strings.TrimSpace(body.Branch), "main")
	if output, err := exec.CommandContext(
		request.Context(), s.gitBinary, "-C", path, "init", "--initial-branch="+branch,
	).CombinedOutput(); err != nil {
		writeError(writer, http.StatusBadRequest, "initialize Git: "+strings.TrimSpace(string(output)))
		return
	}
	status, err := s.InspectRepository(request.Context(), path)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, status)
}

func (s *Server) InspectRepository(ctx context.Context, requestedPath string) (RepositoryStatus, error) {
	path, err := existingDirectory(requestedPath)
	if err != nil {
		return RepositoryStatus{}, err
	}
	topLevel, err := s.git(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil || !samePath(path, topLevel) {
		return RepositoryStatus{Path: path, CanConnect: true}, nil
	}
	origin, _ := s.git(ctx, path, "remote", "get-url", "origin")
	branch, _ := s.git(ctx, path, "branch", "--show-current")
	if branch == "" {
		branch = "main"
	}
	return RepositoryStatus{
		Path: path, Git: true, Origin: origin, Branch: branch,
		HasRemote: origin != "", CanConnect: true,
	}, nil
}

func samePath(left, right string) bool {
	left, leftErr := filepath.Abs(strings.TrimSpace(left))
	right, rightErr := filepath.Abs(strings.TrimSpace(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
