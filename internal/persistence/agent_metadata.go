package persistence

import (
	"context"

	"github.com/runspace/runspace/internal/agentregistry"
)

func (s *Store) UpsertAgentMetadata(
	ctx context.Context, ownerID string, item agentregistry.Installation,
) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_agent_metadata
		(owner_id,id,registry_id,name,description,protocol,placement,status,capabilities,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (owner_id,id) DO UPDATE SET registry_id=EXCLUDED.registry_id,name=EXCLUDED.name,
		description=EXCLUDED.description,protocol=EXCLUDED.protocol,placement=EXCLUDED.placement,
		status=EXCLUDED.status,capabilities=EXCLUDED.capabilities,updated_at=EXCLUDED.updated_at`,
		ownerID, item.ID, item.RegistryID, item.Name, item.Description, item.Protocol, item.Placement,
		item.Status, configJSON(item.Capabilities), item.UpdatedAt)
	return err
}

func (s *Store) ListAgentMetadata(
	ctx context.Context, ownerID string,
) ([]agentregistry.Installation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,registry_id,name,description,protocol,placement,status,
		capabilities,updated_at FROM user_agent_metadata WHERE owner_id=$1 ORDER BY name,id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []agentregistry.Installation
	for rows.Next() {
		var item agentregistry.Installation
		var capabilities []byte
		if err := rows.Scan(&item.ID, &item.RegistryID, &item.Name, &item.Description, &item.Protocol,
			&item.Placement, &item.Status, &capabilities, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Capabilities = parseStringSlice(capabilities)
		item.OwnerID = ownerID
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListWorkspaceAgentMetadata(
	ctx context.Context, viewerID, workspaceID string,
) ([]agentregistry.Installation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.owner_id,a.id,a.registry_id,a.name,a.description,
		a.protocol,a.placement,a.status,a.capabilities,a.updated_at
		FROM user_agent_metadata a
		JOIN workspace_members owner ON owner.user_id=a.owner_id AND owner.workspace_id=$1
		JOIN workspace_members viewer ON viewer.workspace_id=owner.workspace_id AND viewer.user_id=$2
		ORDER BY a.name,a.id`, workspaceID, viewerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []agentregistry.Installation
	for rows.Next() {
		var item agentregistry.Installation
		var capabilities []byte
		if err := rows.Scan(&item.OwnerID, &item.ID, &item.RegistryID, &item.Name, &item.Description,
			&item.Protocol, &item.Placement, &item.Status, &capabilities, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Capabilities = parseStringSlice(capabilities)
		out = append(out, item)
	}
	return out, rows.Err()
}
