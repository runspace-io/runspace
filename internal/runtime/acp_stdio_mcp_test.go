package runtime

import "testing"

func TestACPSessionParamsIncludeConfiguredMCPServers(t *testing.T) {
	client := &stdioACP{mcpServers: []MCPServer{{
		Name: "Runspace", Command: "/runspace-host-agent",
		Args: []string{"mcp-proxy", "--url", "http://localhost/workspaces/ws_1/mcp"},
		Env:  []EnvVariable{},
	}}}
	params := client.sessionParams("/workspace")
	servers, ok := params["mcpServers"].([]MCPServer)
	if !ok || len(servers) != 1 {
		t.Fatalf("MCP servers missing from session params: %#v", params)
	}
	if servers[0].Name != "Runspace" || servers[0].Command == "" {
		t.Fatalf("unexpected MCP server: %#v", servers[0])
	}
	servers[0].Args[0] = "changed"
	if client.mcpServers[0].Args[0] != "mcp-proxy" {
		t.Fatal("session params aliased the configured MCP server")
	}
}
