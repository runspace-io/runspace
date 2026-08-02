package persistence

import (
	"context"
	"database/sql"

	"github.com/runspace/runspace/internal/collaboration"
)

func (s *Store) CreateThread(ctx context.Context, thread collaboration.Thread) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO threads (id,workspace_id,channel_id,parent_thread_id,parent_message_id,visibility,title,created_by,created_at) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),$6,$7,$8,$9)`, thread.ID, thread.WorkspaceID, thread.ChannelID, thread.ParentThreadID, thread.ParentMessageID, defaultThreadVisibility(thread.Visibility), thread.Title, thread.CreatedBy, thread.CreatedAt)
	return err
}

const threadColumns = `t.id,t.workspace_id,COALESCE(t.channel_id,''),COALESCE(t.parent_thread_id,''),COALESCE(t.parent_message_id,''),t.visibility,t.title,t.created_by,t.created_at`

func scanThread(rows *sql.Rows, item *collaboration.Thread) error {
	return rows.Scan(&item.ID, &item.WorkspaceID, &item.ChannelID, &item.ParentThreadID, &item.ParentMessageID, &item.Visibility, &item.Title, &item.CreatedBy, &item.CreatedAt)
}

func defaultThreadVisibility(visibility string) string {
	if visibility == "" {
		return collaboration.ThreadVisibilityPublic
	}
	return visibility
}

// ListThreads returns only top-level, channel-rooted threads — subthreads
// (ParentThreadID set) are reached through ListThreadsByParentThreadID and
// ListThreadsByCreator instead, so existing "this channel's thread" lookups
// are not surprised by subthreads mixed into the result.
func (s *Store) ListThreads(ctx context.Context, userID, workspaceID string) ([]collaboration.Thread, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+threadColumns+` FROM threads t JOIN workspace_members m ON m.workspace_id=t.workspace_id WHERE t.workspace_id=$1 AND m.user_id=$2 AND t.parent_thread_id IS NULL ORDER BY t.created_at`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []collaboration.Thread
	for rows.Next() {
		var item collaboration.Thread
		if err := scanThread(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListThreadsByParentThreadID returns every subthread anchored to a message
// inside parentThreadID, regardless of visibility; the service layer filters
// by viewer.
func (s *Store) ListThreadsByParentThreadID(ctx context.Context, workspaceID, parentThreadID string) ([]collaboration.Thread, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+threadColumns+` FROM threads t WHERE t.workspace_id=$1 AND t.parent_thread_id=$2 ORDER BY t.created_at`, workspaceID, parentThreadID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]collaboration.Thread, 0)
	for rows.Next() {
		var item collaboration.Thread
		if err := scanThread(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListThreadsByCreator returns every thread of the given visibility that
// userID created, across the workspace — the viewer's private-threads tab.
func (s *Store) ListThreadsByCreator(ctx context.Context, workspaceID, userID, visibility string) ([]collaboration.Thread, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+threadColumns+` FROM threads t WHERE t.workspace_id=$1 AND t.created_by=$2 AND t.visibility=$3 AND t.parent_message_id IS NOT NULL ORDER BY t.created_at`, workspaceID, userID, visibility)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]collaboration.Thread, 0)
	for rows.Next() {
		var item collaboration.Thread
		if err := scanThread(rows, &item); err != nil {
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

// ListMessages excludes a private thread's messages for anyone but its
// creator — the WHERE clause reuses $3 (userID) rather than requiring a
// second round trip to check thread visibility.
func (s *Store) ListMessages(ctx context.Context, userID, workspaceID, threadID string) ([]collaboration.Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.thread_id,m.actor_id,m.actor_type,m.body,m.created_at FROM thread_messages m JOIN threads t ON t.id=m.thread_id JOIN workspace_members wm ON wm.workspace_id=t.workspace_id WHERE t.workspace_id=$1 AND t.id=$2 AND wm.user_id=$3 AND (t.visibility IS DISTINCT FROM $4 OR t.created_by=$3) ORDER BY m.created_at`, workspaceID, threadID, userID, collaboration.ThreadVisibilityPrivate)
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
