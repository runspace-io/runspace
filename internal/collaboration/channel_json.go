package collaboration

import (
	"encoding/json"
	"time"
)

// MarshalJSON exposes resource naming as the canonical channel contract while
// retaining repository fields for clients that have not migrated yet.
func (channel Channel) MarshalJSON() ([]byte, error) {
	type channelJSON struct {
		ID            string         `json:"id"`
		WorkspaceID   string         `json:"workspace_id"`
		Name          string         `json:"name"`
		ParentID      string         `json:"parent_id,omitempty"`
		ResourceID    string         `json:"resource_id,omitempty"`
		ResourceIDs   []string       `json:"resource_ids,omitempty"`
		RepositoryID  string         `json:"repository_id,omitempty"`
		RepositoryIDs []string       `json:"repository_ids,omitempty"`
		Config        map[string]any `json:"config,omitempty"`
		CreatedBy     string         `json:"created_by"`
		CreatedAt     time.Time      `json:"created_at"`
	}
	return json.Marshal(channelJSON{
		ID: channel.ID, WorkspaceID: channel.WorkspaceID, Name: channel.Name,
		ParentID: channel.ParentID, ResourceID: channel.RepositoryID,
		ResourceIDs: channel.RepositoryIDs, RepositoryID: channel.RepositoryID,
		RepositoryIDs: channel.RepositoryIDs, Config: channel.Config,
		CreatedBy: channel.CreatedBy, CreatedAt: channel.CreatedAt,
	})
}
