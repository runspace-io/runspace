package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runspace/runspace/internal/resourcegraph"
)

func (s *Store) UpsertGraphNode(ctx context.Context, node resourcegraph.Node) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO resource_graph_nodes
		(id,workspace_id,kind,type,title,summary,external_ref,owner_id,metadata,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET kind=EXCLUDED.kind,type=EXCLUDED.type,
		title=EXCLUDED.title,summary=EXCLUDED.summary,external_ref=EXCLUDED.external_ref,
		metadata=EXCLUDED.metadata,updated_at=EXCLUDED.updated_at
		WHERE resource_graph_nodes.workspace_id=EXCLUDED.workspace_id`,
		node.ID, node.WorkspaceID, node.Kind, node.Type, node.Title, node.Summary,
		node.ExternalRef, node.OwnerID, configJSON(node.Metadata), node.CreatedAt, node.UpdatedAt,
	)
	return err
}

func (s *Store) CreateGraphEdge(ctx context.Context, edge resourcegraph.Edge) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO resource_graph_edges
		(id,workspace_id,from_id,to_id,relation,created_by,metadata,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET relation=EXCLUDED.relation,metadata=EXCLUDED.metadata`,
		edge.ID, edge.WorkspaceID, edge.FromID, edge.ToID, edge.Relation,
		edge.CreatedBy, configJSON(edge.Metadata), edge.CreatedAt,
	)
	return err
}

func (s *Store) ListGraphNodes(
	ctx context.Context, workspaceID string, query resourcegraph.Query,
) ([]resourcegraph.Node, error) {
	statement := `SELECT id,workspace_id,kind,type,title,summary,external_ref,owner_id,
		metadata,created_at,updated_at FROM resource_graph_nodes WHERE workspace_id=$1`
	args := []any{workspaceID}
	if query.Kind != "" {
		args = append(args, query.Kind)
		statement += fmt.Sprintf(" AND kind=$%d", len(args))
	}
	if query.Type != "" {
		args = append(args, query.Type)
		statement += fmt.Sprintf(" AND type=$%d", len(args))
	}
	if query.ThreadID != "" {
		args = append(args, query.ThreadID)
		statement += fmt.Sprintf(" AND metadata->>'thread_id'=$%d", len(args))
	}
	if query.Text != "" {
		args = append(args, "%"+query.Text+"%")
		statement += fmt.Sprintf(" AND (title ILIKE $%d OR summary ILIKE $%d)", len(args), len(args))
	}
	args = append(args, query.Limit)
	statement += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d", len(args))
	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]resourcegraph.Node, 0)
	for rows.Next() {
		node, scanErr := scanGraphNode(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, node)
	}
	return items, rows.Err()
}

func (s *Store) GetGraphNode(
	ctx context.Context, workspaceID, nodeID string,
) (resourcegraph.Node, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,workspace_id,kind,type,title,summary,
		external_ref,owner_id,metadata,created_at,updated_at
		FROM resource_graph_nodes WHERE workspace_id=$1 AND id=$2`, workspaceID, nodeID)
	node, err := scanGraphNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return resourcegraph.Node{}, resourcegraph.ErrNotFound
	}
	return node, err
}

func (s *Store) ListGraphEdges(
	ctx context.Context, workspaceID, nodeID string,
) ([]resourcegraph.Edge, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,workspace_id,from_id,to_id,relation,
		created_by,metadata,created_at FROM resource_graph_edges
		WHERE workspace_id=$1 AND (from_id=$2 OR to_id=$2) ORDER BY created_at`, workspaceID, nodeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]resourcegraph.Edge, 0)
	for rows.Next() {
		var edge resourcegraph.Edge
		var metadata []byte
		if err := rows.Scan(
			&edge.ID, &edge.WorkspaceID, &edge.FromID, &edge.ToID, &edge.Relation,
			&edge.CreatedBy, &metadata, &edge.CreatedAt,
		); err != nil {
			return nil, err
		}
		edge.Metadata = parseConfig(metadata)
		items = append(items, edge)
	}
	return items, rows.Err()
}

type graphNodeScanner interface {
	Scan(...any) error
}

func scanGraphNode(scanner graphNodeScanner) (resourcegraph.Node, error) {
	var node resourcegraph.Node
	var metadata []byte
	err := scanner.Scan(
		&node.ID, &node.WorkspaceID, &node.Kind, &node.Type, &node.Title, &node.Summary,
		&node.ExternalRef, &node.OwnerID, &metadata, &node.CreatedAt, &node.UpdatedAt,
	)
	node.Metadata = parseConfig(metadata)
	return node, err
}
