# Feature 1: Identity and Workspace

## Outcome

A developer signs in with GitHub, creates a workspace, connects one repository,
and returns later to the same authorized workspace.

## Build

- Auth.js GitHub provider and PostgreSQL adapter.
- Same-origin short-lived API-token exchange.
- Gateway signature validation and actor context.
- Workspace, membership, channel, repository, and encrypted credential records.
- GitHub repository picker or validated `owner/name` entry.
- Repository access/default-branch validation through `RemoteProvider`.
- Workspace list, create dialog, repository-connect flow, and settings summary.
- Owner/member authorization middleware and tests.
- `workspace.created`, `workspace.member_added`, and
  `repository.connected` events.

## Product rules

- Creator becomes owner.
- MVP creates a `general` channel automatically.
- Exactly one active GitHub repository can be connected.
- Connecting does not yet clone; it verifies access and stores metadata.
- Agents do not appear until a run is created.
- Revoked credentials produce a reconnect action without leaking provider data.

## Acceptance scenarios

1. A new GitHub user can create and reopen a workspace.
2. A private repository can be connected when the account has access.
3. An invalid or inaccessible repository is rejected with a useful error.
4. A member can view but cannot change repository settings.
5. A user cannot access another workspace through REST or WebSocket ID changes.
6. Expired API access tokens refresh without requiring a new OAuth login.

## Not included

Google login, GitLab, organizations, invitations by email, SSO, and multiple
repositories.
