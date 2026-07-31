package resourcegraph

import (
	"context"
	"fmt"
	"net/http"

	"github.com/runspace/runspace/internal/collaboration"
)

const (
	defaultDiscussionLimit = 50
	maxDiscussionLimit     = 200
)

type DiscussionReader interface {
	ListMessages(context.Context, string, string, string) ([]collaboration.Message, error)
}

type AgentMessageWriter interface {
	RecordOutput(context.Context, string, string, string, string, string) (collaboration.Message, error)
}

func (h *MCPHandler) readDiscussion(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (any, error) {
	if h.discussions == nil {
		return nil, fmt.Errorf("%w: discussion context is unavailable", ErrInvalid)
	}
	threadID := scopedArg(args, "thread_id", scope.ThreadID)
	if threadID == "" {
		return nil, fmt.Errorf("%w: thread_id is required outside a discussion", ErrInvalid)
	}
	messages, err := h.discussions.ListMessages(
		request.Context(), userID, scope.WorkspaceID, threadID,
	)
	if err != nil {
		return nil, err
	}
	limit := boundedDiscussionLimit(intArg(args, "limit"))
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	return map[string]any{"thread_id": threadID, "messages": messages}, nil
}

func (h *MCPHandler) sendMessage(
	request *http.Request, userID string, scope mcpScope, args map[string]any,
) (any, error) {
	body := stringArg(args, "body")
	if h.messages == nil || scope.ThreadID == "" || scope.AgentID == "" || body == "" {
		return nil, fmt.Errorf("%w: connected agent, discussion, and body are required", ErrInvalid)
	}
	return h.messages.RecordOutput(
		request.Context(), userID, scope.WorkspaceID, scope.ThreadID, scope.AgentID, body,
	)
}

func boundedDiscussionLimit(limit int) int {
	if limit <= 0 {
		return defaultDiscussionLimit
	}
	if limit > maxDiscussionLimit {
		return maxDiscussionLimit
	}
	return limit
}
