# Build Tasks: MVP Agent Engineering Workspace

Generated from: `.design/mvp-engineering-workspace/DESIGN_BRIEF.md`  
Date: 2026-07-29

Each checkbox is a vertical slice that must include UI, API/worker behavior,
persistence, authorization, events, failure states, and a verifying test where
those layers apply. Detailed acceptance criteria are in
[`docs/mvp/features/`](../../docs/mvp/features/README.md).

## Verified Implementation Status

This table records current evidence without weakening the stricter acceptance
criteria in the checklist below.

| Capability                         | Status                             | Authoritative evidence                                                                                       |
| ---------------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Local identity and collaboration   | Verified                           | Playwright signs in as admin, adds Alice, and verifies live and replayed attribution                         |
| Workspace/channel hierarchy        | Verified                           | Playwright creates/reopens a workspace and channel; unit tests cover parent inheritance and repository sets  |
| Repository connection and browsing | Verified                           | Playwright clones a local Git repository, expands folders, and opens Monaco code                             |
| Writable contained terminal        | Verified                           | Playwright edits the mounted checkout as UID 1000 and observes the command result                            |
| Event-driven chat and recovery     | Verified                           | PostgreSQL + NATS JetStream + WebSocket path survives gateway restart in Playwright                          |
| Agent collaboration                | Verified for mock and ACP contract | Browser exercises mock lifecycle; Go adapter tests cover ACP framing and lifecycle                           |
| Secrets and members                | Verified                           | Playwright persists and restores channel secrets and workspace members                                       |
| Git change review                  | Verified                           | Native Git integration tests cover statuses/content/safety; Playwright proves terminal edit → Changes → diff |
| Resizable responsive shell         | Verified                           | `react-resizable-panels` persists desktop sizes; captured tablet/mobile drawer screenshots                   |
| Pull-request orchestration         | Verified in containment            | Integration test proves branch, commit, push, and GitHub-compatible PR request                               |
| True PTY semantics                 | Partial                            | Terminal streams a writable shell, but resize/full-screen PTY behavior remains                               |
| Production GitHub OAuth/publishing | Configuration-dependent            | Interfaces and runtime path exist; external credentials are intentionally not embedded in local tests        |

## Foundation

- [ ] **Stream a mock run through the walking skeleton**: Render the calm,
      dense, VS Code/GitHub-inspired workspace shell; start a deterministic mock
      run through REST → PostgreSQL outbox → NATS → worker → WebSocket and display
      ordered output through completion. _Creates app shell, run composer, status,
      timeline, event library, and infrastructure; reuses shadcn/Radix primitives._
- [ ] **Recover the live mock experience**: Refresh, reconnect, duplicate an
      event, and restart the worker while preserving a correct snapshot and
      terminal run state. _Modifies realtime client, outbox consumer, and run
      components; depends on: Stream a mock run._

## Identity and Workspace

- [ ] **Sign in and exchange identity safely**: Add Auth.js GitHub OAuth and
      short-lived API tokens so the shell shows the authenticated actor and
      protected queries reject missing/expired identity. _Creates sign-in screen,
      session boundary, and user menu; reuses shadcn buttons/dropdowns._
- [ ] **Create and reopen a workspace**: Let an authenticated user create a
      workspace with an automatic general channel, list it, and reopen it after a
      new session. _Creates workspace list/switcher/dialog; depends on: Sign in._
- [ ] **Connect one GitHub repository**: Validate access, store encrypted
      provider credentials/metadata, show the default branch, and provide revoked
      credential recovery. _Creates repository connect form and settings summary;
      depends on: Create and reopen a workspace._
- [ ] **Enforce the tenant boundary**: Apply owner/member policies to every
      workspace REST, WebSocket, and event path and pass the ID-substitution test
      matrix. _Modifies gateway/service middleware and all workspace components._

## Repository and Runtime

- [ ] **Prepare a safe repository checkout**: Clone via a cached bare mirror at
      a pinned base SHA, start a limited non-root container, show preparation
      progress, and clean up on failure. _Creates checkout status UI and Git/container
      services; depends on: Connect one GitHub repository._
- [ ] **Browse repository files**: Show a keyboard-accessible react-arborist
      tree and Monaco read-only viewer with safe binary, symlink, oversized, empty,
      and traversal-error states. _Creates file tree and code viewer; depends on:
      Prepare a safe repository checkout._
- [ ] **Use the run terminal**: Attach xterm.js to a scoped PTY, resize it,
      reconnect safely, redact/bound stored output, and terminate it on stop.
      _Creates terminal panel; depends on: Prepare a safe repository checkout._
- [ ] **Detect authoritative file changes**: Coalesce fsnotify hints, refresh
      porcelain Git status, and update the changed-files summary without duplicate
      events. _Creates changed-files list; modifies repository tree; depends on:
      Prepare a safe repository checkout._
- [ ] **Run the reference coding agent**: Implement the runtime interface,
      conformance tests, Codex CLI adapter, normalization, limits, and live
      message/command/log presentation. _Creates agent adapter, command cards, and
      log panel; modifies run composer; depends on: Prepare a safe repository
      checkout._
- [ ] **Control and recover a run**: Send follow-up input, stop idempotently,
      handle timeout/crash/malformed output, and retry in a fresh checkout with a
      linked run. _Creates input request, stop, error, and retry states; modifies
      run controls; depends on: Run the reference coding agent._

## Collaboration and Review

- [ ] **Share a durable human/agent thread**: Create task threads, send
      Markdown messages with pending/failure states, attribute agent actors, and
      render a virtualized timeline that replays correctly. _Creates message
      composer and modifies timeline; depends on: Control and recover a run._
- [ ] **Notify important state changes**: Notify members of agent input
      requests, completion, failure, and PR creation without flooding them with raw
      logs. _Creates notification center and sonner integration; depends on: Share a
      durable human/agent thread._
- [ ] **Review the actual diff**: Select added/modified/deleted/renamed files
      and inspect authoritative Monaco diffs with binary, large-file, and empty
      states. _Creates diff viewer; modifies changed-files list; depends on: Detect
      authoritative file changes._
- [ ] **Publish changes as a GitHub PR**: Validate inputs and execute the
      idempotent branch → commit → push → open-PR state machine with visible stage
      progress and recovery. _Creates publish dialog and PR card; depends on: Review
      the actual diff and Connect one GitHub repository._

## Responsive, Accessible, and Operational

- [ ] **Adapt the workspace across supported screens**: Preserve resizable
      desktop panels, use tablet drawers/tabs, and provide mobile view/stop mode
      with persisted preferences. _Modifies app shell and all major panels._
- [ ] **Complete the accessibility pass**: Meet WCAG 2.2 AA contrast, keyboard
      and focus behavior, status announcements, reduced motion, accessible panel
      resizing, Monaco/terminal labels, and non-color statuses. _Modifies all UI
      components._
- [ ] **Harden the alpha runtime**: Enforce container, path, Git argument,
      credential, redaction, resource, and rate-limit controls and prove them with
      negative tests. _Modifies Git/runner/gateway services and error states._
- [ ] **Make failure operable**: Add reconciliation, backups/restore,
      retention, dashboards, alerts, and runbooks; pass forced-restart scenarios
      for services, NATS, and PostgreSQL. _Modifies shared operations and status
      surfaces._

## Review

- [ ] **Golden-path release review**: Run the automated browser E2E from GitHub
      sign-in through PR creation with the mock runtime and the nightly conformance
      path with the real runtime.
- [ ] **Design review**: Run `/design-review` against the brief at desktop,
      tablet, and mobile sizes and resolve hierarchy, consistency, performance,
      accessibility, and aesthetic-fidelity findings.
- [ ] **Documentation review**: Verify the product scope, REST/event schemas,
      diagrams, operational limits, and feature acceptance scenarios match the
      implementation before tagging the private alpha.
