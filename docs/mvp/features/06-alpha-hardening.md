# Feature 6: Alpha Hardening

## Outcome

The private alpha is observable, recoverable, and safe enough for invited
developers using controlled repositories.

## Build

- Full authorization and tenant-isolation test matrix.
- Container and secret-handling hardening checklist.
- Reconciliation for runs, containers, outbox, and Git operations.
- Rate limits and resource quotas by user/workspace.
- Backups, migration verification, retention jobs, and restore drill.
- Production dashboards, alerts, and operator runbooks.
- Accessible focus/failure states and desktop responsive polish.
- Golden-path browser E2E against mock runtime plus nightly real-adapter test.
- Dependency/image scanning and documented risk acceptance.

## Release scenarios

1. Gateway, runner, workspace, Git, NATS, and PostgreSQL interruption tests have
   deterministic recovery or explicit failure outcomes.
2. Cross-workspace ID substitution fails across every API and subscription.
3. Known secrets in output fixtures never reach logs, events, or the UI.
4. Run limits stop fork bombs, excessive output, disk fill, and wall-clock
   overruns in test conditions.
5. Database restore recreates durable history and reconciliation cleans runtime
   leftovers.
6. Keyboard navigation, focus management, contrast, reduced motion, and screen
   reader labels pass the documented checks.

## Alpha boundary

Launch is invite-only. Supported configuration is GitHub plus the reference
runtime on the documented Docker host platform. Unsupported providers,
runtimes, or host configurations are not silently accepted.
