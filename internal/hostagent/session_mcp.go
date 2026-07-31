package hostagent

import (
	"net/url"
	"os"
	"path"
	"strings"

	acpruntime "github.com/runspace/runspace/internal/runtime"
)

func runspaceMCPServers(
	binding LocalResourceBinding, userID, threadID, agentID string,
) []acpruntime.MCPServer {
	endpoint := runspaceMCPEndpoint(binding, threadID, agentID)
	userID = strings.TrimSpace(userID)
	if endpoint == "" || userID == "" {
		return []acpruntime.MCPServer{}
	}
	executable, err := os.Executable()
	if err != nil || strings.TrimSpace(executable) == "" {
		return []acpruntime.MCPServer{}
	}
	return []acpruntime.MCPServer{{
		Name:    "Runspace",
		Command: executable,
		Args: []string{
			"mcp-proxy", "--url", endpoint, "--user-id", userID,
		},
		Env: []acpruntime.EnvVariable{},
	}}
}

func runspaceMCPEndpoint(binding LocalResourceBinding, threadID, agentID string) string {
	gatewayURL := strings.TrimSpace(binding.GatewayURL)
	workspaceID := strings.TrimSpace(binding.WorkspaceID)
	parsed, err := url.Parse(gatewayURL)
	if err != nil || workspaceID == "" || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	parsed.Path = path.Join(
		strings.TrimSuffix(parsed.Path, "/"), "workspaces", workspaceID, "mcp",
	)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	query := parsed.Query()
	if threadID = strings.TrimSpace(threadID); threadID != "" {
		query.Set("thread_id", threadID)
	}
	if agentID = strings.TrimSpace(agentID); agentID != "" {
		query.Set("agent_id", agentID)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
