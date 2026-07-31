# Data, API, and Realtime Contracts

## Core data model

All records use UUIDv7 primary keys plus `created_at` and `updated_at` where
appropriate.

| Record                 | Essential fields                                                                           |
| ---------------------- | ------------------------------------------------------------------------------------------ |
| `users`                | identity, display name, avatar                                                             |
| `accounts`, `sessions` | Auth.js-managed OAuth/session records                                                      |
| `workspaces`           | name, slug, owner ID, archived time                                                        |
| `workspace_members`    | workspace ID, user ID, role                                                                |
| `repositories`         | workspace ID, provider, owner/name, default branch, encrypted credential reference         |
| `channels`             | workspace ID, name; MVP creates one `general` channel                                      |
| `threads`              | channel ID, repository ID, title, status                                                   |
| `actors`               | workspace ID, type (`user`, `agent`, `system`), referenced identity                        |
| `messages`             | thread ID, actor ID, kind, Markdown body, run ID, sequence                                 |
| `runs`                 | workspace/repository/thread IDs, runtime, task, base SHA, status, timestamps, retry parent |
| `run_log_blocks`       | run ID, sequence range, stream, redacted content                                           |
| `run_commands`         | run ID, alias, safe display command, cwd, status, exit code, duration                      |
| `run_files`            | run ID, path, Git status, old/new blob IDs, status version                                 |
| `git_operations`       | run ID, type, idempotency key, stage, state, safe error                                    |
| `pull_requests`        | repository ID, run ID, provider number, URL, head/base, state                              |
| `notifications`        | workspace/user IDs, type, payload, read time                                               |
| `outbox_events`        | event envelope, publish attempts, published time                                           |
| `consumer_receipts`    | consumer name, event ID, processed time                                                    |

### Constraints

- Membership is unique on `(workspace_id, user_id)`.
- Only one repository is active per workspace in the MVP.
- A partial unique index enforces one non-terminal run per repository.
- Message and log sequences are unique within their parent.
- Git operation idempotency keys are unique per workspace.
- Paths are normalized, relative, slash-separated, and cannot contain `..`.
- Deleting a workspace is initially an asynchronous archive, not a hard delete.

## Ownership

- Workspace service owns workspace, membership, channel, thread, message, and
  notification tables.
- Agent runner owns run, log, and command tables.
- Git service owns repository execution metadata, run file status, operations,
  and PR tables.
- Auth.js owns its adapter tables.

Cross-service joins are allowed only in read models maintained for the gateway
or in explicit query composition. A service must not mutate another service's
tables.

## REST API

All endpoints are under `/api/v1`. Mutating endpoints accept
`Idempotency-Key`; responses include a correlation ID.

### Identity and workspaces

```text
POST   /auth/api-token
GET    /me
GET    /workspaces
POST   /workspaces
GET    /workspaces/{workspaceId}
POST   /workspaces/{workspaceId}/members
POST   /workspaces/{workspaceId}/repository
```

### Threads and messages

```text
GET    /workspaces/{workspaceId}/threads
POST   /workspaces/{workspaceId}/threads
GET    /threads/{threadId}/messages?after=
POST   /threads/{threadId}/messages
```

### Runs

```text
POST   /threads/{threadId}/runs
GET    /runs/{runId}
POST   /runs/{runId}/input
POST   /runs/{runId}/stop
POST   /runs/{runId}/retry
GET    /runs/{runId}/logs?after_sequence=
GET    /runs/{runId}/commands
```

`POST /runs` returns `202 Accepted` with the durable queued run. Starting a
container is asynchronous.

### Files, diff, and terminal

```text
GET    /runs/{runId}/files
GET    /runs/{runId}/file?path=
GET    /runs/{runId}/diff?path=
POST   /runs/{runId}/terminal-sessions
DELETE /runs/{runId}/terminal-sessions/{sessionId}
```

File endpoints enforce normalized repository-relative paths and bound response
size. Binary and oversized files return metadata, not raw content.

### Publish

```text
POST   /runs/{runId}/publish
GET    /git-operations/{operationId}
GET    /runs/{runId}/pull-request
```

Publish input contains base branch, new branch, commit message, PR title, and PR
body. The server derives repository and checkout paths; the client never sends
them.

## Error shape

```json
{
  "error": {
    "code": "RUN_ALREADY_ACTIVE",
    "message": "This repository already has an active run.",
    "retryable": false,
    "details": {}
  },
  "correlation_id": "0191f5de..."
}
```

Messages are safe to display. Internal command output and provider responses
remain in redacted server logs.

## WebSocket protocol

Connect to `/api/v1/realtime` using the short-lived access token during the
handshake. The browser never connects directly to NATS.

### Client frames

```json
{"type":"subscribe","workspace_id":"...","last_event_id":"..."}
{"type":"terminal.input","session_id":"...","data":"..."}
{"type":"terminal.resize","session_id":"...","cols":120,"rows":30}
{"type":"ping"}
```

### Server frames

```json
{"type":"snapshot","workspace_id":"...","data":{}}
{"type":"event","event":{}}
{"type":"terminal.output","session_id":"...","sequence":42,"data":"..."}
{"type":"subscribed","workspace_id":"...","cursor":"..."}
{"type":"error","code":"FORBIDDEN","message":"..."}
{"type":"pong"}
```

Domain events and terminal frames are distinct. Terminal input/output is
low-latency session traffic and is not published as a general domain event.
Auditable command summaries and bounded redacted logs are durable.

### Connection behavior

- The gateway authorizes every subscription against current membership.
- One connection may subscribe only to a bounded number of workspaces.
- Heartbeats detect dead clients.
- The client reconnects with exponential backoff and its last durable cursor.
- The server sends a current snapshot if the replay cursor is unavailable.
- Backpressure drops/coalesces nonessential live output, never state changes.
- Token expiry triggers a refresh flow without losing the replay cursor.

## Frontend data rules

- TanStack Query owns REST snapshots and mutations.
- WebSocket events update or invalidate query entries by aggregate.
- The timeline uses cursor pagination and Virtuoso.
- Optimistic updates are limited to reversible UI actions such as a pending
  message; run and Git states use confirmed server state.
- URL state identifies workspace, thread, selected file, and active panel so a
  refresh restores context.
