package main

import (
	"os"
	"strings"
	"time"

	"github.com/runspace/runspace/internal/agentregistry"
	"github.com/runspace/runspace/internal/collaboration"
	"github.com/runspace/runspace/internal/events"
	"github.com/runspace/runspace/internal/persistence"
	"github.com/runspace/runspace/internal/resourcegraph"
	"github.com/runspace/runspace/internal/workspace"
)

// newAgentRegistry wires the agent registry and returns the Host Agent base URL
// alongside it, since resource plugins address the same host.
func newAgentRegistry(
	workspaces *workspace.MemoryService,
	chat *collaboration.MemoryService,
	graph *resourcegraph.Service,
	store *persistence.Store,
	publisher *events.NATSPublisher,
) (*agentregistry.Service, string) {
	registry := agentregistry.New(time.Now, workspaces)
	if store != nil {
		registry.SetStore(store)
		registry.SetTaskGrantStore(store)
		registry.SetTaskStore(store)
		registry.SetTaskMessageStore(store)
		registry.SetTaskQuestionStore(store)
	}
	if publisher != nil {
		registry.SetEventPublisher(publisher)
	}
	registry.SetMessageWriter(chat)
	registry.SetGraphProjector(graph)
	hostAgentURL := strings.TrimSpace(os.Getenv("HOST_AGENT_URL"))
	if hostAgentURL == "" {
		hostAgentURL = "http://host.docker.internal:7799"
	}
	executor := agentregistry.NewHostTaskExecutor(hostAgentURL)
	registry.SetTaskExecutor(executor)
	registry.SetQuestionAnswerer(executor)
	return registry, hostAgentURL
}
