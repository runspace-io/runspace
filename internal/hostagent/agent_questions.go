package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	acpruntime "github.com/runspace/runspace/internal/runtime"
)

// LocalPendingQuestion is a permission request the agent is blocked on. It
// lives on the session so the owner's own UI can render it, and is mirrored to
// the gateway so anyone holding the right grant can answer instead.
type LocalPendingQuestion struct {
	ID      string                        `json:"id"`
	Title   string                        `json:"title"`
	Options []acpruntime.PermissionOption `json:"options"`
	AskedAt string                        `json:"asked_at"`
}

type answerQuestionRequest struct {
	ResourceID string `json:"resource_id"`
	ThreadID   string `json:"thread_id"`
	TaskID     string `json:"task_id,omitempty"`
	QuestionID string `json:"question_id"`
	OptionID   string `json:"option_id"`
}

// parkQuestion records a question and mirrors it to the gateway. The turn stays
// blocked inside the ACP peer until someone answers or it times out.
func (s *Server) parkQuestion(
	ctx context.Context, session *agentSession,
	chunk acpruntime.ACPNotification, state *turnState,
) error {
	request, ok := decodePermissionRequest(chunk.Payload)
	if !ok {
		return nil
	}
	question := LocalPendingQuestion{
		ID: request.QuestionID, Title: request.Title, Options: request.Options,
		AskedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	s.setSessionQuestion(session.publicID, &question)
	// Announce before releasing the caller. park() unblocks the waiting HTTP
	// request, and a question the caller can see but the gateway has not yet
	// recorded cannot be answered by anyone else.
	err := s.pushTaskEvent(ctx, session, nil, "waiting_approval", &question)
	state.park(question)
	return err
}

// decodePermissionRequest is defensive: the ACP client marshals this payload
// itself, so a malformed one is a bug rather than a transport failure. There is
// no question ID to answer with, so the request can only wait out its timeout.
func decodePermissionRequest(payload []byte) (acpruntime.PermissionRequest, bool) {
	var request acpruntime.PermissionRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return acpruntime.PermissionRequest{}, false
	}
	return request, request.QuestionID != "" && len(request.Options) > 0
}

func (s *Server) setSessionQuestion(key string, question *LocalPendingQuestion) {
	s.mu.Lock()
	session := s.userConfigSessionLocked(key)
	session.Question = question
	if question != nil {
		session.Status = "waiting_approval"
	}
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.replaceUserSessionLocked(key, session)
	s.mu.Unlock()
	_ = s.saveConfig()
}

// releasePendingQuestion cancels any parked question so a blocked turn can wind
// down. Answering may fail if it already resolved, which is fine.
func (s *Server) releasePendingQuestion(
	ctx context.Context, key string, session *agentSession,
) {
	s.mu.RLock()
	pending := s.userConfigSessionLocked(key).Question
	s.mu.RUnlock()
	if pending == nil {
		return
	}
	_ = session.client.AnswerPermission(ctx, pending.ID, "")
	s.clearSessionQuestion(key)
}

func (s *Server) clearSessionQuestion(key string) {
	s.mu.RLock()
	pending := s.userConfigSessionLocked(key).Question
	s.mu.RUnlock()
	if pending == nil {
		return
	}
	s.setSessionQuestion(key, nil)
}

func (s *Server) answerAgentQuestion(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	agentID := chi.URLParam(request, "agentID")
	var body answerQuestionRequest
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	if decodeRequest(writer, request, &body) != nil ||
		strings.TrimSpace(body.ThreadID) == "" || strings.TrimSpace(body.QuestionID) == "" {
		writeError(writer, http.StatusBadRequest, "thread and question are required")
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
	if err := session.client.AnswerPermission(
		request.Context(), body.QuestionID, body.OptionID,
	); err != nil {
		writeError(writer, http.StatusConflict, err.Error())
		return
	}
	// runTurn clears the question when the turn ends, but the agent resumes
	// immediately, so drop it now to stop the UI offering a stale prompt.
	s.clearSessionQuestion(key)
	_ = s.pushTaskUpdate(request.Context(), session, nil, "running")
	writeJSON(writer, http.StatusOK, map[string]string{"status": "answered"})
}
