package persistence

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/runspace/runspace/internal/workspace"
)

func invitationFixture(t *testing.T) (*Store, context.Context, string) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := New(db)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	workspaceID := "ws_inv_" + t.Name()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM workspaces WHERE id=$1`, workspaceID)
	})
	if err := store.CreateWorkspaceWithMember(ctx, workspace.Workspace{
		ID: workspaceID, Slug: "slug-inv-" + t.Name(), Name: t.Name(),
		CreatedBy: "alice", CreatedAt: now, UpdatedAt: now,
	}, workspace.Member{
		WorkspaceID: workspaceID, UserID: "alice",
		Role: workspace.RoleOwner, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	return store, ctx, workspaceID
}

func seedInvitation(
	t *testing.T, store *Store, ctx context.Context, workspaceID, id, hash string,
) workspace.Invitation {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	invitation := workspace.Invitation{
		ID: id, WorkspaceID: workspaceID, Role: workspace.RoleMember,
		CreatedBy: "alice", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}
	if err := store.CreateInvitation(ctx, invitation, hash); err != nil {
		t.Fatal(err)
	}
	return invitation
}

func TestInvitationRoundTripsByTokenHash(t *testing.T) {
	store, ctx, workspaceID := invitationFixture(t)
	seeded := seedInvitation(t, store, ctx, workspaceID, "inv_1", "hash-1")
	loaded, err := store.GetInvitationByTokenHash(ctx, "hash-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != seeded.ID || loaded.Role != workspace.RoleMember ||
		loaded.CreatedBy != "alice" || loaded.AcceptedAt != nil {
		t.Fatalf("invitation did not round-trip: %+v", loaded)
	}
	if _, err := store.GetInvitationByTokenHash(
		ctx, "hash-does-not-exist",
	); !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("unknown hash error=%v", err)
	}
}

// Two people can click the same link at once. The conditional UPDATE is the
// lock, so exactly one redemption may win.
func TestConcurrentRedemptionAdmitsExactlyOne(t *testing.T) {
	store, ctx, workspaceID := invitationFixture(t)
	seedInvitation(t, store, ctx, workspaceID, "inv_race", "hash-race")
	const racers = 8
	var wait sync.WaitGroup
	results := make(chan error, racers)
	wait.Add(racers)
	for i := range racers {
		go func(n int) {
			defer wait.Done()
			results <- store.MarkInvitationAccepted(
				ctx, "inv_race", "user-"+string(rune('a'+n)), time.Now().UTC(),
			)
		}(i)
	}
	wait.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
			continue
		}
		if !errors.Is(err, workspace.ErrNotFound) {
			t.Fatalf("unexpected error from a losing redemption: %v", err)
		}
	}
	if accepted != 1 {
		t.Fatalf("%d redemptions succeeded, want exactly 1", accepted)
	}
}

func TestRevokeRemovesTheInvitation(t *testing.T) {
	store, ctx, workspaceID := invitationFixture(t)
	seedInvitation(t, store, ctx, workspaceID, "inv_revoke", "hash-revoke")
	if err := store.RevokeInvitation(ctx, workspaceID, "inv_revoke"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetInvitationByTokenHash(
		ctx, "hash-revoke",
	); !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("revoked invitation still resolves: %v", err)
	}
	// Revoking something that is already gone is a not-found, not a crash.
	if err := store.RevokeInvitation(
		ctx, workspaceID, "inv_revoke",
	); !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("second revoke error=%v", err)
	}
}

// Invitations belong to their workspace and must not be deletable from another.
func TestRevokeIsScopedToTheWorkspace(t *testing.T) {
	store, ctx, workspaceID := invitationFixture(t)
	seedInvitation(t, store, ctx, workspaceID, "inv_scope", "hash-scope")
	if err := store.RevokeInvitation(
		ctx, "ws_someone_else", "inv_scope",
	); !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("cross-workspace revoke error=%v", err)
	}
	if _, err := store.GetInvitationByTokenHash(ctx, "hash-scope"); err != nil {
		t.Fatalf("invitation was removed by another workspace: %v", err)
	}
}

func TestListInvitationsIsScopedAndOrdered(t *testing.T) {
	store, ctx, workspaceID := invitationFixture(t)
	seedInvitation(t, store, ctx, workspaceID, "inv_a", "hash-a")
	seedInvitation(t, store, ctx, workspaceID, "inv_b", "hash-b")
	items, err := store.ListInvitations(ctx, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected both invitations, got %+v", items)
	}
	other, err := store.ListInvitations(ctx, "ws_someone_else")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("invitations leaked across workspaces: %+v", other)
	}
}
