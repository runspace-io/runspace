package hostagent

import (
	"context"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) agentModels(writer http.ResponseWriter, request *http.Request) {
	if localUserID(request) == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	s.agentMu.RLock()
	launch, found := s.agentLaunch[chi.URLParam(request, "agentID")]
	s.agentMu.RUnlock()
	if !found {
		s.DiscoverAgents()
		s.agentMu.RLock()
		launch, found = s.agentLaunch[chi.URLParam(request, "agentID")]
		s.agentMu.RUnlock()
	}
	if !found {
		writeError(writer, http.StatusNotFound, "agent is not available")
		return
	}
	models := []string{}
	switch launch.registryID {
	case "claude-acp":
		models = []string{"sonnet", "opus"}
	case "opencode":
		models = localOpenCodeModels(request.Context(), launch.command)
	}
	writeJSON(writer, http.StatusOK, map[string]any{"models": models})
}

func localOpenCodeModels(parent context.Context, command string) []string {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, command, "models").Output()
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	models := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "/") {
			models = append(models, line)
		}
		if len(models) == 200 {
			break
		}
	}
	return models
}
