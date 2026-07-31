# Event Model

## Principles

- An event is a past-tense fact, not an instruction.
- Subject names are stable routing keys; payload schemas are versioned.
- IDs are opaque UUIDv7 values and every event identifies its workspace.
- A service emits an event only after its state transaction succeeds.
- Consumers assume duplicate and out-of-order delivery.
- Sensitive values, complete environment variables, tokens, and unredacted
  terminal input never appear in events.

## Envelope

```json
{
  "specversion": "1.0",
  "id": "0191f5e0-7b57-7b8c-a3e9-3b8c2f5f1d91",
  "source": "agent-runner",
  "type": "run.started.v1",
  "subject": "run/0191f5df...",
  "time": "2026-07-29T10:30:00Z",
  "trace_id": "8f4c...",
  "workspace_id": "0191f5d0...",
  "repository_id": "0191f5d1...",
  "actor": {
    "id": "0191f5d2...",
    "type": "user"
  },
  "correlation_id": "0191f5de...",
  "causation_id": "0191f5dd...",
  "data": {}
}
```

The envelope follows CloudEvents concepts without requiring all transports to
use the CloudEvents wire format.

### Required identifiers

| Field            | Meaning                                        |
| ---------------- | ---------------------------------------------- |
| `id`             | Unique delivery/deduplication key              |
| `workspace_id`   | Authorization and tenant boundary              |
| `repository_id`  | Present when repository-scoped                 |
| `subject`        | Primary aggregate type and ID                  |
| `actor`          | User, agent, or system responsible             |
| `correlation_id` | One user intent across services                |
| `causation_id`   | Direct command/event that caused this event    |
| `trace_id`       | Observability trace, not a domain relationship |

`channel_id`, `thread_id`, `run_id`, `task_id`, `deployment_id`, and
`pull_request_id` belong in `data` when relevant. They are not all nullable
top-level columns.

## NATS subjects

Use two subject families:

```text
cmd.<service>.<command>.v1
evt.<domain>.<fact>.v1
```

Commands are consumed by one owning service through a durable work queue.
Events are broadcast to any interested durable consumer. Examples:

```text
cmd.agent.start.v1
cmd.agent.stop.v1
cmd.git.publish_changes.v1

evt.run.requested.v1
evt.run.started.v1
evt.run.output_appended.v1
evt.run.completed.v1
evt.git.status_changed.v1
evt.pull_request.opened.v1
```

Do not put workspace IDs into the NATS subject. Authorization occurs before a
client subscription, and services filter using the envelope. This avoids an
unbounded subject taxonomy and prevents direct browser access to NATS.

## Initial event catalog

### Workspace and collaboration

| Type                        | Producer  | Durable data |
| --------------------------- | --------- | ------------ |
| `workspace.created.v1`      | workspace | workspace    |
| `workspace.member_added.v1` | workspace | membership   |
| `repository.connected.v1`   | workspace | repository   |
| `thread.created.v1`         | workspace | thread       |
| `message.created.v1`        | workspace | message      |
| `notification.created.v1`   | workspace | notification |

### Run lifecycle

| Type                      | Producer     | Important data                       |
| ------------------------- | ------------ | ------------------------------------ |
| `run.requested.v1`        | workspace    | run ID, runtime, task, base SHA      |
| `run.preparing.v1`        | agent-runner | image/runtime metadata               |
| `run.started.v1`          | agent-runner | container ID alias, start time       |
| `run.output_appended.v1`  | agent-runner | block ID, sequence, stream, content  |
| `run.command_started.v1`  | agent-runner | command alias, cwd, sequence         |
| `run.command_finished.v1` | agent-runner | command alias, exit code, duration   |
| `run.input_requested.v1`  | agent-runner | prompt and allowed response shape    |
| `run.completed.v1`        | agent-runner | finish reason, usage summary         |
| `run.failed.v1`           | agent-runner | safe error code, stage, retryability |
| `run.stop_requested.v1`   | workspace    | requesting actor                     |
| `run.stopped.v1`          | agent-runner | stop reason                          |

### Git and review

| Type                      | Producer | Important data                                |
| ------------------------- | -------- | --------------------------------------------- |
| `git.checkout_ready.v1`   | git      | base branch and SHA                           |
| `git.status_changed.v1`   | git      | added/modified/deleted counts, status version |
| `git.commit_created.v1`   | git      | commit SHA, branch                            |
| `git.push_completed.v1`   | git      | remote and branch                             |
| `pull_request.opened.v1`  | git      | provider, number, URL                         |
| `git.operation_failed.v1` | git      | operation, safe error, retryability           |

`agent.spawn`, `git.commit`, and similar imperative names from early sketches
become commands or API actions. Events use `run.requested`,
`git.commit_created`, and other factual names.

## Aggregate state machines

### Run

```text
queued → preparing → running → completed
                    ├────────→ failed
                    └────────→ stopping → stopped
queued/preparing ───────────────────────→ failed
```

Terminal states are immutable. A retry creates a new run with
`retry_of_run_id`; it does not move a failed run back to queued.

### Git publish operation

```text
requested → branching → committing → pushing → opening_pr → completed
       └──────────────── any stage ───────────────────────→ failed
```

Each stage records enough data to retry safely. Repeating a completed operation
returns the existing result by idempotency key.

## Ordering and replay

- Each aggregate uses a monotonic `aggregate_version`.
- Run output additionally uses a monotonic `sequence`.
- Consumers ignore an event whose aggregate version is already applied.
- Missing versions trigger a database refresh rather than indefinite buffering.
- WebSocket reconnect sends `last_event_id`; the gateway replays available
  events and then returns a current-state snapshot.
- JetStream retention supports operational replay, but product history comes
  from PostgreSQL.

## Output strategy

PTY output can be extremely chatty. The runner:

1. redacts secrets;
2. coalesces bytes for up to 100 ms or a bounded size;
3. writes a run-log block with a sequence;
4. publishes `run.output_appended.v1` pointing to that block and optionally
   carrying a small display payload;
5. lets the UI fetch missing blocks after reconnect.

This preserves live feel without treating every terminal byte as a domain event.

## Schema governance

- JSON Schemas live beside event definitions in code.
- Backward-compatible additions remain in the same version.
- Renames, removals, or semantic changes require a new version.
- Producers publish one version at a time; consumers accept the current and
  previous version during a migration.
- Contract tests validate examples and forbid unknown secret-shaped fields.
