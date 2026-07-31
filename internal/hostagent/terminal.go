package hostagent

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var terminalUpgrader = websocket.Upgrader{
	CheckOrigin: func(request *http.Request) bool {
		origin := request.Header.Get("Origin")
		return origin == "" ||
			strings.HasPrefix(origin, "http://localhost:") ||
			strings.HasPrefix(origin, "http://127.0.0.1:")
	},
}

func (s *Server) status(writer http.ResponseWriter, _ *http.Request) {
	elevated := isElevated()
	level := "user"
	if elevated {
		level = "administrator"
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"status": "ok", "access_level": level, "elevated": elevated,
	})
}

func (s *Server) openTerminal(writer http.ResponseWriter, request *http.Request) {
	repositoryID := chi.URLParam(request, "repositoryID")
	s.mu.RLock()
	path := s.mirrors[scopedResourceKey(localUserID(request), repositoryID)]
	s.mu.RUnlock()
	if path == "" {
		writeError(writer, http.StatusNotFound, "local folder is not approved for host access")
		return
	}
	requestedLevel := fallback(strings.TrimSpace(request.URL.Query().Get("level")), "user")
	if err := validateTerminalLevel(requestedLevel); err != nil {
		writeError(writer, http.StatusForbidden, err.Error())
		return
	}
	connection, err := terminalUpgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	defer func() { _ = connection.Close() }()
	runHostShell(request.Context(), connection, path)
}

func validateTerminalLevel(level string) error {
	elevated := isElevated()
	if level == "administrator" && !elevated {
		return errors.New("restart the Runspace Host Agent as Administrator to use this terminal")
	}
	if level == "user" && elevated {
		return errors.New("the elevated Host Agent cannot create a standard-user terminal")
	}
	if level != "user" && level != "administrator" {
		return errors.New("unsupported host terminal access level")
	}
	return nil
}

func runHostShell(ctx context.Context, connection *websocket.Conn, path string) {
	command := hostShell(ctx, path)
	stdin, err := command.StdinPipe()
	if err != nil {
		writeTerminalError(connection, err)
		return
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		writeTerminalError(connection, err)
		return
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		writeTerminalError(connection, err)
		return
	}
	if err := command.Start(); err != nil {
		writeTerminalError(connection, err)
		return
	}
	var writes sync.Mutex
	go streamTerminal(connection, stdout, &writes)
	go streamTerminal(connection, stderr, &writes)
	readTerminalInput(connection, stdin)
	_ = stdin.Close()
	_ = command.Process.Kill()
	_ = command.Wait()
}

func hostShell(ctx context.Context, path string) *exec.Cmd {
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NoExit")
	} else {
		shell := fallback(strings.TrimSpace(os.Getenv("SHELL")), "/bin/sh")
		command = exec.CommandContext(ctx, shell, "-i")
	}
	command.Dir = path
	return command
}

func readTerminalInput(connection *websocket.Conn, stdin io.Writer) {
	for {
		var frame struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}
		if err := connection.ReadJSON(&frame); err != nil {
			return
		}
		if frame.Type == "input" {
			_, _ = io.WriteString(stdin, frame.Data)
		}
	}
}

func streamTerminal(connection *websocket.Conn, reader io.Reader, writes *sync.Mutex) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanBytes)
	buffer := make([]byte, 0, 256)
	for scanner.Scan() {
		buffer = append(buffer, scanner.Bytes()...)
		if len(buffer) >= 256 || buffer[len(buffer)-1] == '\n' {
			writeTerminalData(connection, string(buffer), writes)
			buffer = buffer[:0]
		}
	}
	if len(buffer) > 0 {
		writeTerminalData(connection, string(buffer), writes)
	}
}

func writeTerminalData(connection *websocket.Conn, data string, writes *sync.Mutex) {
	writes.Lock()
	defer writes.Unlock()
	_ = connection.WriteJSON(map[string]string{"type": "output", "data": data})
}

func writeTerminalError(connection *websocket.Conn, err error) {
	_ = connection.WriteJSON(map[string]string{"type": "error", "data": err.Error() + "\r\n"})
}
