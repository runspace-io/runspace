package resourcegraph

import (
	"context"
	"fmt"
	"time"
)

func (s *Service) ProjectResource(
	ctx context.Context, id, workspaceID, ownerID, provider, title string, createdAt time.Time,
) error {
	_, err := s.upsert(ctx, ownerID, Node{
		ID: "resource:" + id, WorkspaceID: workspaceID, Kind: KindResource,
		Type: provider, Title: title, OwnerID: ownerID, CreatedAt: createdAt,
		Metadata: map[string]any{"entity_id": id, "provider": provider},
	})
	return err
}

func (s *Service) ProjectDiscussion(
	ctx context.Context, id, workspaceID, channelID, ownerID, title string, createdAt time.Time,
) error {
	_, err := s.upsert(ctx, ownerID, Node{
		ID: "discussion:" + id, WorkspaceID: workspaceID, Kind: KindDiscussion,
		Type: "thread", Title: title, OwnerID: ownerID, CreatedAt: createdAt,
		Metadata: map[string]any{"entity_id": id, "thread_id": id, "channel_id": channelID},
	})
	return err
}

func (s *Service) ProjectAgentTask(
	ctx context.Context, id, workspaceID, threadID, ownerID, agentID, resourceID,
	title, status string, createdAt, updatedAt time.Time,
) error {
	taskID := "task:" + id
	_, err := s.upsert(ctx, ownerID, Node{
		ID: taskID, WorkspaceID: workspaceID, Kind: KindTask, Type: "agent_work",
		Title: title, OwnerID: ownerID, CreatedAt: createdAt, UpdatedAt: updatedAt,
		Metadata: map[string]any{
			"entity_id": id, "thread_id": threadID, "agent_id": agentID,
			"resource_id": resourceID, "status": status,
		},
	})
	if err != nil {
		return err
	}
	_, _ = s.createEdge(ctx, ownerID, Edge{
		ID: "edge:task-discussion:" + id, WorkspaceID: workspaceID, FromID: taskID,
		ToID: "discussion:" + threadID, Relation: "discussed_in", CreatedBy: ownerID,
	})
	_, _ = s.createEdge(ctx, ownerID, Edge{
		ID: "edge:task-resource:" + id, WorkspaceID: workspaceID, FromID: taskID,
		ToID: "resource:" + resourceID, Relation: "uses", CreatedBy: ownerID,
	})
	return nil
}

func (s *Service) ProjectTaskArtifact(
	ctx context.Context, taskID, messageID, workspaceID, threadID, ownerID, title string,
	createdAt time.Time,
) error {
	nodeID := "artifact:" + messageID
	_, err := s.upsert(ctx, ownerID, Node{
		ID: nodeID, WorkspaceID: workspaceID, Kind: KindArtifact, Type: "agent_output",
		Title: title, OwnerID: ownerID, CreatedAt: createdAt,
		Metadata: map[string]any{
			"entity_id": messageID, "thread_id": threadID, "task_id": taskID,
		},
	})
	if err != nil {
		return err
	}
	_, err = s.createEdge(ctx, ownerID, Edge{
		ID: fmt.Sprintf("edge:artifact-task:%s", messageID), WorkspaceID: workspaceID,
		FromID: nodeID, ToID: "task:" + taskID, Relation: "produced_by", CreatedBy: ownerID,
	})
	return err
}
