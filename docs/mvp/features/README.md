# Feature Implementation Guides

Build the MVP as vertical slices. Each slice must be demonstrable through the
UI, persist its truth, publish the relevant events, enforce authorization, and
include failure states.

| Order | Feature                                            | User-visible outcome                      | Main risk retired    |
| ----- | -------------------------------------------------- | ----------------------------------------- | -------------------- |
| 0     | [Walking skeleton](00-walking-skeleton.md)         | A fake run streams end to end             | System integration   |
| 1     | [Identity and workspace](01-identity-workspace.md) | User enters a connected workspace         | Auth/tenant boundary |
| 2     | [Repository sandbox](02-repository-sandbox.md)     | User browses a safe checkout and terminal | Isolation/Git        |
| 3     | [Agent runs](03-agent-runs.md)                     | Real agent works and can be controlled    | Runtime variability  |
| 4     | [Collaboration timeline](04-collaboration.md)      | Humans and agents share durable context   | Replay/order         |
| 5     | [Review and publish](05-review-publish.md)         | Changes become a GitHub PR                | Git correctness      |
| 6     | [Alpha hardening](06-alpha-hardening.md)           | System survives expected failures         | Operability/security |

## Shared definition of done

A feature is done only when:

- its success, empty, loading, unauthorized, retryable-error, and terminal
  states are represented where applicable;
- workspace authorization is tested;
- durable mutations use an outbox event;
- consumers are idempotent;
- relevant traces and metrics exist;
- documentation and API/event schemas match the implementation;
- the vertical slice has an automated integration or E2E test.
