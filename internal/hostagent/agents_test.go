package hostagent

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDiscoverAgentsKeepsExecutablePathsPrivate(t *testing.T) {
	server, err := NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	server.deviceID = "device-1"
	server.lookPath = func(command string) (string, error) {
		switch command {
		case "opencode":
			return filepath.Join("private", "bin", "opencode"), nil
		case "codex":
			return filepath.Join("private", "bin", "codex"), nil
		default:
			return "", errors.New("not found")
		}
	}
	agents := server.DiscoverAgents()
	if len(agents) != 2 {
		t.Fatalf("agents = %#v", agents)
	}
	if agents[0].RegistryID != "opencode" || agents[0].Status != "ready" {
		t.Fatalf("OpenCode discovery = %#v", agents[0])
	}
	if agents[1].RegistryID != "codex-acp" || agents[1].Status != "adapter_required" {
		t.Fatalf("Codex discovery = %#v", agents[1])
	}
	if agents[0].ID == "" || agents[0].ID == agents[1].ID {
		t.Fatalf("installation IDs are not stable and unique: %#v", agents)
	}
	if launch := server.agentLaunch[agents[0].ID]; launch.command == "" {
		t.Fatal("ready agent launch was not retained locally")
	}
	if _, exposed := server.agentLaunch[agents[1].ID]; exposed {
		t.Fatal("agent without an ACP adapter must not be launchable")
	}
}
