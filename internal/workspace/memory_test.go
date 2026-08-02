package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fixedClock() time.Time { return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC) }

func TestMemoryWorkspaceLifecycleAndAuthorization(t *testing.T) {
	s := NewMemoryService(fixedClock)
	ws, err := s.CreateWorkspace(context.Background(), "alice", CreateWorkspaceRequest{Name: "Product Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if ws.Slug != "product-alpha" || ws.ID == "" {
		t.Fatalf("unexpected workspace: %+v", ws)
	}
	if _, err = s.GetWorkspace(context.Background(), "bob", ws.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if _, err = s.AddMember(context.Background(), "alice", ws.ID, "bob", RoleViewer); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConnectRepository(context.Background(), "bob", ws.ID, ConnectRepositoryRequest{Provider: "github", FullName: "o/r", CloneURL: "https://github.com/o/r.git", DefaultBranch: "main"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer should not connect: %v", err)
	}
	if _, err = s.ConnectRepository(context.Background(), "alice", ws.ID, ConnectRepositoryRequest{Provider: "github", FullName: "o/r", CloneURL: "https://github.com/o/r.git", DefaultBranch: "main"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConnectRepository(context.Background(), "alice", ws.ID, ConnectRepositoryRequest{Provider: "github", FullName: "o/other", CloneURL: "https://github.com/o/other.git", DefaultBranch: "main"}); err != nil {
		t.Fatalf("second repository should be allowed: %v", err)
	}
	if _, err = s.ConnectRepository(context.Background(), "alice", ws.ID, ConnectRepositoryRequest{Provider: "github", FullName: "o/r", CloneURL: "https://github.com/o/r.git", DefaultBranch: "main"}); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestRepositoryDuplicateCloneURL(t *testing.T) {
	service := NewMemoryService(fixedClock)
	workspaceModel, err := service.CreateWorkspace(context.Background(), "alice", CreateWorkspaceRequest{Name: "Repos"})
	if err != nil {
		t.Fatal(err)
	}
	request := ConnectRepositoryRequest{Provider: "github", FullName: "o/r", CloneURL: "https://github.com/o/r.git", DefaultBranch: "main"}
	if _, err = service.ConnectRepository(context.Background(), "alice", workspaceModel.ID, request); err != nil {
		t.Fatal(err)
	}
	request.Provider, request.FullName = "gitlab", "o/mirror"
	if _, err = service.ConnectRepository(context.Background(), "alice", workspaceModel.ID, request); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected duplicate clone URL error, got %v", err)
	}
}

func TestListMembers(t *testing.T) {
	service := NewMemoryService(fixedClock)
	workspaceModel, err := service.CreateWorkspace(context.Background(), "alice", CreateWorkspaceRequest{Name: "Members"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddMember(context.Background(), "alice", workspaceModel.ID, "bob", RoleViewer); err != nil {
		t.Fatal(err)
	}
	members, err := service.ListMembers(context.Background(), "alice", workspaceModel.ID)
	if err != nil || len(members) != 2 || !containsMember(members, "bob") {
		t.Fatalf("members=%v err=%v", members, err)
	}
}

func containsMember(members []Member, userID string) bool {
	for _, member := range members {
		if member.UserID == userID {
			return true
		}
	}
	return false
}

func TestAllowsPolicy(t *testing.T) {
	if !Allows(RoleViewer, ActionRead) || Allows(RoleViewer, ActionConnectRepo) {
		t.Fatal("viewer policy incorrect")
	}
	if Allows(RoleMember, ActionConnectRepo) || Allows(RoleMember, ActionManageMembers) {
		t.Fatal("member policy incorrect")
	}
	if Allows(RoleOwner, Action("unknown")) {
		t.Fatal("unknown action must be denied")
	}
}

func TestHTTPRoutes(t *testing.T) {
	h := NewHandler(NewMemoryService(fixedClock)).Routes()
	create := httptest.NewRequest(http.MethodPost, "/workspaces", strings.NewReader(`{"name":"Alpha"}`))
	create.Header.Set("X-User-ID", "alice")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, create)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", res.Code, res.Body.String())
	}
	// Read the ID back rather than assuming its shape; workspace IDs are opaque.
	var created Workspace
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("could not read created workspace: %v body=%s", err, res.Body.String())
	}
	list := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	list.Header.Set("X-User-ID", "alice")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, list)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "Alpha") {
		t.Fatalf("list response=%d %s", res.Code, res.Body.String())
	}
	resource := httptest.NewRequest(http.MethodPost, "/workspaces/"+created.ID+"/resources", strings.NewReader(`{"provider":"folder","full_name":"notes","clone_url":"local-mirror://notes","default_branch":""}`))
	resource.Header.Set("X-User-ID", "alice")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, resource)
	if res.Code != http.StatusCreated {
		t.Fatalf("connect resource status=%d body=%s", res.Code, res.Body.String())
	}
	resources := httptest.NewRequest(http.MethodGet, "/workspaces/"+created.ID+"/resources", nil)
	resources.Header.Set("X-User-ID", "alice")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, resources)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"resources"`) {
		t.Fatalf("list resources response=%d %s", res.Code, res.Body.String())
	}
	members := httptest.NewRequest(http.MethodGet, "/workspaces/"+created.ID+"/members", nil)
	members.Header.Set("X-User-ID", "alice")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, members)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "alice") {
		t.Fatalf("members response=%d %s", res.Code, res.Body.String())
	}
	missingUser := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	res = httptest.NewRecorder()
	h.ServeHTTP(res, missingUser)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("missing user status=%d", res.Code)
	}
}

func TestContextCancellation(t *testing.T) {
	s := NewMemoryService(fixedClock)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.CreateWorkspace(ctx, "u", CreateWorkspaceRequest{Name: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

type recordingWorkspaceStore struct {
	created []Workspace
	repos   []Repository
}

func (r *recordingWorkspaceStore) CreateWorkspaceWithMember(_ context.Context, w Workspace, _ Member) error {
	r.created = append(r.created, w)
	return nil
}
func (r *recordingWorkspaceStore) ListWorkspaces(context.Context, string) ([]Workspace, error) {
	return append([]Workspace(nil), r.created...), nil
}
func (r *recordingWorkspaceStore) CreateRepository(_ context.Context, repo Repository) error {
	r.repos = append(r.repos, repo)
	return nil
}
func (r *recordingWorkspaceStore) ListRepositories(context.Context, string, string) ([]Repository, error) {
	return append([]Repository(nil), r.repos...), nil
}

func TestStoreWriteThrough(t *testing.T) {
	store := &recordingWorkspaceStore{}
	service := NewMemoryService(fixedClock)
	service.SetStore(store)
	ws, err := service.CreateWorkspace(context.Background(), "alice", CreateWorkspaceRequest{Name: "Durable"})
	if err != nil || len(store.created) != 1 || store.created[0].ID != ws.ID {
		t.Fatalf("workspace=%+v stored=%+v err=%v", ws, store.created, err)
	}
	if _, err := service.ConnectRepository(context.Background(), "alice", ws.ID, ConnectRepositoryRequest{Provider: "github", FullName: "o/r", CloneURL: "https://github.com/o/r.git", DefaultBranch: "main"}); err != nil {
		t.Fatal(err)
	}
	if len(store.repos) != 1 {
		t.Fatalf("expected repository write-through, got %d", len(store.repos))
	}
}

// uniqueIDStore models the primary key the real database enforces, which a
// store fake that only appends would let a duplicate ID slide past.
type uniqueIDStore struct {
	recordingWorkspaceStore
}

func (s *uniqueIDStore) CreateWorkspaceWithMember(
	ctx context.Context, w Workspace, m Member,
) error {
	for _, existing := range s.created {
		if existing.ID == w.ID {
			return errors.New("duplicate key value violates unique constraint")
		}
	}
	return s.recordingWorkspaceStore.CreateWorkspaceWithMember(ctx, w, m)
}

// Workspace IDs used to come from a counter that restarted with the process, so
// the first workspace created after a gateway restart collided with one already
// in the database.
func TestWorkspaceIDsSurviveARestart(t *testing.T) {
	store := &uniqueIDStore{}
	clock := advancingClock()
	before := NewMemoryService(clock)
	before.SetStore(store)
	first, err := before.CreateWorkspace(
		context.Background(), "alice", CreateWorkspaceRequest{Name: "First"},
	)
	if err != nil {
		t.Fatal(err)
	}
	// A fresh service is what the gateway has after a restart: empty maps, a
	// zeroed counter, and the same durable store underneath.
	after := NewMemoryService(clock)
	after.SetStore(store)
	second, err := after.CreateWorkspace(
		context.Background(), "alice", CreateWorkspaceRequest{Name: "Second"},
	)
	if err != nil {
		t.Fatalf("creating a workspace after a restart failed: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("restart reused workspace ID %q", first.ID)
	}
}

func advancingClock() Clock {
	current := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	return func() time.Time {
		current = current.Add(time.Millisecond)
		return current
	}
}

type durableMemberStore struct {
	recordingWorkspaceStore
	workspace Workspace
	members   []Member
}

func (s *durableMemberStore) GetWorkspace(context.Context, string, string) (Workspace, error) {
	return s.workspace, nil
}
func (s *durableMemberStore) ListMembers(context.Context, string, string) ([]Member, error) {
	return append([]Member(nil), s.members...), nil
}
func (s *durableMemberStore) CreateMember(_ context.Context, member Member) error {
	s.members = append(s.members, member)
	return nil
}

func TestAddMemberHydratesDurableWorkspaceAfterRestart(t *testing.T) {
	workspaceModel := Workspace{ID: "workspace-1", Name: "Durable", CreatedBy: "alice"}
	store := &durableMemberStore{
		workspace: workspaceModel,
		members: []Member{{
			WorkspaceID: workspaceModel.ID,
			UserID:      "alice",
			Role:        RoleOwner,
		}},
	}
	service := NewMemoryService(fixedClock)
	service.SetStore(store)
	member, err := service.AddMember(
		context.Background(),
		"alice",
		workspaceModel.ID,
		"nahid",
		RoleMember,
	)
	if err != nil {
		t.Fatal(err)
	}
	if member.UserID != "nahid" || len(store.members) != 2 {
		t.Fatalf("member=%+v stored=%+v", member, store.members)
	}
}
