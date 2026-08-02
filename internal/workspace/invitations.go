package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
)

// DefaultInvitationTTL bounds how long an unused link stays valid.
const DefaultInvitationTTL = 7 * 24 * time.Hour

// Invitation binds a not-yet-known identity to a workspace.
//
// Membership otherwise requires an existing member to name someone by their
// exact user ID, which nobody can know in advance for a GitHub-authenticated
// teammate. A link closes that gap without needing a user table: whoever
// redeems it while signed in becomes the member.
//
// The token itself is never stored. Only its hash is, so a leaked database
// cannot be replayed as a pile of working invitations.
type Invitation struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Role        Role       `json:"role"`
	CreatedBy   string     `json:"created_by"`
	ExpiresAt   time.Time  `json:"expires_at"`
	AcceptedBy  string     `json:"accepted_by,omitempty"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// InvitationPreview is what a holder may see before joining: enough to decide,
// and nothing about the workspace's contents or other members.
type InvitationPreview struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	Role          Role   `json:"role"`
	InvitedBy     string `json:"invited_by"`
}

type InvitationStore interface {
	CreateInvitation(context.Context, Invitation, string) error
	GetInvitationByTokenHash(context.Context, string) (Invitation, error)
	MarkInvitationAccepted(context.Context, string, string, time.Time) error
	ListInvitations(context.Context, string) ([]Invitation, error)
	RevokeInvitation(context.Context, string, string) error
}

func (s *MemoryService) invitationStore() (InvitationStore, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, ok := s.store.(InvitationStore)
	return store, ok
}

// CreateInvitation returns the invitation and its token. The token is shown
// exactly once; afterwards only its hash exists.
func (s *MemoryService) CreateInvitation(
	ctx context.Context, userID, workspaceID string, role Role, ttl time.Duration,
) (Invitation, string, error) {
	if err := ctx.Err(); err != nil {
		return Invitation{}, "", err
	}
	if err := requireUser(userID); err != nil {
		return Invitation{}, "", err
	}
	// Owner is deliberately not invitable: it would let a link holder match the
	// authority of the person who sent it.
	if !role.valid() || role == RoleOwner {
		return Invitation{}, "", ErrInvalidInput
	}
	if ttl <= 0 {
		ttl = DefaultInvitationTTL
	}
	if err := s.hydrateWorkspaceMembership(ctx, userID, workspaceID); err != nil {
		return Invitation{}, "", err
	}
	store, ok := s.invitationStore()
	if !ok {
		return Invitation{}, "", ErrInvalidInput
	}
	s.mu.Lock()
	if _, exists := s.workspaces[workspaceID]; !exists {
		s.mu.Unlock()
		return Invitation{}, "", ErrNotFound
	}
	if err := s.authorizeLocked(workspaceID, userID, ActionManageMembers); err != nil {
		s.mu.Unlock()
		return Invitation{}, "", err
	}
	now := s.clock().UTC()
	invitation := Invitation{
		ID: s.nextID("inv"), WorkspaceID: workspaceID, Role: role, CreatedBy: userID,
		ExpiresAt: now.Add(ttl), CreatedAt: now,
	}
	s.mu.Unlock()
	token, err := newInvitationToken()
	if err != nil {
		return Invitation{}, "", err
	}
	if err := store.CreateInvitation(ctx, invitation, hashInvitationToken(token)); err != nil {
		return Invitation{}, "", err
	}
	return invitation, token, nil
}

// PreviewInvitation resolves a token without joining, so the redeem page can
// name the workspace before asking someone to commit.
func (s *MemoryService) PreviewInvitation(
	ctx context.Context, token string,
) (InvitationPreview, error) {
	invitation, err := s.usableInvitation(ctx, token)
	if err != nil {
		return InvitationPreview{}, err
	}
	name := invitation.WorkspaceID
	s.mu.RLock()
	if item, ok := s.workspaces[invitation.WorkspaceID]; ok {
		name = item.Name
	}
	s.mu.RUnlock()
	return InvitationPreview{
		WorkspaceID: invitation.WorkspaceID, WorkspaceName: name,
		Role: invitation.Role, InvitedBy: invitation.CreatedBy,
	}, nil
}

// AcceptInvitation makes the caller a member. The caller's identity comes from
// their session, never from the request body, so a token cannot be redeemed on
// somebody else's behalf.
func (s *MemoryService) AcceptInvitation(
	ctx context.Context, userID, token string,
) (Member, error) {
	if err := requireUser(userID); err != nil {
		return Member{}, err
	}
	invitation, err := s.usableInvitation(ctx, token)
	if err != nil {
		return Member{}, err
	}
	store, ok := s.invitationStore()
	if !ok {
		return Member{}, ErrInvalidInput
	}
	now := s.clock().UTC()
	member := Member{
		WorkspaceID: invitation.WorkspaceID, UserID: userID,
		Role: invitation.Role, CreatedAt: now,
	}
	// Claim the invitation first: a single-use link that added the member but
	// stayed open could be replayed by anyone else holding it.
	if err := store.MarkInvitationAccepted(ctx, invitation.ID, userID, now); err != nil {
		return Member{}, err
	}
	if err := persistMember(ctx, s.store, member); err != nil {
		return Member{}, err
	}
	s.mu.Lock()
	if s.members[invitation.WorkspaceID] == nil {
		s.members[invitation.WorkspaceID] = map[string]Member{}
	}
	s.members[invitation.WorkspaceID][userID] = member
	s.mu.Unlock()
	return member, nil
}

func (s *MemoryService) ListInvitations(
	ctx context.Context, userID, workspaceID string,
) ([]Invitation, error) {
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	if err := s.hydrateWorkspaceMembership(ctx, userID, workspaceID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	err := s.authorizeLocked(workspaceID, userID, ActionManageMembers)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	store, ok := s.invitationStore()
	if !ok {
		return []Invitation{}, nil
	}
	return store.ListInvitations(ctx, workspaceID)
}

func (s *MemoryService) RevokeInvitation(
	ctx context.Context, userID, workspaceID, invitationID string,
) error {
	if err := requireUser(userID); err != nil {
		return err
	}
	if err := s.hydrateWorkspaceMembership(ctx, userID, workspaceID); err != nil {
		return err
	}
	s.mu.RLock()
	err := s.authorizeLocked(workspaceID, userID, ActionManageMembers)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	store, ok := s.invitationStore()
	if !ok {
		return ErrNotFound
	}
	return store.RevokeInvitation(ctx, workspaceID, invitationID)
}

// usableInvitation resolves a token and rejects anything already spent or past
// its expiry. Every failure returns ErrNotFound so probing cannot distinguish
// "wrong token" from "expired token".
func (s *MemoryService) usableInvitation(
	ctx context.Context, token string,
) (Invitation, error) {
	if err := ctx.Err(); err != nil {
		return Invitation{}, err
	}
	if strings.TrimSpace(token) == "" {
		return Invitation{}, ErrNotFound
	}
	store, ok := s.invitationStore()
	if !ok {
		return Invitation{}, ErrNotFound
	}
	invitation, err := store.GetInvitationByTokenHash(ctx, hashInvitationToken(token))
	if err != nil {
		return Invitation{}, ErrNotFound
	}
	if invitation.AcceptedAt != nil || !invitation.ExpiresAt.After(s.clock().UTC()) {
		return Invitation{}, ErrNotFound
	}
	return invitation, nil
}

func newInvitationToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
