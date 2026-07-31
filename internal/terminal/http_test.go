package terminal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func TestHandlerStreamsTerminalFrames(t *testing.T) {
	session := &fakeSession{output: make(chan []byte, 1), input: make(chan []byte, 1)}
	service := &fakeFactory{session: session}
	router := chi.NewRouter()
	NewHandler(service, fakeResolver{}, fakeAuthorizer{}).RegisterRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	endpoint := "ws" + server.URL[len("http"):] + "/workspaces/w1/repositories/r1/terminal?user_id=u&command=sh"
	connection, response, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if response != nil && response.Body != nil {
		defer func() { _ = response.Body.Close() }()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	session.output <- []byte("hello\n")
	if err := connection.WriteJSON(frame{Type: "input", Data: "ls\n"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := connection.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-session.input:
		if string(got) != "ls\n" {
			t.Fatalf("input=%q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal input was not delivered")
	}
}

func TestHandlerRequiresWorkspaceWriteAccess(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(&fakeFactory{}, fakeResolver{}, denyingAuthorizer{}).RegisterRoutes(router)
	request := httptest.NewRequest(http.MethodGet, "/workspaces/w1/repositories/r1/terminal?user_id=u", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recorder.Code)
	}
}

type fakeFactory struct{ session Session }

func (f *fakeFactory) Open(context.Context, OpenRequest) (Session, error) { return f.session, nil }

type fakeResolver struct{}

func (fakeResolver) Root(context.Context, string, string) (string, error) { return "/workspace", nil }

type fakeAuthorizer struct{}

func (fakeAuthorizer) CanWrite(context.Context, string, string) error { return nil }

type denyingAuthorizer struct{}

func (denyingAuthorizer) CanWrite(context.Context, string, string) error {
	return context.DeadlineExceeded
}

type fakeSession struct {
	output chan []byte
	input  chan []byte
}

func (s *fakeSession) Input(data []byte) error { s.input <- data; return nil }
func (s *fakeSession) Output() <-chan []byte   { return s.output }
func (*fakeSession) Close() error              { return nil }
