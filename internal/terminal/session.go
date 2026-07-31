package terminal

import (
	"context"
	"errors"
	"io"
	"sync"
)

const (
	maxCommandBytes = 4096
	maxInputBytes   = 64 * 1024
	outputBuffer    = 64
)

var (
	ErrInvalidRequest = errors.New("invalid terminal request")
	ErrClosed         = errors.New("terminal session is closed")
)

// OpenRequest describes a command in one repository checkout. Root must be an
// absolute, validated directory; factories are responsible for isolation.
type OpenRequest struct {
	WorkspaceID  string
	RepositoryID string
	Root         string
	Command      string
}

// Session is the transport-neutral terminal boundary used by HTTP/WebSocket
// adapters and by future PTY implementations.
type Session interface {
	Input([]byte) error
	Output() <-chan []byte
	Close() error
}

type Factory interface {
	Open(context.Context, OpenRequest) (Session, error)
}

type process interface {
	Start() error
	Wait() error
	Kill() error
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
}

type processSession struct {
	stdin     io.WriteCloser
	process   process
	output    chan []byte
	closeOnce sync.Once
	stateMu   sync.RWMutex
	closed    bool
	closeErr  error
	cancel    context.CancelFunc
}

func newProcessSession(ctx context.Context, command process) (*processSession, error) {
	if command == nil {
		return nil, ErrInvalidRequest
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	session := &processSession{stdin: stdin, process: command, output: make(chan []byte, outputBuffer), cancel: cancel}
	go session.collect(sessionCtx, stdout, stderr)
	return session, nil
}

func (s *processSession) Input(data []byte) error {
	if len(data) == 0 || len(data) > maxInputBytes {
		return ErrInvalidRequest
	}
	s.stateMu.RLock()
	closed := s.closed
	s.stateMu.RUnlock()
	if closed {
		return ErrClosed
	}
	if _, err := s.stdin.Write(data); err != nil {
		return err
	}
	return nil
}

func (s *processSession) Output() <-chan []byte { return s.output }

func (s *processSession) Close() error {
	s.closeOnce.Do(func() {
		s.stateMu.Lock()
		s.closed = true
		s.stateMu.Unlock()
		s.cancel()
		_ = s.stdin.Close()
		s.closeErr = s.process.Kill()
	})
	return s.closeErr
}

func (s *processSession) collect(ctx context.Context, streams ...io.ReadCloser) {
	var readers sync.WaitGroup
	readers.Add(len(streams))
	for _, stream := range streams {
		go func(reader io.ReadCloser) {
			defer readers.Done()
			defer func() { _ = reader.Close() }()
			copyOutput(ctx, reader, s.output)
		}(stream)
	}
	_ = s.process.Wait()
	readers.Wait()
	close(s.output)
}

func copyOutput(ctx context.Context, reader io.Reader, output chan<- []byte) {
	buffer := make([]byte, 16*1024)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			chunk := append([]byte(nil), buffer[:count]...)
			select {
			case output <- chunk:
			case <-ctx.Done():
				return
			}
		}
		if err != nil {
			return
		}
	}
}
