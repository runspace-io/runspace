package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// memoryInvitationStore mirrors the real store's single-use claim so tests
// exercise the same race guard the database provides.
type memoryInvitationStore struct {
	recordingWorkspaceStore
	invitations map[string]Invitation
	hashes      map[string]string
	members     []Member
}

func newInvitationStore() *memoryInvitationStore {
	return &memoryInvitationStore{
		invitations: map[string]Invitation{}, hashes: map[string]string{},
	}
}

func (s *memoryInvitationStore) CreateInvitation(
	_ context.Context, invitation Invitation, tokenHash string,
) error {
	s.invitations[invitation.ID] = invitation
	s.hashes[tokenHash] = invitation.ID
	return nil
}

func (s *memoryInvitationStore) GetInvitationByTokenHash(
	_ context.Context, tokenHash string,
) (Invitation, error) {
	id, ok := s.hashes[tokenHash]
	if !ok {
		return Invitation{}, ErrNotFound
	}
	return s.invitations[id], nil
}

func (s *memoryInvitationStore) MarkInvitationAccepted(
	_ context.Context, id, userID string, at time.Time,
) error {
	invitation, ok := s.invitations[id]
	if !ok || invitation.AcceptedAt != nil {
		return ErrNotFound
	}
	accepted := at
	invitation.AcceptedBy, invitation.AcceptedAt = userID, &accepted
	s.invitations[id] = invitation
	return nil
}

func (s *memoryInvitationStore) ListInvitations(
	_ context.Context, workspaceID string,
) ([]Invitation, error) {
	items := make([]Invitation, 0)
	for _, invitation := range s.invitations {
		if invitation.WorkspaceID == workspaceID {
			items = append(items, invitation)
		}
	}
	return items, nil
}

func (s *memoryInvitationStore) RevokeInvitation(_ context.Context, workspaceID, id string) error {
	invitation, ok := s.invitations[id]
	if !ok || invitation.WorkspaceID != workspaceID {
		return ErrNotFound
	}
	delete(s.invitations, id)
	return nil
}

func (s *memoryInvitationStore) CreateMember(_ context.Context, member Member) error {
	s.members = append(s.members, member)
	return nil
}

func invitingService(t *testing.T) (*MemoryService, *memoryInvitationStore, string) {
	t.Helper()
	store := newInvitationStore()
	service := NewMemoryService(advancingClock())
	service.SetStore(store)
	created, err := service.CreateWorkspace(
		context.Background(), "alice", CreateWorkspaceRequest{Name: "Invites"},
	)
	if err != nil {
		t.Fatal(err)
	}
	return service, store, created.ID
}

func TestInvitationLetsAnUnknownIdentityJoin(t *testing.T) {
	service, _, workspaceID := invitingService(t)
	_, token, err := service.CreateInvitation(
		context.Background(), "alice", workspaceID, RoleMember, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewInvitation(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if preview.WorkspaceName != "Invites" || preview.Role != RoleMember ||
		preview.InvitedBy != "alice" {
		t.Fatalf("preview=%+v", preview)
	}
	// "carol" was never named by anyone; the token is the whole introduction.
	member, err := service.AcceptInvitation(context.Background(), "carol", token)
	if err != nil {
		t.Fatal(err)
	}
	if member.UserID != "carol" || member.Role != RoleMember {
		t.Fatalf("member=%+v", member)
	}
	visible, err := service.ListWorkspaces(context.Background(), "carol")
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || visible[0].ID != workspaceID {
		t.Fatalf("joined workspace not visible to the new member: %+v", visible)
	}
}

// A single-use link must not admit a second person who also has the URL.
func TestInvitationCannotBeRedeemedTwice(t *testing.T) {
	service, _, workspaceID := invitingService(t)
	_, token, err := service.CreateInvitation(
		context.Background(), "alice", workspaceID, RoleMember, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptInvitation(context.Background(), "carol", token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptInvitation(
		context.Background(), "dave", token,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second redemption error=%v", err)
	}
}

func TestExpiredInvitationIsRejected(t *testing.T) {
	service, store, workspaceID := invitingService(t)
	invitation, token, err := service.CreateInvitation(
		context.Background(), "alice", workspaceID, RoleMember, time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}
	invitation.ExpiresAt = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	store.invitations[invitation.ID] = invitation
	if _, err := service.AcceptInvitation(
		context.Background(), "carol", token,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired invitation error=%v", err)
	}
}

// Only someone who can manage members may widen access to the workspace.
func TestOnlyMemberManagersCanInvite(t *testing.T) {
	service, _, workspaceID := invitingService(t)
	if _, err := service.AddMember(
		context.Background(), "alice", workspaceID, "bob", RoleMember,
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.CreateInvitation(
		context.Background(), "bob", workspaceID, RoleMember, 0,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member could invite: %v", err)
	}
	if _, _, err := service.CreateInvitation(
		context.Background(), "mallory", workspaceID, RoleMember, 0,
	); err == nil {
		t.Fatal("a non-member could invite")
	}
}

// An invitation must not be able to hand out authority equal to its sender's.
func TestOwnerRoleCannotBeInvited(t *testing.T) {
	service, _, workspaceID := invitingService(t)
	if _, _, err := service.CreateInvitation(
		context.Background(), "alice", workspaceID, RoleOwner, 0,
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("owner invitation error=%v", err)
	}
}

// The raw token must never be recoverable from stored state.
func TestOnlyTheTokenHashIsStored(t *testing.T) {
	service, store, workspaceID := invitingService(t)
	_, token, err := service.CreateInvitation(
		context.Background(), "alice", workspaceID, RoleMember, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	for hash := range store.hashes {
		if strings.Contains(hash, token) || hash == token {
			t.Fatal("the raw token was stored")
		}
	}
	if _, err := service.PreviewInvitation(
		context.Background(), token+"x",
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a wrong token resolved: %v", err)
	}
}

func TestRevokedInvitationStopsWorking(t *testing.T) {
	service, _, workspaceID := invitingService(t)
	invitation, token, err := service.CreateInvitation(
		context.Background(), "alice", workspaceID, RoleMember, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeInvitation(
		context.Background(), "alice", workspaceID, invitation.ID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptInvitation(
		context.Background(), "carol", token,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked invitation error=%v", err)
	}
}
