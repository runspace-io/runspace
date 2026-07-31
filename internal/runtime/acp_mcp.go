package runtime

func (c *stdioACP) sessionParams(cwd string) map[string]any {
	return map[string]any{
		"cwd": cwd, "mcpServers": cloneMCPServers(c.mcpServers),
	}
}

func cloneMCPServers(servers []MCPServer) []MCPServer {
	result := make([]MCPServer, len(servers))
	for index, server := range servers {
		server.Args = append([]string(nil), server.Args...)
		server.Env = append([]EnvVariable(nil), server.Env...)
		result[index] = server
	}
	return result
}
