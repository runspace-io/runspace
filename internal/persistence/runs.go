package persistence

import (
	"context"

	"github.com/runspace/runspace/internal/runs"
)

func (s *Store) SaveRun(ctx context.Context, run runs.Run) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO runs (id,workspace_id,thread_id,channel_id,repository_id,prompt,agent_command,status,created_at,updated_at) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,$9,$10) ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status,updated_at=EXCLUDED.updated_at`, run.ID, run.WorkspaceID, run.ThreadID, run.ChannelID, run.RepositoryID, run.Prompt, run.AgentCommand, run.Status, run.CreatedAt, run.UpdatedAt)
	return err
}
func (s *Store) GetRun(ctx context.Context, id string) (runs.Run, error) {
	var run runs.Run
	err := s.db.QueryRowContext(ctx, `SELECT id,workspace_id,COALESCE(thread_id,''),COALESCE(channel_id,''),repository_id,prompt,agent_command,status,created_at,updated_at FROM runs WHERE id=$1`, id).Scan(&run.ID, &run.WorkspaceID, &run.ThreadID, &run.ChannelID, &run.RepositoryID, &run.Prompt, &run.AgentCommand, &run.Status, &run.CreatedAt, &run.UpdatedAt)
	return run, err
}
func (s *Store) ListRuns(ctx context.Context, threadID string) ([]runs.Run, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,workspace_id,COALESCE(thread_id,''),COALESCE(channel_id,''),repository_id,prompt,agent_command,status,created_at,updated_at FROM runs WHERE thread_id=$1 ORDER BY created_at`, threadID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []runs.Run
	for rows.Next() {
		var item runs.Run
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.ThreadID, &item.ChannelID, &item.RepositoryID, &item.Prompt, &item.AgentCommand, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) SaveOutput(ctx context.Context, output runs.Output) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO run_outputs (id,run_id,kind,text,sequence,created_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING`, output.ID, output.RunID, output.Kind, output.Text, output.Sequence, output.CreatedAt)
	return err
}
func (s *Store) ListOutputs(ctx context.Context, runID string) ([]runs.Output, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,run_id,kind,text,sequence,created_at FROM run_outputs WHERE run_id=$1 ORDER BY sequence`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]runs.Output, 0)
	for rows.Next() {
		var item runs.Output
		if err := rows.Scan(&item.ID, &item.RunID, &item.Kind, &item.Text, &item.Sequence, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
