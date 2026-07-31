# Feature 2: Repository Sandbox

## Outcome

The user can prepare a clean, isolated repository checkout, browse files, and
open an interactive terminal inside the run container.

## Build

- Safe native Git wrapper and cached bare mirror.
- Checkout pinned to the selected base SHA.
- Runner container creation with resource/security policy.
- File tree and bounded read-only file endpoint.
- Monaco code viewer with binary/large-file fallbacks.
- xterm.js session over dedicated WebSocket frames.
- `fsnotify` change hints, debounce, and authoritative porcelain Git status.
- Run cleanup and orphan-container reconciliation.
- `git.checkout_ready` and `git.status_changed` events.

The gateway exposes `POST /api/v1/workspaces/{workspaceID}/repositories/{repositoryID}/clone`
to create the repository-data checkout before using the bounded tree/file
endpoints. The destination is derived from the repository ID and cannot be
chosen by the client.

An interactive terminal is available at the WebSocket endpoint
`/api/v1/workspaces/{workspaceID}/repositories/{repositoryID}/terminal`. It
requires workspace write access, accepts bounded text/binary input, and emits
bounded output/error frames. The gateway uses `AGENT_IMAGE` and Docker's
network, CPU, memory, PID, capability, and privilege restrictions.

## MVP HTTP contract

The gateway exposes read-only repository-relative browsing once a checkout is
mounted at `REPOSITORY_ROOT/<repository-id>`:

```text
GET /api/v1/workspaces/{workspace-id}/repositories/{repository-id}/tree?path=src
GET /api/v1/workspaces/{workspace-id}/repositories/{repository-id}/file?path=src/main.go
```

Both endpoints require workspace membership (`X-User-ID` in the MVP). Paths
are normalized and bounded; traversal, absolute paths, symlink escapes,
binary content, and files over the configured read limit are rejected. Tree
entries expose safe fallback states for symlinks, repository metadata, special
files, and oversized files rather than following or reading them.

## Product rules

- Preparing a checkout creates a run-scoped environment; users never browse a
  mutable shared clone.
- Paths are repository-relative and server-normalized.
- One active environment/run is allowed per repository.
- Terminal access is available only while the container is active.
- Terminal content is redacted and bounded before durable storage.
- A retry uses a new clean checkout, not a partially modified directory.

## Acceptance scenarios

1. A connected repository appears as a navigable file tree.
2. Selecting a text file renders correct content in Monaco.
3. Binary, symlink, oversized, ignored, and submodule entries have safe states.
4. Terminal commands run as non-root in the checkout.
5. A file created in the terminal appears in changed files after debouncing.
6. `../`, absolute, encoded traversal, and symlink escape requests fail.
7. Stop/timeout removes the container and terminates the PTY.

## Not included

Browser file editing, port forwarding, dev-server previews, multiple worktrees,
or arbitrary Docker images supplied by users.
