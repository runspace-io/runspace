package resourcegraph

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type mcpScope struct {
	WorkspaceID string
	ChannelID   string
	ThreadID    string
	AgentID     string
}

func requestMCPScope(request *http.Request) mcpScope {
	return mcpScope{
		WorkspaceID: chi.URLParam(request, "workspaceID"),
		ChannelID:   chi.URLParam(request, "channelID"),
		ThreadID:    strings.TrimSpace(request.URL.Query().Get("thread_id")),
		AgentID:     strings.TrimSpace(request.URL.Query().Get("agent_id")),
	}
}

func scopedArg(args map[string]any, key, fallback string) string {
	return defaultString(stringArg(args, key), fallback)
}

func scopedSearchThread(kind Kind, args map[string]any, scope mcpScope) string {
	if explicit := stringArg(args, "thread_id"); explicit != "" {
		return explicit
	}
	switch kind {
	case KindTask, KindArtifact, KindAction, KindDiscussion, KindPolicy, KindEvent:
		return scope.ThreadID
	default:
		return ""
	}
}

func scopeInstructions(scope mcpScope) string {
	base := "Search Runspace resources and publish governed shared work. Use ui.list_components and ui.get_component_schema before creating runspace.ui/v1 artifacts; never emit executable JavaScript. Private local agent prompts, transcripts, paths, and credentials are never exposed."
	if scope.ThreadID != "" {
		message := base + " Use read_discussion for the shared chat. New work defaults to discussion thread " + scope.ThreadID + "."
		if scope.AgentID != "" {
			message += " send_message publishes as the connected agent."
		}
		return message
	}
	if scope.ChannelID != "" {
		return base + " This connection is scoped to channel " + scope.ChannelID + "."
	}
	return base
}
