package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/runspace/runspace/internal/contracts"
)

const (
	defaultDebounce = 150 * time.Millisecond
	watcherActor    = "repository-watcher"
)

// StatusReader is deliberately narrower than GitProvider: a filesystem hint
// never infers changes from fsnotify alone; git status is authoritative.
type StatusReader interface {
	Status(context.Context, string) (string, error)
}

type StatusPublisher interface {
	Publish(context.Context, contracts.EventEnvelope) error
}

type StatusChanged struct {
	WorkspaceID  string    `json:"workspace_id"`
	RepositoryID string    `json:"repository_id"`
	Status       string    `json:"status"`
	ObservedAt   time.Time `json:"observed_at"`
}

type Watcher struct {
	path       string
	workspace  string
	repository string
	debounce   time.Duration
	status     StatusReader
	publisher  StatusPublisher
	new        func() (*fsnotify.Watcher, error)
	sequence   atomic.Uint64
	mu         sync.Mutex
	stop       context.CancelFunc
	done       chan struct{}
}

func NewWatcher(path, workspaceID, repositoryID string, status StatusReader, publisher StatusPublisher) (*Watcher, error) {
	return NewWatcherWithDebounce(path, workspaceID, repositoryID, defaultDebounce, status, publisher)
}

func NewWatcherWithDebounce(path, workspaceID, repositoryID string, debounce time.Duration, status StatusReader, publisher StatusPublisher) (*Watcher, error) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(repositoryID) == "" {
		return nil, errors.New("repository watcher identity and path are required")
	}
	if status == nil || publisher == nil {
		return nil, errors.New("repository watcher dependencies are required")
	}
	if debounce <= 0 {
		return nil, errors.New("repository watcher debounce must be positive")
	}
	clean, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("resolve repository path: %w", err)
	}
	info, err := os.Stat(clean)
	if err != nil {
		return nil, fmt.Errorf("stat repository path: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("repository watcher path must be a directory")
	}
	return &Watcher{path: clean, workspace: workspaceID, repository: repositoryID, debounce: debounce, status: status, publisher: publisher, new: fsnotify.NewWatcher}, nil
}

// Start launches a single watcher. Calling Start more than once is rejected;
// Stop is idempotent and waits for the fsnotify resources to be released.
func (w *Watcher) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("watcher context is required")
	}
	w.mu.Lock()
	if w.stop != nil {
		w.mu.Unlock()
		return errors.New("repository watcher already started")
	}
	watcher, err := w.new()
	if err != nil {
		w.mu.Unlock()
		return err
	}
	if err := addDirectories(watcher, w.path); err != nil {
		_ = watcher.Close()
		w.mu.Unlock()
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	w.stop, w.done = cancel, done
	w.mu.Unlock()
	go w.run(runCtx, watcher, done)
	return nil
}

func (w *Watcher) Stop() {
	w.mu.Lock()
	cancel, done := w.stop, w.done
	w.stop, w.done = nil, nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (w *Watcher) run(ctx context.Context, watcher *fsnotify.Watcher, done chan struct{}) {
	defer close(done)
	defer func() { _ = watcher.Close() }()
	var timer *time.Timer
	var timerC <-chan time.Time
	defer stopTimer(timer)
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			timer, timerC = w.handleEvent(watcher, event, timer)
		case <-timerC:
			timerC = nil
			_ = w.publishStatus(ctx)
		case <-watcher.Errors:
			// fsnotify errors are hints only. A subsequent filesystem event or
			// reconnect can produce another authoritative status refresh.
		}
	}
}

func (w *Watcher) handleEvent(watcher *fsnotify.Watcher, event fsnotify.Event, timer *time.Timer) (*time.Timer, <-chan time.Time) {
	if event.Op&fsnotify.Create != 0 {
		_ = addDirectories(watcher, event.Name)
	}
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		if timer == nil {
			return nil, nil
		}
		return timer, timer.C
	}
	return resetTimer(timer, w.debounce)
}

func (w *Watcher) publishStatus(ctx context.Context) error {
	status, err := w.status.Status(ctx, w.path)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	event, err := contracts.NewEvent(w.nextID(), contracts.EventGitStatusChanged, w.workspace, watcherActor, "system", StatusChanged{WorkspaceID: w.workspace, RepositoryID: w.repository, Status: strings.TrimSpace(status), ObservedAt: now}, now)
	if err != nil {
		return err
	}
	event.RepositoryID = w.repository
	return w.publisher.Publish(ctx, event)
}

func (w *Watcher) nextID() string {
	return fmt.Sprintf("git_status_%d_%d", time.Now().UnixNano(), w.sequence.Add(1))
}

func addDirectories(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
}

func resetTimer(timer *time.Timer, delay time.Duration) (*time.Timer, <-chan time.Time) {
	if timer == nil {
		timer = time.NewTimer(delay)
		return timer, timer.C
	}
	stopTimer(timer)
	timer.Reset(delay)
	return timer, timer.C
}

func stopTimer(timer *time.Timer) {
	if timer != nil && !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
