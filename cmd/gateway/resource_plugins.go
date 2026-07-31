package main

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/persistence"
	"github.com/runspace/runspace/internal/resourcegraph"
	"github.com/runspace/runspace/internal/resourceplugin"
	"github.com/runspace/runspace/internal/workspace"
)

func configureResourcePlugins(
	api chi.Router,
	handler *resourcegraph.Handler,
	workspaces *workspace.MemoryService,
	graph *resourcegraph.Service,
	store *persistence.Store,
	hostAgentURL string,
) error {
	native, err := resourceplugin.New(workspaces, graph, channelSecretKey(), time.Now)
	if err != nil {
		return err
	}
	if store != nil {
		native.SetStore(store)
	}
	resourceplugin.NewHandler(native).RegisterRoutes(api)
	handler.SetCapabilityQuerier(resourcegraph.NewPlacementCapabilityQuerier(
		map[string]resourcegraph.CapabilityQuerier{
			"runspace": native,
			"host":     resourcegraph.NewHostCapabilityQuerier(hostAgentURL),
		},
	))
	return nil
}
