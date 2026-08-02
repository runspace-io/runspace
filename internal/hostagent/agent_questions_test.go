package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	acpruntime "github.com/runspace/runspace/internal/runtime"
)

// askingACP parks a permission request and only completes the turn once the
// question is answered, mirroring how a real ACP peer blocks mid-turn.
type askingACP struct {
	notices  chan acpruntime.ACPNotification
	answered chan string
	mu       sync.Mutex
	resolved bool
}

func newAskingACP() *askingACP {
	return &askingACP{
		notices:  make(chan acpruntime.ACPNotification, 8),
		answered: make(chan string, 1),
	}
}

func (fake *askingACP) Initialize(context.Context) error { return nil }
func (fake *askingACP) NewSession(context.Context, string) (string, error) {
	return "native-session", nil
}
func (fake *askingACP) ResumeSession(context.Context, string, string) error   { return nil }
func (fake *askingACP) SetSessionModel(context.Context, string, string) error { return nil }

func (fake *askingACP) Prompt(_ context.Context, sessionID, _ string) error {
	payload, _ := json.Marshal(acpruntime.PermissionRequest{
		QuestionID: "q_1_7", Title: "Run rm -rf build/",
		Options: []acpruntime.PermissionOption{
			{ID: "once", Name: "Allow once", Kind: "allow_once"},
			{ID: "reject", Name: "Reject", Kind: "reject_once"},
		},
	})
	fake.notices <- acpruntime.ACPNotification{
		SessionID: sessionID, Kind: acpruntime.NotificationPermissionRequest,
		Text: "Run rm -rf build/", Payload: payload,
	}
	option := <-fake.answered
	if option == "once" {
		fake.notices <- acpruntime.ACPNotification{
			SessionID: sessionID, Kind: "agent_message_chunk", Text: "Removed build/",
		}
	}
	return nil
}

func (fake *askingACP) Cancel(context.Context, string) error { return nil }

func (fake *askingACP) AnswerPermission(_ context.Context, questionID, optionID string) error {
	if questionID != "q_1_7" {
		return context.Canceled
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.resolved {
		return context.Canceled
	}
	fake.resolved = true
	fake.answered <- optionID
	return nil
}

func (fake *askingACP) Notifications() <-chan acpruntime.ACPNotification { return fake.notices }
func (fake *askingACP) Close() error                                     { return nil }

func askingServer(t *testing.T, gatewayURL string) (*Server, *askingACP) {
	t.Helper()
	server := streamingServer(t, gatewayURL, nil)
	peer := newAskingACP()
	server.newACPClient = func(
		context.Context, acpruntime.StdioOptions,
	) (acpruntime.ACPClient, error) {
		return peer, nil
	}
	return server, peer
}

func answerRequest(t *testing.T, server *Server, optionID string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(answerQuestionRequest{
		ResourceID: "resource-1", ThreadID: "thread-1",
		QuestionID: "q_1_7", OptionID: optionID,
	})
	request := httptest.NewRequest(
		http.MethodPost, "/v1/agents/local_agent_test/session/answer", bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "nahid")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

// A human may take minutes to answer, so the prompt request must return as soon
// as the question is parked rather than holding the connection open.
func TestPromptReturnsWhileAgentWaitsOnQuestion(t *testing.T) {
	gateway, pushes := gatewayRecorder(t)
	server, _ := askingServer(t, gateway.URL)
	response := promptOnce(t, server, "clean the build")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var parked struct {
		Status   string `json:"status"`
		Question struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Options []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"options"`
		} `json:"question"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &parked); err != nil {
		t.Fatal(err)
	}
	if parked.Status != "waiting_approval" || parked.Question.ID != "q_1_7" ||
		parked.Question.Title != "Run rm -rf build/" || len(parked.Question.Options) != 2 {
		t.Fatalf("question not surfaced to the caller: %s", response.Body.String())
	}
	var announced bool
	for _, push := range pushes() {
		if push.update.Status == "waiting_approval" && push.update.Question != nil &&
			push.update.Question.ID == "q_1_7" {
			announced = true
		}
	}
	if !announced {
		t.Fatal("question was never mirrored to the gateway")
	}
}

// The whole point of W2: someone answers, and the agent resumes.
//
// This runs against a real HTTP server rather than httptest.NewRecorder,
// because net/http cancels a request context once its handler returns. A parked
// turn returns early, so anything still streaming runs under that cancelled
// context — output produced after the answer was silently dropped before the
// turn was given a detached context. A recorder never cancels and cannot catch
// it.
func TestAnsweringQuestionResumesTheTurn(t *testing.T) {
	gateway, pushes := gatewayRecorder(t)
	server, _ := askingServer(t, gateway.URL)
	host := httptest.NewServer(server.Handler())
	t.Cleanup(host.Close)
	if status := postJSON(t, host.URL+"/v1/agents/local_agent_test/prompt", agentPromptRequest{
		ResourceID: "resource-1", ThreadID: "thread-1", Prompt: "clean the build",
	}); status != http.StatusOK {
		t.Fatalf("prompt status=%d", status)
	}
	if status := postJSON(t,
		host.URL+"/v1/agents/local_agent_test/session/answer", answerQuestionRequest{
			ResourceID: "resource-1", ThreadID: "thread-1",
			QuestionID: "q_1_7", OptionID: "once",
		}); status != http.StatusOK {
		t.Fatalf("answer status=%d", status)
	}
	waitForStatus(t, pushes, "completed")
	var resumed bool
	for _, push := range pushes() {
		for _, message := range push.update.Messages {
			if message.Body == "Removed build/" {
				resumed = true
			}
		}
	}
	if !resumed {
		t.Fatal("agent output after the answer never reached the gateway")
	}
}

func postJSON(t *testing.T, url string, body any) int {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, url, bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "nahid")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode
}

func TestAnsweringUnknownQuestionConflicts(t *testing.T) {
	gateway, _ := gatewayRecorder(t)
	server, _ := askingServer(t, gateway.URL)
	if response := promptOnce(t, server, "clean the build"); response.Code != http.StatusOK {
		t.Fatalf("prompt status=%d", response.Code)
	}
	body, _ := json.Marshal(answerQuestionRequest{
		ResourceID: "resource-1", ThreadID: "thread-1",
		QuestionID: "q_does_not_exist", OptionID: "once",
	})
	request := httptest.NewRequest(
		http.MethodPost, "/v1/agents/local_agent_test/session/answer", bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "nahid")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	_ = answerRequest(t, server, "reject")
}

// The pending question must survive into the session view so a browser that
// reloads mid-question can still render and answer it.
func TestSessionViewExposesPendingQuestion(t *testing.T) {
	gateway, _ := gatewayRecorder(t)
	server, _ := askingServer(t, gateway.URL)
	if response := promptOnce(t, server, "clean the build"); response.Code != http.StatusOK {
		t.Fatalf("prompt status=%d", response.Code)
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/agents/local_agent_test/session?resource_id=resource-1&thread_id=thread-1", nil,
	)
	request.Header.Set("X-User-ID", "nahid")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	var view localSessionView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Status != "waiting_approval" || view.Question == nil || view.Question.ID != "q_1_7" {
		t.Fatalf("session view lost the pending question: %s", response.Body.String())
	}
	_ = answerRequest(t, server, "reject")
}

func waitForStatus(t *testing.T, pushes func() []capturedPush, status string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, push := range pushes() {
			if push.update.Status == status {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("never observed a %q push", status)
}
