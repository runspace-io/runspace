package persistence

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/runspace/runspace/internal/workspace"
)

// invitationSchema stores only the token's hash. A leaked backup then yields no
// usable invitation links, the same reason password resets are stored hashed.
const invitationSchema = `
CREATE TABLE IF NOT EXISTS workspace_invitations (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, token_hash text NOT NULL UNIQUE, role text NOT NULL, created_by text NOT NULL, expires_at timestamptz NOT NULL, accepted_by text NOT NULL DEFAULT '', accepted_at timestamptz, created_at timestamptz NOT NULL);
CREATE INDEX IF NOT EXISTS workspace_invitations_workspace ON workspace_invitations(workspace_id,created_at DESC);`

func (s *Store) CreateInvitation(
	ctx context.Context, invitation workspace.Invitation, tokenHash string,
) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspace_invitations
		(id,workspace_id,token_hash,role,created_by,expires_at,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		invitation.ID, invitation.WorkspaceID, tokenHash, invitation.Role,
		invitation.CreatedBy, invitation.ExpiresAt, invitation.CreatedAt,
	)
	return alreadyExists(err)
}

func (s *Store) GetInvitationByTokenHash(
	ctx context.Context, tokenHash string,
) (workspace.Invitation, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,workspace_id,role,created_by,expires_at,
		accepted_by,accepted_at,created_at FROM workspace_invitations WHERE token_hash=$1`,
		tokenHash)
	invitation, err := scanInvitation(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return workspace.Invitation{}, workspace.ErrNotFound
	}
	return invitation, err
}

// MarkInvitationAccepted claims a single-use link. The accepted_by guard makes
// the update itself the lock, so two people redeeming the same link at once
// cannot both succeed.
func (s *Store) MarkInvitationAccepted(
	ctx context.Context, id, userID string, at time.Time,
) error {
	result, err := s.db.ExecContext(ctx, `UPDATE workspace_invitations
		SET accepted_by=$2, accepted_at=$3 WHERE id=$1 AND accepted_at IS NULL`,
		id, userID, at)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return workspace.ErrNotFound
	}
	return nil
}

func (s *Store) ListInvitations(
	ctx context.Context, workspaceID string,
) ([]workspace.Invitation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,workspace_id,role,created_by,expires_at,
		accepted_by,accepted_at,created_at FROM workspace_invitations
		WHERE workspace_id=$1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	invitations := make([]workspace.Invitation, 0)
	for rows.Next() {
		invitation, err := scanInvitation(rows.Scan)
		if err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func (s *Store) RevokeInvitation(ctx context.Context, workspaceID, id string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM workspace_invitations WHERE workspace_id=$1 AND id=$2`, workspaceID, id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected == 0 {
		return workspace.ErrNotFound
	}
	return nil
}

func scanInvitation(scan func(...any) error) (workspace.Invitation, error) {
	var invitation workspace.Invitation
	var acceptedAt sql.NullTime
	if err := scan(
		&invitation.ID, &invitation.WorkspaceID, &invitation.Role, &invitation.CreatedBy,
		&invitation.ExpiresAt, &invitation.AcceptedBy, &acceptedAt, &invitation.CreatedAt,
	); err != nil {
		return workspace.Invitation{}, err
	}
	if acceptedAt.Valid {
		accepted := acceptedAt.Time
		invitation.AcceptedAt = &accepted
	}
	return invitation, nil
}
