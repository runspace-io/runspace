package resourceplugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/runspace/runspace/internal/resourcegraph"
)

const availabilityTTL = 15 * time.Second

type Service struct {
	mu           sync.RWMutex
	connections  map[string]Connection
	availability map[string]Availability
	authorizer   Authorizer
	graph        Graph
	store        Store
	key          []byte
	now          func() time.Time
	providers    providerClients
}

func New(authorizer Authorizer, graph Graph, key []byte, now func() time.Time) (*Service, error) {
	if authorizer == nil || graph == nil || len(key) != 32 {
		return nil, ErrInvalid
	}
	if now == nil {
		now = time.Now
	}
	return &Service{
		connections: make(map[string]Connection), availability: make(map[string]Availability),
		authorizer: authorizer, graph: graph, key: append([]byte(nil), key...), now: now,
		providers: defaultProviderClients(),
	}, nil
}

func (s *Service) SetStore(store Store) { s.store = store }

func (s *Service) Connect(
	ctx context.Context, userID, workspaceID string, input ConnectRequest,
) (Connection, error) {
	if err := s.authorizer.CanWrite(ctx, workspaceID, userID); err != nil {
		return Connection{}, err
	}
	item, ok := manifest(strings.TrimSpace(input.PluginID))
	title, credential := strings.TrimSpace(input.Title), strings.TrimSpace(input.Credential)
	if !ok || title == "" || len(title) > 120 || credential == "" ||
		!validChoice(input.Placement, item.Placements) ||
		!validAuth(input.AuthMethod, item.AuthMethods) ||
		!validChoice(input.AccessMode, []string{"read", "manage", "full"}) {
		return Connection{}, ErrInvalid
	}
	if input.Placement != "runspace" {
		return Connection{}, errors.New("self-hosted connector registration is not available yet")
	}
	sealed, err := sealCredential(s.key, credential)
	if err != nil {
		return Connection{}, err
	}
	now := s.now().UTC()
	connection := Connection{
		ID: connectionID(workspaceID, userID, item.ID, title, now), WorkspaceID: workspaceID,
		PluginID: item.ID, Title: title, Placement: input.Placement,
		AuthMethod: input.AuthMethod, AccessMode: input.AccessMode, OwnerID: userID,
		Config: safeConfig(input.Config), Secret: sealed,
		Capabilities: append([]Capability(nil), item.Capabilities...),
		CreatedAt:    now, UpdatedAt: now,
	}
	if s.store != nil {
		if err := s.store.SaveResourceConnection(ctx, connection); err != nil {
			return Connection{}, err
		}
	}
	s.mu.Lock()
	s.connections[connection.ID] = connection
	s.mu.Unlock()
	_, err = s.graph.UpsertNode(ctx, userID, graphNode(connection, item))
	connection.Secret = nil
	return connection, err
}

func (s *Service) List(
	ctx context.Context, userID, workspaceID string,
) ([]Connection, error) {
	if err := s.authorizer.CanRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	items := make([]Connection, 0)
	if s.store != nil {
		stored, err := s.store.ListResourceConnections(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		items = stored
	} else {
		s.mu.RLock()
		for _, item := range s.connections {
			if item.WorkspaceID == workspaceID {
				items = append(items, item)
			}
		}
		s.mu.RUnlock()
	}
	for index := range items {
		items[index].Secret = nil
	}
	return items, nil
}

func graphNode(connection Connection, plugin Manifest) resourcegraph.Node {
	capabilities := make([]map[string]any, 0, len(plugin.Capabilities))
	for _, item := range plugin.Capabilities {
		capabilities = append(capabilities, map[string]any{
			"id": item.ID, "label": item.Label, "description": item.Description,
			"mode": item.Mode, "risk": item.Risk,
		})
	}
	return resourcegraph.Node{
		ID: "resource:" + connection.ID, WorkspaceID: connection.WorkspaceID,
		Kind: resourcegraph.KindResource, Type: plugin.ID, Title: connection.Title,
		Summary: plugin.Description, OwnerID: connection.OwnerID,
		Metadata: map[string]any{
			"entity_id": connection.ID, "plugin_id": plugin.ID,
			"resource_type": plugin.ResourceType, "placement": connection.Placement,
			"access_mode": connection.AccessMode, "capabilities": capabilities,
		},
	}
}

func (s *Service) connection(ctx context.Context, resourceID string) (Connection, error) {
	s.mu.RLock()
	item, ok := s.connections[resourceID]
	s.mu.RUnlock()
	if ok {
		return item, nil
	}
	if s.store == nil {
		return Connection{}, ErrNotFound
	}
	item, err := s.store.GetResourceConnection(ctx, resourceID)
	if err != nil {
		return Connection{}, ErrNotFound
	}
	return item, nil
}

func connectionID(workspaceID, userID, pluginID, title string, now time.Time) string {
	hash := sha256.Sum256([]byte(
		workspaceID + "\x00" + userID + "\x00" + pluginID + "\x00" + title + "\x00" + now.String(),
	))
	return "native_resource_" + hex.EncodeToString(hash[:12])
}

func validChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func validAuth(value string, methods []AuthMethod) bool {
	for _, method := range methods {
		if value == method.ID {
			return true
		}
	}
	return false
}

func safeConfig(input map[string]any) map[string]any {
	result := make(map[string]any)
	for key, value := range input {
		if text, ok := value.(string); ok && len(text) <= 240 {
			result[key] = strings.TrimSpace(text)
		}
	}
	return result
}
