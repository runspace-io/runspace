package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/runspace/runspace/internal/collaboration"
	"github.com/runspace/runspace/internal/workspace"
)

type Workspace struct {
	ID, Slug, Name, CreatedBy string
	CreatedAt, UpdatedAt      time.Time
}
type Channel struct {
	ID, WorkspaceID, Name string
	CreatedAt             time.Time
}
type Message struct {
	ID, ChannelID, AuthorID, Body string
	CreatedAt                     time.Time
}

type Store struct{ db *sql.DB }

func configJSON(value any) []byte {
	if value == nil {
		return []byte("{}")
	}
	encoded, _ := json.Marshal(value)
	return encoded
}
func parseConfig(value []byte) map[string]any {
	var output map[string]any
	if len(value) > 0 {
		_ = json.Unmarshal(value, &output)
	}
	return output
}
func parseStringSlice(value []byte) []string {
	var output []string
	if len(value) > 0 {
		_ = json.Unmarshal(value, &output)
	}
	return output
}
func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema+graphSchema)
	return err
}
func (s *Store) CreateWorkspace(ctx context.Context, w Workspace) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspaces (id, slug, name, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6)`, w.ID, w.Slug, w.Name, w.CreatedBy, w.CreatedAt, w.UpdatedAt)
	return err
}

func (s *Store) CreateWorkspaceModel(ctx context.Context, w workspace.Workspace, m workspace.Member) error {
	return s.CreateWorkspace(ctx, Workspace{ID: w.ID, Slug: w.Slug, Name: w.Name, CreatedBy: w.CreatedBy, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt})
}

// CreateWorkspace satisfies workspace.Store while preserving the member row
// in the same transaction as the workspace itself.
func (s *Store) CreateWorkspaceWithMember(ctx context.Context, w workspace.Workspace, m workspace.Member) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspaces (id,slug,name,created_by,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6)`, w.ID, w.Slug, w.Name, w.CreatedBy, w.CreatedAt, w.UpdatedAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role,created_at) VALUES ($1,$2,$3,$4)`, m.WorkspaceID, m.UserID, m.Role, m.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) ListWorkspaces(ctx context.Context, userID string) ([]workspace.Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT w.id,w.slug,w.name,w.created_by,w.created_at,w.updated_at FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id WHERE m.user_id=$1 ORDER BY w.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []workspace.Workspace
	for rows.Next() {
		var w workspace.Workspace
		if err := rows.Scan(&w.ID, &w.Slug, &w.Name, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) GetWorkspace(ctx context.Context, userID, id string) (workspace.Workspace, error) {
	var item workspace.Workspace
	err := s.db.QueryRowContext(ctx, `SELECT w.id,w.slug,w.name,w.created_by,w.created_at,w.updated_at FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id WHERE w.id=$1 AND m.user_id=$2`, id, userID).Scan(&item.ID, &item.Slug, &item.Name, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return workspace.Workspace{}, workspace.ErrNotFound
	}
	return item, err
}

func (s *Store) ListMembers(ctx context.Context, userID, workspaceID string) ([]workspace.Member, error) {
	if err := s.CanRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT workspace_id,user_id,role,created_at FROM workspace_members WHERE workspace_id=$1 ORDER BY created_at`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var members []workspace.Member
	for rows.Next() {
		var item workspace.Member
		if err := rows.Scan(&item.WorkspaceID, &item.UserID, &item.Role, &item.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, item)
	}
	return members, rows.Err()
}

func (s *Store) CreateMember(ctx context.Context, member workspace.Member) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role,created_at) VALUES ($1,$2,$3,$4)`, member.WorkspaceID, member.UserID, member.Role, member.CreatedAt)
	return err
}

func (s *Store) CreateChannel(ctx context.Context, c Channel) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO channels (id,workspace_id,name,created_at) VALUES ($1,$2,$3,$4)`, c.ID, c.WorkspaceID, c.Name, c.CreatedAt)
	return err
}

func (s *Store) CreateCollaborationChannel(ctx context.Context, c collaboration.Channel) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO channels (id,workspace_id,name,parent_id,repository_id,repository_ids,config,created_by,created_at) VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7,$8,$9)`, c.ID, c.WorkspaceID, c.Name, c.ParentID, c.RepositoryID, configJSON(c.RepositoryIDs), configJSON(c.Config), c.CreatedBy, c.CreatedAt)
	return err
}
func (s *Store) UpdateCollaborationChannel(ctx context.Context, c collaboration.Channel) error {
	_, err := s.db.ExecContext(ctx, `UPDATE channels SET name=$1,repository_id=NULLIF($2,''),repository_ids=$3,config=$4 WHERE id=$5 AND workspace_id=$6`, c.Name, c.RepositoryID, configJSON(c.RepositoryIDs), configJSON(c.Config), c.ID, c.WorkspaceID)
	return err
}
func (s *Store) ListCollaborationChannels(ctx context.Context, userID, workspaceID string) ([]collaboration.Channel, error) {
	query, args := collaborationChannelQuery(workspaceID), []any{userID}
	if workspaceID != "" {
		args = []any{workspaceID, userID}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []collaboration.Channel
	for rows.Next() {
		var item collaboration.Channel
		var raw, repositoryIDs []byte
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.ParentID, &item.RepositoryID, &repositoryIDs, &raw, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.RepositoryIDs = parseStringSlice(repositoryIDs)
		item.Config = parseConfig(raw)
		out = append(out, item)
	}
	return out, rows.Err()
}

func collaborationChannelQuery(workspaceID string) string {
	base := `SELECT c.id,c.workspace_id,c.name,COALESCE(c.parent_id,''),COALESCE(c.repository_id,''),COALESCE(c.repository_ids,'[]'::jsonb),COALESCE(c.config,'{}'::jsonb),c.created_by,c.created_at FROM channels c JOIN workspace_members m ON m.workspace_id=c.workspace_id WHERE `
	if workspaceID == "" {
		return base + `m.user_id=$1 ORDER BY c.created_at`
	}
	return base + `c.workspace_id=$1 AND m.user_id=$2 ORDER BY c.created_at`
}

func (s *Store) CreateRepository(ctx context.Context, r workspace.Repository) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO repositories (id,workspace_id,provider,full_name,clone_url,default_branch,created_by,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, r.ID, r.WorkspaceID, r.Provider, r.FullName, r.CloneURL, r.DefaultBranch, r.CreatedBy, r.CreatedAt)
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return workspace.ErrAlreadyExists
	}
	return err
}

func (s *Store) ListRepositories(ctx context.Context, userID, workspaceID string) ([]workspace.Repository, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.workspace_id,r.provider,r.full_name,r.clone_url,r.default_branch,r.created_by,r.created_at FROM repositories r JOIN workspace_members m ON m.workspace_id=r.workspace_id WHERE r.workspace_id=$1 AND m.user_id=$2 ORDER BY r.created_at`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []workspace.Repository
	for rows.Next() {
		var r workspace.Repository
		if err := rows.Scan(&r.ID, &r.WorkspaceID, &r.Provider, &r.FullName, &r.CloneURL, &r.DefaultBranch, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CanRead(ctx context.Context, workspaceID, userID string) error {
	return s.checkMembership(ctx, workspaceID, userID, false)
}

func (s *Store) CanWrite(ctx context.Context, workspaceID, userID string) error {
	return s.checkMembership(ctx, workspaceID, userID, true)
}

func (s *Store) checkMembership(ctx context.Context, workspaceID, userID string, write bool) error {
	var rawRole string
	err := s.db.QueryRowContext(ctx, `SELECT role FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspaceID, userID).Scan(&rawRole)
	if err == sql.ErrNoRows {
		return workspace.ErrForbidden
	}
	if err != nil {
		return err
	}
	if write && workspace.Role(rawRole) == workspace.RoleViewer {
		return workspace.ErrForbidden
	}
	return nil
}

func (s *Store) CreateThread(ctx context.Context, thread collaboration.Thread) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO threads (id,workspace_id,channel_id,title,created_by,created_at) VALUES ($1,$2,NULLIF($3,''),$4,$5,$6)`, thread.ID, thread.WorkspaceID, thread.ChannelID, thread.Title, thread.CreatedBy, thread.CreatedAt)
	return err
}
func (s *Store) ListThreads(ctx context.Context, userID, workspaceID string) ([]collaboration.Thread, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id,t.workspace_id,COALESCE(t.channel_id,''),t.title,t.created_by,t.created_at FROM threads t JOIN workspace_members m ON m.workspace_id=t.workspace_id WHERE t.workspace_id=$1 AND m.user_id=$2 ORDER BY t.created_at`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []collaboration.Thread
	for rows.Next() {
		var item collaboration.Thread
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.ChannelID, &item.Title, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) CreateMessage(ctx context.Context, message collaboration.Message) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO thread_messages (id,thread_id,actor_id,actor_type,body,created_at) VALUES ($1,$2,$3,$4,$5,$6)`, message.ID, message.ThreadID, message.ActorID, message.ActorType, message.Body, message.CreatedAt)
	return err
}
func (s *Store) ListMessages(ctx context.Context, userID, workspaceID, threadID string) ([]collaboration.Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.thread_id,m.actor_id,m.actor_type,m.body,m.created_at FROM thread_messages m JOIN threads t ON t.id=m.thread_id JOIN workspace_members wm ON wm.workspace_id=t.workspace_id WHERE t.workspace_id=$1 AND t.id=$2 AND wm.user_id=$3 ORDER BY m.created_at`, workspaceID, threadID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]collaboration.Message, 0)
	for rows.Next() {
		var item collaboration.Message
		if err := rows.Scan(&item.ID, &item.ThreadID, &item.ActorID, &item.ActorType, &item.Body, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) CreateChannelMessage(ctx context.Context, m Message) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO messages (id,channel_id,author_id,body,created_at) VALUES ($1,$2,$3,$4,$5)`, m.ID, m.ChannelID, m.AuthorID, m.Body, m.CreatedAt)
	return err
}

const schema = `CREATE TABLE IF NOT EXISTS workspaces (id text PRIMARY KEY, slug text NOT NULL UNIQUE, name text NOT NULL, created_by text NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS workspace_members (workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, user_id text NOT NULL, role text NOT NULL, created_at timestamptz NOT NULL, PRIMARY KEY(workspace_id,user_id));
CREATE TABLE IF NOT EXISTS channels (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, name text NOT NULL, parent_id text REFERENCES channels(id) ON DELETE CASCADE, repository_id text, config jsonb NOT NULL DEFAULT '{}'::jsonb, created_by text NOT NULL DEFAULT '', created_at timestamptz NOT NULL);
ALTER TABLE channels ADD COLUMN IF NOT EXISTS parent_id text REFERENCES channels(id) ON DELETE CASCADE;
ALTER TABLE channels ADD COLUMN IF NOT EXISTS repository_id text;
ALTER TABLE channels ADD COLUMN IF NOT EXISTS repository_ids jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE channels ADD COLUMN IF NOT EXISTS config jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE channels ADD COLUMN IF NOT EXISTS created_by text NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS messages (id text PRIMARY KEY, channel_id text NOT NULL REFERENCES channels(id) ON DELETE CASCADE, author_id text NOT NULL, body text NOT NULL, created_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS repositories (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, provider text NOT NULL, full_name text NOT NULL, clone_url text NOT NULL, default_branch text NOT NULL, created_by text NOT NULL, created_at timestamptz NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS repositories_workspace_provider_name ON repositories(workspace_id,provider,full_name);
CREATE UNIQUE INDEX IF NOT EXISTS repositories_workspace_clone_url ON repositories(workspace_id,clone_url);
CREATE TABLE IF NOT EXISTS threads (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, channel_id text REFERENCES channels(id) ON DELETE CASCADE, title text NOT NULL, created_by text NOT NULL, created_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS runs (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, thread_id text, channel_id text, repository_id text NOT NULL, prompt text NOT NULL, agent_command text NOT NULL DEFAULT '', status text NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS run_outputs (id text PRIMARY KEY, run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE, kind text NOT NULL, text text NOT NULL, sequence bigint NOT NULL, created_at timestamptz NOT NULL);
ALTER TABLE threads ADD COLUMN IF NOT EXISTS channel_id text REFERENCES channels(id) ON DELETE CASCADE;
CREATE TABLE IF NOT EXISTS thread_messages (id text PRIMARY KEY, thread_id text NOT NULL REFERENCES threads(id) ON DELETE CASCADE, actor_id text NOT NULL, actor_type text NOT NULL, body text NOT NULL, created_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS channel_secrets (channel_id text NOT NULL REFERENCES channels(id) ON DELETE CASCADE, name text NOT NULL, sealed bytea NOT NULL, updated_at timestamptz NOT NULL, PRIMARY KEY(channel_id,name));
CREATE TABLE IF NOT EXISTS user_agent_metadata (owner_id text NOT NULL, id text NOT NULL, registry_id text NOT NULL, name text NOT NULL, description text NOT NULL DEFAULT '', protocol text NOT NULL, placement text NOT NULL, status text NOT NULL, capabilities jsonb NOT NULL DEFAULT '[]'::jsonb, updated_at timestamptz NOT NULL, PRIMARY KEY(owner_id,id));
CREATE TABLE IF NOT EXISTS agent_tasks (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, thread_id text NOT NULL REFERENCES threads(id) ON DELETE CASCADE, owner_id text NOT NULL, agent_id text NOT NULL, resource_id text NOT NULL, title text NOT NULL, status text NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS agent_task_grants (task_id text NOT NULL, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, owner_id text NOT NULL, agent_id text NOT NULL, principal_id text NOT NULL, role text NOT NULL, permissions jsonb NOT NULL DEFAULT '[]'::jsonb, expires_at timestamptz, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, PRIMARY KEY(task_id,principal_id));
CREATE TABLE IF NOT EXISTS resource_connections (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, plugin_id text NOT NULL, title text NOT NULL, placement text NOT NULL, auth_method text NOT NULL, access_mode text NOT NULL, owner_id text NOT NULL, config jsonb NOT NULL DEFAULT '{}'::jsonb, sealed_credential bytea NOT NULL, capabilities jsonb NOT NULL DEFAULT '[]'::jsonb, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
CREATE INDEX IF NOT EXISTS resource_connections_workspace ON resource_connections(workspace_id,updated_at DESC);`
