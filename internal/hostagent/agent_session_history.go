package hostagent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type localSessionView struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	AgentID      string                `json:"agent_id"`
	ResourceID   string                `json:"resource_id"`
	ThreadID     string                `json:"thread_id"`
	Status       string                `json:"status"`
	PauseSupport string                `json:"pause_support"`
	Messages     []LocalSessionMessage `json:"messages"`
	Question     *LocalPendingQuestion `json:"question,omitempty"`
	UpdatedAt    string                `json:"updated_at,omitempty"`
}

type localChatSummary struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	AgentID    string `json:"agent_id"`
	ResourceID string `json:"resource_id"`
	Status     string `json:"status"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

func (s *Server) listAgentChats(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	workspaceID := strings.TrimSpace(request.URL.Query().Get("workspace_id"))
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	s.mu.RLock()
	user := s.userConfigLockedReadOnly(userID)
	chats := make([]localChatSummary, 0, len(user.Sessions))
	for id, session := range user.Sessions {
		binding, found := user.Resources[session.ResourceID]
		if workspaceID != "" && (!found || binding.WorkspaceID != workspaceID) {
			continue
		}
		summary := chatSummary(id, session)
		if summary.Title != "Untitled agent chat" {
			chats = append(chats, summary)
		}
	}
	s.mu.RUnlock()
	sort.Slice(chats, func(left, right int) bool {
		return chats[left].UpdatedAt > chats[right].UpdatedAt
	})
	writeJSON(writer, http.StatusOK, map[string]any{"chats": chats})
}

func (s *Server) getAgentSession(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	agentID := chi.URLParam(request, "agentID")
	resourceID := strings.TrimSpace(request.URL.Query().Get("resource_id"))
	threadID := strings.TrimSpace(request.URL.Query().Get("thread_id"))
	taskID := strings.TrimSpace(request.URL.Query().Get("task_id"))
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	if agentID == "" || threadID == "" {
		writeError(writer, http.StatusBadRequest, "agent and thread are required")
		return
	}
	key := localTaskID(userID, agentID, resourceID, threadID, taskID)
	s.mu.RLock()
	session, found := s.userConfigLockedReadOnly(userID).Sessions[key]
	s.mu.RUnlock()
	if !found {
		session = LocalACPSession{
			AgentID: agentID, ResourceID: resourceID, ThreadID: threadID, Status: "draft",
		}
	}
	writeJSON(writer, http.StatusOK, sessionView(key, session))
}

func (s *Server) cancelAgentSession(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	agentID := chi.URLParam(request, "agentID")
	var body struct {
		ResourceID string `json:"resource_id"`
		ThreadID   string `json:"thread_id"`
		TaskID     string `json:"task_id,omitempty"`
	}
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	if decodeRequest(writer, request, &body) != nil || strings.TrimSpace(body.ThreadID) == "" {
		writeError(writer, http.StatusBadRequest, "thread is required")
		return
	}
	key := localTaskID(userID, agentID, body.ResourceID, body.ThreadID, body.TaskID)
	s.sessionMu.Lock()
	session := s.sessions[key]
	s.sessionMu.Unlock()
	if session == nil {
		writeError(writer, http.StatusNotFound, "agent task is not active")
		return
	}
	session.cancelled.Store(true)
	// A turn parked on a question is blocked inside the ACP peer, so release the
	// question first — otherwise the cancel cannot reach the agent until the
	// question times out.
	s.releasePendingQuestion(request.Context(), key, session)
	if err := session.client.Cancel(request.Context(), session.nativeID); err != nil {
		writeError(writer, http.StatusBadGateway, err.Error())
		return
	}
	if err := s.setSessionStatus(key, "cancelled"); err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.pushTaskUpdate(request.Context(), session, nil, "cancelled")
	writeJSON(writer, http.StatusOK, map[string]string{"status": "cancelled"})
}

// appendSessionMessage records one turn message and returns it so callers can
// forward the same identity to the gateway. The sequence counter keeps IDs
// unique when several streamed chunks land inside one clock tick — a coarse
// clock would otherwise mint duplicate IDs and the server would discard chunks.
func (s *Server) appendSessionMessage(
	key, role, kind, body, status string,
) (LocalSessionMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return LocalSessionMessage{}, nil
	}
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	digest := sha256.Sum256(fmt.Appendf(nil, "%s%s%s%d", key, stamp, role, s.messageSeq.Add(1)))
	message := LocalSessionMessage{
		ID:   "local_message_" + hex.EncodeToString(digest[:8]),
		Role: role, Kind: kind, Body: body, CreatedAt: stamp,
	}
	s.mu.Lock()
	session := s.userConfigSessionLocked(key)
	if role == "user" && strings.TrimSpace(session.Title) == "" {
		session.Title = localChatTitle(body)
	}
	session.Status = status
	session.UpdatedAt = stamp
	session.Messages = append(session.Messages, message)
	s.replaceUserSessionLocked(key, session)
	s.mu.Unlock()
	return message, s.saveConfig()
}

func (s *Server) setSessionStatus(key, status string) error {
	s.mu.Lock()
	session := s.userConfigSessionLocked(key)
	session.Status = status
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.replaceUserSessionLocked(key, session)
	s.mu.Unlock()
	return s.saveConfig()
}

func (s *Server) userConfigSessionLocked(key string) LocalACPSession {
	for _, user := range s.config.Users {
		if session, ok := user.Sessions[key]; ok {
			return session
		}
	}
	return LocalACPSession{}
}

func (s *Server) replaceUserSessionLocked(key string, session LocalACPSession) {
	for _, user := range s.config.Users {
		if _, ok := user.Sessions[key]; ok {
			user.Sessions[key] = session
			return
		}
	}
}

func sessionView(key string, session LocalACPSession) localSessionView {
	status := session.Status
	if status == "" {
		status = "ready"
	}
	messages := append([]LocalSessionMessage(nil), session.Messages...)
	if messages == nil {
		messages = []LocalSessionMessage{}
	}
	return localSessionView{
		ID: key, Title: sessionTitle(session), AgentID: session.AgentID, ResourceID: session.ResourceID,
		ThreadID: session.ThreadID, Status: status, PauseSupport: "cancel-only",
		Messages: messages, Question: session.Question, UpdatedAt: session.UpdatedAt,
	}
}

func chatSummary(id string, session LocalACPSession) localChatSummary {
	return localChatSummary{
		ID: id, Title: sessionTitle(session), AgentID: session.AgentID,
		ResourceID: session.ResourceID, Status: fallback(session.Status, "ready"),
		UpdatedAt: session.UpdatedAt,
	}
}

func sessionTitle(session LocalACPSession) string {
	if title := strings.TrimSpace(session.Title); title != "" {
		return title
	}
	for _, message := range session.Messages {
		if message.Role == "user" {
			return localChatTitle(message.Body)
		}
	}
	return "Untitled agent chat"
}

func localChatTitle(input string) string {
	title := strings.TrimSpace(strings.SplitN(strings.TrimSpace(input), "\n", 2)[0])
	for _, prefix := range []string{"please ", "can you ", "could you "} {
		if strings.HasPrefix(strings.ToLower(title), prefix) {
			title = strings.TrimSpace(title[len(prefix):])
			break
		}
	}
	title = strings.TrimRight(title, ".!?")
	runes := []rune(title)
	if len(runes) > 72 {
		return strings.TrimSpace(string(runes[:69])) + "…"
	}
	return fallback(title, "Untitled agent chat")
}
