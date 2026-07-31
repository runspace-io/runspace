package hostagent

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

type AgentInstallation struct {
	ID             string   `json:"id"`
	RegistryID     string   `json:"registry_id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Protocol       string   `json:"protocol"`
	Placement      string   `json:"placement"`
	Status         string   `json:"status"`
	Capabilities   []string `json:"capabilities"`
	Model          string   `json:"model,omitempty"`
	PermissionMode string   `json:"permission_mode,omitempty"`
}

type agentLaunch struct {
	registryID string
	command    string
	args       []string
}

type agentCandidate struct {
	registryID  string
	name        string
	description string
	launches    []agentLaunch
	baseCLIs    []string
}

var localAgentCandidates = []agentCandidate{
	{
		registryID: "opencode", name: "OpenCode",
		description: "Native ACP agent detected on this PC",
		launches:    []agentLaunch{{command: "opencode", args: []string{"acp"}}},
	},
	{
		registryID: "gemini", name: "Gemini CLI",
		description: "Native ACP agent detected on this PC",
		launches:    []agentLaunch{{command: "gemini", args: []string{"--acp"}}},
	},
	{
		registryID: "codex-acp", name: "Codex",
		description: "OpenAI Codex through the official ACP adapter",
		launches: []agentLaunch{
			{command: "codex-acp"},
			{command: "npx", args: []string{"-y", "@agentclientprotocol/codex-acp"}},
		},
		baseCLIs: []string{"codex"},
	},
	{
		registryID: "claude-acp", name: "Claude Agent",
		description: "Claude through the official ACP adapter",
		launches: []agentLaunch{
			{command: "claude-agent-acp"},
			{command: "claude-code-acp"},
			{command: "claude-acp"},
			{command: "npx", args: []string{"-y", "@agentclientprotocol/claude-agent-acp"}},
		},
		baseCLIs: []string{"claude"},
	},
}

func (s *Server) discoverAgents(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	agents := s.DiscoverAgents()
	s.mu.RLock()
	preferences := s.config.Users[userID]
	for index := range agents {
		if preferences != nil {
			preference := preferences.Agents[agents[index].ID]
			agents[index].Model = preference.Model
			agents[index].PermissionMode = preference.PermissionMode
		}
		if agents[index].PermissionMode == "" {
			agents[index].PermissionMode = "default"
		}
	}
	s.mu.RUnlock()
	writeJSON(writer, http.StatusOK, map[string]any{"agents": agents})
}

func (s *Server) DiscoverAgents() []AgentInstallation {
	discovered := make([]AgentInstallation, 0, len(localAgentCandidates))
	launches := make(map[string]agentLaunch)
	for _, candidate := range localAgentCandidates {
		installation, launch, found := s.discoverAgent(candidate)
		if !found {
			continue
		}
		discovered = append(discovered, installation)
		if installation.Status == "ready" {
			launches[installation.ID] = launch
		}
	}
	s.agentMu.Lock()
	s.agentLaunch = launches
	s.agentMu.Unlock()
	return discovered
}

func (s *Server) discoverAgent(candidate agentCandidate) (AgentInstallation, agentLaunch, bool) {
	id := localAgentID(s.deviceID, candidate.registryID)
	base := AgentInstallation{
		ID: id, RegistryID: candidate.registryID, Name: candidate.name,
		Description: candidate.description, Protocol: "acp", Placement: "host",
		Capabilities: []string{"sessions", "files", "terminal"},
	}
	for _, launch := range candidate.launches {
		path, err := s.lookPath(launch.command)
		if err == nil && strings.TrimSpace(path) != "" {
			base.Status = "ready"
			return base, agentLaunch{
				registryID: candidate.registryID,
				command:    path, args: append([]string(nil), launch.args...),
			}, true
		}
	}
	for _, command := range candidate.baseCLIs {
		if path, err := s.lookPath(command); err == nil && strings.TrimSpace(path) != "" {
			base.Status = "adapter_required"
			return base, agentLaunch{}, true
		}
	}
	return AgentInstallation{}, agentLaunch{}, false
}

func localAgentID(deviceID, registryID string) string {
	sum := sha256.Sum256([]byte(deviceID + "\x00" + registryID))
	return "local_agent_" + hex.EncodeToString(sum[:12])
}
