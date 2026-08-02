package hostagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	acpruntime "github.com/runspace/runspace/internal/runtime"
)

type agentSession struct {
	mu         sync.Mutex
	client     acpruntime.ACPClient
	nativeID   string
	publicID   string
	userID     string
	agentID    string
	resourceID string
	threadID   string
	// cancelled records an operator stop so the turn reports "cancelled" rather
	// than the "failed" the aborted prompt would otherwise produce.
	cancelled atomic.Bool
}

type agentClientFactory func(context.Context, acpruntime.StdioOptions) (acpruntime.ACPClient, error)

func defaultAgentClient(
	ctx context.Context, options acpruntime.StdioOptions,
) (acpruntime.ACPClient, error) {
	return acpruntime.NewStdioACPFactoryWithOptions(options)(ctx)
}

type agentPromptRequest struct {
	ResourceID string `json:"resource_id"`
	ThreadID   string `json:"thread_id"`
	TaskID     string `json:"task_id,omitempty"`
	Prompt     string `json:"prompt"`
}

type agentPromptOutput struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

func (s *Server) promptAgent(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	agentID := chi.URLParam(request, "agentID")
	var body agentPromptRequest
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	if decodeRequest(writer, request, &body) != nil || strings.TrimSpace(body.Prompt) == "" ||
		strings.TrimSpace(body.ThreadID) == "" {
		writeError(writer, http.StatusBadRequest, "prompt and thread are required")
		return
	}
	//nolint:contextcheck // the ACP session is long-lived and intentionally outlives this request
	session, err := s.localAgentSession(userID, agentID, body.ResourceID, body.ThreadID, body.TaskID)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	prompted, err := s.appendSessionMessage(session.publicID, "user", "", body.Prompt, "running")
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.pushTaskUpdate(request.Context(), session, []LocalSessionMessage{prompted}, "running")
	// streamTurn already persisted and forwarded each chunk, and owns the
	// terminal status; the response only replays what the turn produced so the
	// caller's synchronous contract stays intact.
	outcome, err := s.streamTurn(request.Context(), session, body.Prompt)
	if err != nil {
		writeError(writer, http.StatusBadGateway, err.Error())
		return
	}
	response := map[string]any{
		"session_id": session.publicID,
		"outputs":    outcome.outputs,
		"status":     outcome.status,
	}
	if outcome.question != nil {
		response["question"] = outcome.question
	}
	writeJSON(writer, http.StatusOK, response)
}

func (s *Server) localAgentSession(
	userID, agentID, resourceID, threadID, taskID string,
) (*agentSession, error) {
	key := localTaskID(userID, agentID, resourceID, threadID, taskID)
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	if existing := s.sessions[key]; existing != nil {
		return existing, nil
	}
	s.agentMu.RLock()
	launch, found := s.agentLaunch[agentID]
	s.agentMu.RUnlock()
	if !found {
		s.DiscoverAgents()
		s.agentMu.RLock()
		launch, found = s.agentLaunch[agentID]
		s.agentMu.RUnlock()
	}
	if !found {
		return nil, errors.New("local ACP agent is not ready")
	}
	s.mu.RLock()
	userConfig := s.userConfigLockedReadOnly(userID)
	binding := userConfig.Resources[resourceID]
	path := s.mirrors[scopedResourceKey(userID, resourceID)]
	preference := userConfig.Agents[agentID]
	previous := userConfig.Sessions[key]
	s.mu.RUnlock()
	if path == "" {
		return nil, errors.New("resource is not available to this user")
	}
	client, err := s.newACPClient(context.Background(), acpruntime.StdioOptions{
		Command: launch.command, Args: launch.args,
		Env:            launchEnvironment(launch.registryID, preference),
		PermissionMode: preference.PermissionMode,
		MCPServers:     runspaceMCPServers(binding, userID, threadID, agentID),
	})
	if err != nil {
		return nil, err
	}
	if err := client.Initialize(context.Background()); err != nil {
		_ = client.Close()
		return nil, err
	}
	nativeID := previous.SessionID
	if nativeID == "" || client.ResumeSession(context.Background(), nativeID, path) != nil {
		nativeID, err = client.NewSession(context.Background(), path)
		if err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	if preference.Model != "" {
		if modelErr := client.SetSessionModel(context.Background(), nativeID, preference.Model); modelErr != nil && launch.registryID != "codex-acp" {
			_ = client.Close()
			return nil, modelErr
		}
	}
	session := &agentSession{
		client: client, nativeID: nativeID, publicID: key, userID: userID,
		agentID: agentID, resourceID: resourceID, threadID: threadID,
	}
	s.sessions[key] = session
	s.mu.Lock()
	s.userConfigLocked(userID).Sessions[key] = LocalACPSession{
		AgentID: agentID, ResourceID: resourceID, ThreadID: threadID, SessionID: nativeID,
		Status: "ready", Messages: append([]LocalSessionMessage(nil), previous.Messages...),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	s.mu.Unlock()
	if err := s.saveConfig(); err != nil {
		_ = client.Close()
		delete(s.sessions, key)
		return nil, err
	}
	return session, nil
}

// collectSessionNotifications owns target and closes it on return, so the
// consumer can range to completion without racing a send against a close.
// Sends block rather than drop: losing a chunk would silently truncate the
// transcript that the gateway and every grantee read.
func collectSessionNotifications(
	session *agentSession, target chan<- acpruntime.ACPNotification, stop <-chan struct{},
) {
	defer close(target)
	for {
		select {
		case output, ok := <-session.client.Notifications():
			if !ok {
				return
			}
			if output.SessionID != session.nativeID {
				continue
			}
			select {
			case target <- output:
			case <-stop:
				return
			}
		case <-stop:
			return
		}
	}
}

func launchEnvironment(registryID string, preference LocalAgentPreference) map[string]string {
	environment := map[string]string{}
	if registryID != "codex-acp" {
		return environment
	}
	mode := "agent"
	if preference.PermissionMode == "yolo" {
		mode = "agent-full-access"
	}
	environment["INITIAL_AGENT_MODE"] = mode
	if preference.Model != "" {
		config, _ := json.Marshal(map[string]string{"model": preference.Model})
		environment["CODEX_CONFIG"] = string(config)
	}
	return environment
}

func (s *Server) userConfigLockedReadOnly(userID string) *LocalUserConfig {
	user := s.config.Users[userID]
	if user == nil {
		return &LocalUserConfig{
			Resources: map[string]LocalResourceBinding{},
			Agents:    map[string]LocalAgentPreference{},
			Sessions:  map[string]LocalACPSession{},
		}
	}
	return user
}

func localSessionID(userID, agentID, resourceID, threadID string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		userID, agentID, resourceID, threadID,
	}, "\x00")))
	return "local_session_" + hex.EncodeToString(sum[:12])
}

func localTaskID(userID, agentID, resourceID, threadID, taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if strings.HasPrefix(taskID, "local_session_") {
		return taskID
	}
	return localSessionID(userID, agentID, resourceID, threadID)
}

func (s *Server) agentHealth(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	s.sessionMu.Lock()
	active := 0
	for _, session := range s.sessions {
		if session.userID == userID {
			active++
		}
	}
	s.sessionMu.Unlock()
	writeJSON(writer, http.StatusOK, map[string]any{
		"presence": "online", "route": "loopback", "active_sessions": active,
		"observed_at": time.Now().UTC(),
	})
}
