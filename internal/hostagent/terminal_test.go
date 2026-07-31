package hostagent

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHostTerminalRunsInsideApprovedFolder(t *testing.T) {
	server, err := NewServer(&recordingEngine{})
	if err != nil {
		t.Fatal(err)
	}
	server.mirrors[scopedResourceKey("admin", "repository-1")] = t.TempDir()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	level := "user"
	if isElevated() {
		level = "administrator"
	}
	endpoint := "ws" + strings.TrimPrefix(httpServer.URL, "http") +
		"/v1/terminals/repository-1?user_id=admin&level=" + level
	headers := http.Header{"Origin": []string{"http://localhost:3000"}}
	connection, _, err := websocket.DefaultDialer.Dial(endpoint, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	command := "echo HOST_TERMINAL_OK\n"
	if runtime.GOOS == "windows" {
		command = "Write-Output HOST_TERMINAL_OK\n"
	}
	if err := connection.WriteJSON(map[string]string{"type": "input", "data": command}); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(8 * time.Second))
	for {
		var frame struct {
			Data string `json:"data"`
		}
		if err := connection.ReadJSON(&frame); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(frame.Data, "HOST_TERMINAL_OK") {
			return
		}
	}
}
