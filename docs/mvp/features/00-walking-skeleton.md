# Feature 0: Walking Skeleton

## Outcome

A developer can start the stack, open a seeded demo workspace, start a mock run,
and see ordered output reach the browser through the real API, outbox, NATS, and
WebSocket path.

This slice establishes the named aesthetic: a calm, dense developer workspace
inspired by VS Code and GitHub, not a generic chat dashboard.

## Build

- Monorepo toolchain, reproducible Docker Compose, health checks, and CI.
- Next.js workspace shell with resizable file, timeline, status, and terminal
  placeholders using shadcn primitives.
- Go Chi gateway plus one worker process.
- PostgreSQL migrations for a minimal workspace, run, log block, and outbox.
- NATS streams/consumers created declaratively and idempotently.
- Event envelope library, schema validation, outbox publisher, and consumer
  receipt helper.
- WebSocket subscription and reconnect cursor.
- Mock runtime that emits deterministic started/output/completed events.
- OpenTelemetry correlation across HTTP, outbox publication, consumption, and
  WebSocket fan-out.

## Acceptance scenarios

1. Starting a mock run returns a run ID immediately and reaches `completed`.
2. Output appears incrementally and in order.
3. Refreshing during the run restores state and continues streaming.
4. Publishing the same event twice does not duplicate visible or stored output.
5. Restarting the worker mid-run produces a clear recovered or failed state.
6. A basic keyboard-only pass can reach the run composer and stop control.

## Exit evidence

- One E2E video or test trace of the full mock flow.
- Dashboards show request latency, outbox backlog, consumer lag, and run state.
- Event and REST examples validate against committed schemas.

## Not included

OAuth, real repository cloning, real agent CLIs, interactive PTY, or GitHub PRs.
