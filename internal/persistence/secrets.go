package persistence

import (
	"context"
	"database/sql"
	"time"

	"github.com/runspace/runspace/internal/secrets"
)

func (s *Store) SetEncrypted(ctx context.Context, channelID, name string, sealed []byte, updated time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO channel_secrets (channel_id,name,sealed,updated_at) VALUES ($1,$2,$3,$4) ON CONFLICT (channel_id,name) DO UPDATE SET sealed=EXCLUDED.sealed,updated_at=EXCLUDED.updated_at`, channelID, name, sealed, updated)
	return err
}

func (s *Store) ListEncrypted(ctx context.Context, channelID string) ([]secrets.EncryptedMetadata, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name,updated_at FROM channel_secrets WHERE channel_id=$1 ORDER BY name`, channelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []secrets.EncryptedMetadata
	for rows.Next() {
		var item secrets.EncryptedMetadata
		if err := rows.Scan(&item.Name, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ResolveEncrypted(ctx context.Context, channelID, name string) ([]byte, error) {
	var sealed []byte
	err := s.db.QueryRowContext(ctx, `SELECT sealed FROM channel_secrets WHERE channel_id=$1 AND name=$2`, channelID, name).Scan(&sealed)
	return sealed, err
}

func (s *Store) DeleteEncrypted(ctx context.Context, channelID, name string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM channel_secrets WHERE channel_id=$1 AND name=$2`, channelID, name)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}
