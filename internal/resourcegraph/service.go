package resourcegraph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrUnauthorized = errors.New("resource graph authentication required")
	ErrForbidden    = errors.New("resource graph access forbidden")
	ErrInvalid      = errors.New("invalid resource graph input")
	ErrNotFound     = errors.New("resource graph node not found")
)

type Authorizer interface {
	CanRead(context.Context, string, string) error
	CanWrite(context.Context, string, string) error
}

type Store interface {
	UpsertGraphNode(context.Context, Node) error
	CreateGraphEdge(context.Context, Edge) error
	ListGraphNodes(context.Context, string, Query) ([]Node, error)
	GetGraphNode(context.Context, string, string) (Node, error)
	ListGraphEdges(context.Context, string, string) ([]Edge, error)
}

type Service struct {
	mu         sync.RWMutex
	nodes      map[string]Node
	edges      map[string]Edge
	authorizer Authorizer
	store      Store
	now        func() time.Time
	sequence   atomic.Uint64
}

func New(authorizer Authorizer, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		nodes: make(map[string]Node), edges: make(map[string]Edge),
		authorizer: authorizer, now: now,
	}
}

func (s *Service) SetStore(store Store) { s.store = store }

func (s *Service) UpsertNode(ctx context.Context, callerID string, node Node) (Node, error) {
	if err := s.authorize(ctx, callerID, node.WorkspaceID, true); err != nil {
		return Node{}, err
	}
	return s.upsert(ctx, callerID, node)
}

func (s *Service) upsert(ctx context.Context, callerID string, node Node) (Node, error) {
	normalizeNode(&node)
	if !validNode(node) {
		return Node{}, ErrInvalid
	}
	now := s.now().UTC()
	s.mu.Lock()
	existing, exists := s.nodes[node.ID]
	if exists && existing.WorkspaceID != node.WorkspaceID {
		s.mu.Unlock()
		return Node{}, ErrInvalid
	}
	if exists {
		node.CreatedAt = existing.CreatedAt
	} else if node.CreatedAt.IsZero() {
		node.CreatedAt = now
	}
	if node.OwnerID == "" {
		node.OwnerID = callerID
	}
	node.UpdatedAt = now
	node.Metadata = cloneMetadata(node.Metadata)
	s.nodes[node.ID] = node
	s.mu.Unlock()
	if s.store != nil {
		if err := s.store.UpsertGraphNode(ctx, node); err != nil {
			return Node{}, err
		}
	}
	return node, nil
}

func (s *Service) CreateEdge(ctx context.Context, callerID string, edge Edge) (Edge, error) {
	if err := s.authorize(ctx, callerID, edge.WorkspaceID, true); err != nil {
		return Edge{}, err
	}
	return s.createEdge(ctx, callerID, edge)
}

func (s *Service) createEdge(ctx context.Context, callerID string, edge Edge) (Edge, error) {
	edge.WorkspaceID = strings.TrimSpace(edge.WorkspaceID)
	edge.FromID = strings.TrimSpace(edge.FromID)
	edge.ToID = strings.TrimSpace(edge.ToID)
	edge.Relation = strings.TrimSpace(edge.Relation)
	if edge.WorkspaceID == "" || edge.FromID == "" || edge.ToID == "" ||
		edge.Relation == "" || edge.FromID == edge.ToID {
		return Edge{}, ErrInvalid
	}
	if edge.ID == "" {
		edge.ID = s.edgeID(edge)
	}
	if edge.CreatedBy == "" {
		edge.CreatedBy = callerID
	}
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt = s.now().UTC()
	}
	edge.Metadata = cloneMetadata(edge.Metadata)
	s.mu.Lock()
	s.edges[edge.ID] = edge
	s.mu.Unlock()
	if s.store != nil {
		if err := s.store.CreateGraphEdge(ctx, edge); err != nil {
			return Edge{}, err
		}
	}
	return edge, nil
}

func (s *Service) ListNodes(
	ctx context.Context, callerID, workspaceID string, query Query,
) ([]Node, error) {
	if err := s.authorize(ctx, callerID, workspaceID, false); err != nil {
		return nil, err
	}
	query.Limit = normalizedLimit(query.Limit)
	if s.store != nil {
		return s.store.ListGraphNodes(ctx, workspaceID, query)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Node, 0)
	for _, node := range s.nodes {
		if matchesNode(node, workspaceID, query) {
			items = append(items, node)
			if len(items) == query.Limit {
				break
			}
		}
	}
	return items, nil
}

func (s *Service) GetContext(
	ctx context.Context, callerID, workspaceID, nodeID string,
) (Context, error) {
	if err := s.authorize(ctx, callerID, workspaceID, false); err != nil {
		return Context{}, err
	}
	node, err := s.getNode(ctx, workspaceID, nodeID)
	if err != nil {
		return Context{}, err
	}
	edges, err := s.listEdges(ctx, workspaceID, nodeID)
	if err != nil {
		return Context{}, err
	}
	result := Context{Node: node, Incoming: []Edge{}, Outgoing: []Edge{}}
	for _, edge := range edges {
		if edge.FromID == nodeID {
			result.Outgoing = append(result.Outgoing, edge)
		} else {
			result.Incoming = append(result.Incoming, edge)
		}
	}
	return result, nil
}

func (s *Service) authorize(
	ctx context.Context, callerID, workspaceID string, write bool,
) error {
	if strings.TrimSpace(callerID) == "" {
		return ErrUnauthorized
	}
	if strings.TrimSpace(workspaceID) == "" || s.authorizer == nil {
		return ErrInvalid
	}
	var err error
	if write {
		err = s.authorizer.CanWrite(ctx, workspaceID, callerID)
	} else {
		err = s.authorizer.CanRead(ctx, workspaceID, callerID)
	}
	if err != nil {
		return errors.Join(ErrForbidden, err)
	}
	return nil
}

func (s *Service) getNode(ctx context.Context, workspaceID, nodeID string) (Node, error) {
	if s.store != nil {
		return s.store.GetGraphNode(ctx, workspaceID, nodeID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.nodes[nodeID]
	if !ok || node.WorkspaceID != workspaceID {
		return Node{}, ErrNotFound
	}
	return node, nil
}

func (s *Service) listEdges(ctx context.Context, workspaceID, nodeID string) ([]Edge, error) {
	if s.store != nil {
		return s.store.ListGraphEdges(ctx, workspaceID, nodeID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Edge, 0)
	for _, edge := range s.edges {
		if edge.WorkspaceID == workspaceID && (edge.FromID == nodeID || edge.ToID == nodeID) {
			result = append(result, edge)
		}
	}
	return result, nil
}

func (s *Service) edgeID(edge Edge) string {
	sequence := s.sequence.Add(1)
	return fmt.Sprintf("edge:%s:%d", edge.Relation, sequence)
}

func normalizeNode(node *Node) {
	node.ID = strings.TrimSpace(node.ID)
	node.WorkspaceID = strings.TrimSpace(node.WorkspaceID)
	node.Type = strings.TrimSpace(node.Type)
	node.Title = strings.TrimSpace(node.Title)
	node.Summary = strings.TrimSpace(node.Summary)
	node.ExternalRef = strings.TrimSpace(node.ExternalRef)
	node.OwnerID = strings.TrimSpace(node.OwnerID)
}

func validNode(node Node) bool {
	return node.ID != "" && node.WorkspaceID != "" && validKind(node.Kind) &&
		node.Type != "" && node.Title != ""
}

func validKind(kind Kind) bool {
	switch kind {
	case KindResource, KindTask, KindArtifact, KindAction, KindDiscussion,
		KindIdentity, KindPolicy, KindEvent:
		return true
	default:
		return false
	}
}

func matchesNode(node Node, workspaceID string, query Query) bool {
	if node.WorkspaceID != workspaceID || query.Kind != "" && node.Kind != query.Kind ||
		query.Type != "" && node.Type != query.Type {
		return false
	}
	if query.ThreadID != "" && metadataString(node.Metadata, "thread_id") != query.ThreadID {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(query.Text))
	return text == "" || strings.Contains(strings.ToLower(node.Title+" "+node.Summary), text)
}

func normalizedLimit(limit int) int {
	if limit < 1 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return value
}

func cloneMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
