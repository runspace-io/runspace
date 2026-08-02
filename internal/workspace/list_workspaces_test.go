package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/auth"
)

// slowResourceStore delays every resource count, the way a real network round
// trip to the database would. It lets a test observe whether N workspaces cost
// N round trips or one concurrent batch.
type slowResourceStore struct {
	recordingWorkspaceStore
	delay      time.Duration
	workspaces []Workspace
	resources  map[string][]Repository
}

func (s *slowResourceStore) ListWorkspaces(context.Context, string) ([]Workspace, error) {
	return append([]Workspace(nil), s.workspaces...), nil
}

func (s *slowResourceStore) ListRepositories(
	_ context.Context, _, workspaceID string,
) ([]Repository, error) {
	time.Sleep(s.delay)
	return s.resources[workspaceID], nil
}

// The handler used to count each workspace's resources sequentially: N
// workspaces cost N round trips of wall time, invisible with a handful of
// workspaces and severe with the hundred a long-lived team accumulates.
func TestListWorkspacesCountsResourcesConcurrently(t *testing.T) {
	const workspaceCount = 20
	store := &slowResourceStore{delay: 40 * time.Millisecond, resources: map[string][]Repository{}}
	for i := range workspaceCount {
		id := fmt.Sprintf("ws_%02d", i)
		store.workspaces = append(store.workspaces, Workspace{ID: id, Name: id})
		store.resources[id] = []Repository{{ID: "r_" + id, WorkspaceID: id}}
	}
	service := NewMemoryService(fixedClock)
	service.SetStore(store)
	handler := NewHandler(service).Routes()

	request := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	request = request.WithContext(auth.WithUserID(request.Context(), "admin"))
	response := httptest.NewRecorder()
	started := time.Now()
	handler.ServeHTTP(response, request)
	elapsed := time.Since(started)

	// Sequential would take workspaceCount*delay (800ms here); concurrent stays
	// close to one delay. The midpoint is a wide margin against test flakiness
	// while still failing hard if the N+1 pattern comes back.
	if elapsed > (workspaceCount*store.delay)/2 {
		t.Fatalf("took %s, looks sequential rather than concurrent", elapsed)
	}

	var payload struct {
		Workspaces []struct {
			ID            string `json:"id"`
			ResourceCount int    `json:"resource_count"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Workspaces) != workspaceCount {
		t.Fatalf("got %d workspaces, want %d", len(payload.Workspaces), workspaceCount)
	}
	// Concurrency must not scramble the result order the caller asked for.
	for i, item := range payload.Workspaces {
		want := fmt.Sprintf("ws_%02d", i)
		if item.ID != want || item.ResourceCount != 1 {
			t.Fatalf("index %d: got %+v, want id=%s count=1", i, item, want)
		}
	}
}
