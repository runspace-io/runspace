# MVP Architecture

## Context

The system is event-driven because agent work is long-running, stateful, and
observable. Events are used for state changes and live distribution; ordinary
request/response APIs remain the right tool for reads, validation, and
immediate commands.

```text
Browser
  │ HTTPS / WebSocket
  ▼
Next.js web + Auth.js
  │ short-lived API token
  ▼
Go API / realtime gateway
  │ commands, queries, event fan-out
  ├──────────────┬─────────────────┐
  ▼              ▼                 ▼
Workspace       Git              Agent runner
service         service          service
  └──────────────┴──────┬──────────┘
                        ▼
                 NATS JetStream
                        │
                        ▼
                   PostgreSQL
        (system of record + transactional outbox)
```

Docker hosts one isolated container for each run. The repository checkout is a
run-scoped volume or bind mount; the host Docker socket is never mounted inside
the run container.

## Deployable units

Start with a monorepo and four Go binaries. Shared libraries may live in
`internal/`, but service ownership stays explicit.

| Unit           | Responsibility                                                                | Does not own                |
| -------------- | ----------------------------------------------------------------------------- | --------------------------- |
| `web`          | Next.js UI, Auth.js OAuth, API-token exchange                                 | Run execution, Git mutation |
| `gateway`      | Public REST, WebSocket sessions, authorization, query composition             | Long-running jobs           |
| `workspace`    | Workspaces, membership, channels, threads, messages, notifications            | Git commands                |
| `git`          | Repository credentials, clones, status, diffs, branch/commit/push/PR workflow | Agent process               |
| `agent-runner` | Run lifecycle, container isolation, adapter invocation, output normalization  | User auth, PR API           |

For local development these run in Docker Compose with PostgreSQL and NATS.
They can be combined temporarily as processes in one container if operating
cost matters, but they keep separate packages, database ownership, subjects,
and handlers.

## Technology decisions

### Frontend

- Next.js App Router and TypeScript.
- Tailwind CSS and shadcn/ui/Radix primitives.
- TanStack Query for server state.
- `react-resizable-panels` for the workspace shell.
- `react-arborist` for the file tree; defer TanStack Virtual until measured.
- Monaco for read-only code and diffs.
- xterm.js with the fit and WebLinks addons.
- `react-virtuoso` for the timeline/log stream.
- `react-markdown` with sanitization for messages.
- sonner and Lucide.
- React Hook Form plus schema validation.
- Tiptap is deferred: the MVP composer is plain text with Markdown shortcuts.

Using both Shiki and Monaco for code rendering is unnecessary in the first
release. Monaco owns files and diffs; Shiki may later render small code blocks
inside messages.

### Backend and infrastructure

- Go with Chi.
- PostgreSQL.
- Drizzle defines schema and generates reviewed SQL migrations.
- Go accesses PostgreSQL with `pgx`; SQL stays in the owning service package.
- NATS with JetStream for durable domain events and work queues.
- Native Git CLI.
- Docker Engine API from the trusted runner service.
- `fsnotify` for coalesced change hints, followed by authoritative `git status`.
- OpenTelemetry, Prometheus, Grafana, and Loki-compatible structured logs.

## Authentication boundary

Auth.js owns external OAuth and the browser session. The Go services do not
implement OAuth.

1. The browser authenticates with GitHub through Auth.js.
2. A same-origin Next.js route validates the Auth.js session.
3. That route mints a short-lived (five-minute) API access token signed with an
   asymmetric key and containing `user_id`, `session_id`, `aud`, `iss`, and
   `exp`.
4. The browser uses the token for API and WebSocket connection setup.
5. The gateway validates the signature and loads workspace membership for each
   protected operation.

The token carries identity, not workspace permissions. Authorization always
uses current server-side membership.

GitHub installation/access tokens are encrypted at rest and are only exposed to
the Git service for the shortest practical time. Prefer a GitHub App before
public hosting; a personal OAuth token is acceptable only for a tightly
controlled prototype.

## Command and event flow

Commands express intent and can fail:

```text
POST /runs
  → validate membership and repository state
  → persist Run(status=queued) + outbox event in one transaction
  → return 202 with run ID
```

Facts describe accepted state transitions:

```text
run.requested
  → agent-runner claims work
  → creates container
  → run.started
  → run.output.chunk (stream)
  → run.completed | run.failed | run.stopped
```

The gateway sends durable state after a WebSocket connects, then follows the
live event stream. The client must not reconstruct truth solely from ephemeral
chunks.

## Persistence rules

- PostgreSQL is the authoritative source for workspaces, runs, messages,
  repository metadata, Git operations, notifications, and audit history.
- JetStream is the delivery and replay mechanism, not the only database.
- A state change and its outbox row are committed in one database transaction.
- An outbox publisher sends events to JetStream and marks them published.
- Consumers are at-least-once and must be idempotent by `event.id`.
- High-volume output chunks may be batched into durable run-log blocks rather
  than stored as one database row per terminal write.

## Repository and run isolation

Each run receives a fresh checkout created from a cached bare mirror:

```text
bare mirror (read-through cache)
  → run checkout at immutable base SHA
  → agent writes only inside checkout
  → Git service inspects and publishes changes
```

MVP policy permits one active run per repository. This avoids concurrent writes
and ambiguous branch state while leaving room for per-run worktrees later.

Containers run as a non-root user with:

- CPU, memory, process, and wall-clock limits.
- A read/write checkout and dedicated temporary directory.
- No privileged mode or Docker socket.
- A read-only root filesystem where the runtime supports it.
- An explicit outbound-network policy.
- Short-lived secrets injected as files or environment variables and redacted
  from output.

## Agent interface

The internal contract describes capabilities, not a particular CLI:

```go
type Runtime interface {
    Name() string
    Capabilities() Capabilities
    Prepare(ctx context.Context, req PrepareRequest) (PreparedRun, error)
    Start(ctx context.Context, run PreparedRun, sink EventSink) error
    Send(ctx context.Context, runID string, input Input) error
    Stop(ctx context.Context, runID string) error
}
```

`Start` normalizes runtime-specific output into messages, command lifecycle
events, usage records, and raw log blocks. The adapter must tolerate unknown
output and preserve it as raw logs.

## Git provider interface

Local Git operations and remote-provider operations are separate:

```go
type RemoteProvider interface {
    ValidateAccess(ctx context.Context, repo RepoRef) error
    DefaultBranch(ctx context.Context, repo RepoRef) (string, error)
    OpenPullRequest(ctx context.Context, in OpenPRInput) (PullRequest, error)
}
```

Clone, checkout, status, diff, branch, commit, and push use a safe native Git
wrapper with argument arrays, timeouts, bounded output, and no shell string
concatenation.

## Repository shape

The intended structure is:

```text
apps/
  web/
cmd/
  gateway/
  workspace/
  git/
  agent-runner/
internal/
  auth/
  event/
  observability/
  testkit/
services/
  workspace/
  git/
  agent/
db/
  schema/
  migrations/
deploy/
  compose/
docs/
```

This is guidance for implementation, not a requirement to create empty folders.

## Architecture decision records

Create an ADR when changing any of these boundaries:

- Event naming/envelope or delivery semantics.
- Authentication/token exchange.
- Database ownership or migration tool.
- Run isolation or secret delivery.
- Runtime and provider interfaces.
- The one-active-run repository rule.
