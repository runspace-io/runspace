package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/runspace/runspace/internal/resourceplugin"
)

func (s *Store) SaveResourceConnection(
	ctx context.Context, connection resourceplugin.Connection,
) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO resource_connections
		(id,workspace_id,plugin_id,title,placement,auth_method,access_mode,owner_id,
		config,sealed_credential,capabilities,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title,access_mode=EXCLUDED.access_mode,
		config=EXCLUDED.config,sealed_credential=EXCLUDED.sealed_credential,
		capabilities=EXCLUDED.capabilities,updated_at=EXCLUDED.updated_at`,
		connection.ID, connection.WorkspaceID, connection.PluginID, connection.Title,
		connection.Placement, connection.AuthMethod, connection.AccessMode, connection.OwnerID,
		configJSON(connection.Config), connection.Secret, configJSON(connection.Capabilities),
		connection.CreatedAt, connection.UpdatedAt,
	)
	return err
}

func (s *Store) ListResourceConnections(
	ctx context.Context, workspaceID string,
) ([]resourceplugin.Connection, error) {
	rows, err := s.db.QueryContext(ctx, resourceConnectionSelect+
		` WHERE workspace_id=$1 ORDER BY updated_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]resourceplugin.Connection, 0)
	for rows.Next() {
		item, scanErr := scanResourceConnection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetResourceConnection(
	ctx context.Context, resourceID string,
) (resourceplugin.Connection, error) {
	item, err := scanResourceConnection(s.db.QueryRowContext(
		ctx, resourceConnectionSelect+` WHERE id=$1`, resourceID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return resourceplugin.Connection{}, resourceplugin.ErrNotFound
	}
	return item, err
}

const resourceConnectionSelect = `SELECT id,workspace_id,plugin_id,title,placement,
	auth_method,access_mode,owner_id,config,sealed_credential,capabilities,created_at,updated_at
	FROM resource_connections`

type resourceConnectionScanner interface{ Scan(...any) error }

func scanResourceConnection(scanner resourceConnectionScanner) (resourceplugin.Connection, error) {
	var item resourceplugin.Connection
	var config, capabilities []byte
	err := scanner.Scan(
		&item.ID, &item.WorkspaceID, &item.PluginID, &item.Title, &item.Placement,
		&item.AuthMethod, &item.AccessMode, &item.OwnerID, &config, &item.Secret,
		&capabilities, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return resourceplugin.Connection{}, err
	}
	item.Config = parseConfig(config)
	if json.Unmarshal(capabilities, &item.Capabilities) != nil {
		return resourceplugin.Connection{}, errors.New("invalid stored resource capabilities")
	}
	return item, nil
}
