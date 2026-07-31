# Security, Operations, and Testing

## Threat model

The agent executes repository-controlled code and model-generated commands.
Treat every checkout and process as untrusted. The primary risks are:

- tenant data leakage;
- credential exfiltration;
- container escape or host access;
- unsafe path traversal and command construction;
- secrets appearing in logs or model context;
- runaway CPU, memory, process, network, or token usage;
- forged events or unauthorized realtime subscriptions.

Private alpha does not remove these risks. It only limits exposure while the
controls are validated.

## Required security controls

### Authorization

- Every API resource lookup is scoped by workspace and current membership.
- Object IDs alone never grant access.
- Gateway-to-service calls carry a signed service identity and actor context.
- Agents have run-scoped capabilities with explicit expiry.
- Administrative operations produce audit records.

### Execution

- Non-root containers with dropped Linux capabilities.
- No privileged flag, host PID/network namespace, or Docker socket mount.
- CPU, memory, PID, disk, and time limits.
- Approved images pinned by digest and scanned.
- Fresh checkout for each retry.
- Network disabled by default; permitted destinations are allowlisted when a
  runtime requires outbound access.
- Stop uses graceful termination followed by a bounded forced kill.
- A reconciler removes orphaned containers and marks abandoned runs failed.

Docker is an MVP isolation boundary, not a sufficient long-term hostile
multi-tenant sandbox by itself. Public hosting should evaluate stronger
isolation such as microVMs or gVisor.

### Secrets and output

- Encrypt provider credentials at rest using a managed key or a local
  development key outside the database.
- Prefer short-lived GitHub App installation tokens.
- Never write credentials into repository remotes.
- Redact exact known secret values plus common token patterns before persistence
  or event publication.
- Bound log, message, file, and diff sizes.
- Sanitize rendered Markdown and disable raw HTML.

### Git safety

- Invoke Git with an argument vector, never a shell-built command.
- Use `--` before user-controlled path arguments.
- Validate branch names with `git check-ref-format`.
- Derive filesystem paths server-side and verify they remain under the run root.
- Protect the base branch and use force push only if a future policy explicitly
  enables it.

## Reliability

### Reconciliation loops

Periodic jobs compare desired state with reality:

- queued/running records against containers;
- active terminal sessions against connections;
- publishing outbox rows against JetStream acknowledgements;
- incomplete Git operations against local/remote branch and PR state.

Reconciliation is mandatory because events can be duplicated, delayed, or
missed by a process during a crash.

### Timeouts and budgets

Initial configurable defaults:

| Budget                   | Default                     |
| ------------------------ | --------------------------- |
| Container preparation    | 2 minutes                   |
| Run wall clock           | 30 minutes                  |
| Graceful stop            | 5 seconds                   |
| Git command              | 2 minutes                   |
| File response            | 2 MiB                       |
| Diff response            | 5 MiB                       |
| Persisted log per run    | 50 MiB                      |
| WebSocket outbound queue | bounded by frames and bytes |

Limits should produce explicit terminal events and user-facing error codes.

The gateway exposes Prometheus-compatible counters at `GET /metrics` for
HTTP request totals, server errors, and domain event totals. This is the
minimal local observability surface; exporters and trace propagation remain
deployment-specific.

## Observability

Instrument boundaries first:

- HTTP request and WebSocket connection spans.
- NATS publish, consume, acknowledgement, and redelivery.
- Run queue wait, container preparation, runtime duration, and stop latency.
- Git operation stages and provider API calls.
- Outbox backlog and oldest unpublished event.

Initial metrics:

```text
http_request_duration_seconds
websocket_connections
event_publish_total
event_consumer_lag_seconds
event_redelivery_total
outbox_pending
runs_total{runtime,status}
run_queue_wait_seconds
run_duration_seconds
run_stop_latency_seconds
containers_active
git_operations_total{operation,status}
```

Every structured log includes service, environment, correlation ID, trace ID,
workspace ID, and relevant aggregate ID. It excludes message bodies, terminal
content, source files, diffs, and secrets by default.

## Test strategy

### Unit

- State-transition guards.
- Authorization policies.
- Event schema and redaction.
- Path/ref validation.
- Agent output normalization.
- Git/provider error mapping.

### Contract

- Event producer examples validate against schemas.
- Each consumer handles duplicate, old, and unknown-compatible events.
- REST and WebSocket frames validate against generated schemas.
- Every agent adapter passes a shared runtime conformance suite.
- Every Git provider passes a shared provider conformance suite.

### Integration

Use real PostgreSQL, NATS JetStream, Docker, and local bare Git repositories:

- transactional outbox survives a process crash;
- duplicate delivery does not duplicate messages/runs/PR operations;
- a stopped run terminates its container;
- file watcher bursts produce a correct authoritative Git status;
- reconnect returns missing log blocks and current run state.

### End to end

The release gate automates the golden path with the mock runtime and a test
GitHub repository. A smaller nightly test exercises the real runtime adapter.

Critical negative cases:

- access another workspace by substituting IDs;
- start two runs for one repository;
- clone failure or revoked provider token;
- agent exits nonzero or emits malformed output;
- NATS or gateway restarts mid-run;
- push rejected because the branch changed;
- browser disconnects and reconnects;
- oversized/binary file and path traversal request.

## Local development

Docker Compose should start:

- PostgreSQL with a health check;
- NATS with JetStream and monitoring;
- the Go services;
- Next.js;
- an OpenTelemetry collector;
- optional Prometheus/Grafana/Loki profiles.

The mock runtime must make the complete product usable without paid AI
credentials. Seed data should create a demo user only in an explicit development
mode.

## Release gates

- All migrations apply forward on a clean database and an anonymized prior
  snapshot.
- Golden-path and authorization E2E suites pass.
- Container escape checklist and secret-redaction fixtures pass.
- No critical image/dependency vulnerability is accepted without a documented
  exception.
- Backup and restore are tested.
- Run/container reconciliation succeeds after forced service termination.
- Operator runbooks cover stuck runs, outbox backlog, NATS outage, GitHub token
  revocation, and disk exhaustion.
