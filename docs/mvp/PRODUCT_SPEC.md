# MVP Product Specification

## Product statement

The product is a shared engineering workspace in which AI agents behave like
collaborators rather than detached chatbots. A developer gives an agent a task
in the context of a repository, watches the work happen, intervenes when
needed, reviews the resulting changes, and publishes them as a pull request.

The differentiator is the orchestration layer connecting identity, repository
state, isolated execution, live events, review, and human collaboration.

## Primary user and job

The primary MVP user is an individual developer or a small engineering team
already using GitHub. Their job is:

> Safely delegate a repository task to an AI coding agent, retain visibility
> and control while it runs, and turn acceptable output into a reviewable PR.

## Golden path

1. The user signs in with GitHub.
2. The user creates a workspace and connects a GitHub repository.
3. The system clones the repository into an isolated run container.
4. The user starts a run by selecting an agent and entering a task.
5. Run status, agent messages, logs, and file changes stream into the UI.
6. The user can send another message, use the terminal, or stop the run.
7. The user reviews the file tree and Monaco diff.
8. The user enters a branch name and commit message.
9. The system commits, pushes, and opens a GitHub pull request.
10. The workspace timeline preserves who did what and when.

## Experience principles

1. **Control over autonomy** — every run exposes its status, inputs, outputs,
   permissions, and a reliable stop control.
2. **Repository truth over chat theater** — files, diffs, commands, commits,
   and PR state are first-class; chat explains work but does not replace it.
3. **Progressive complexity** — the primary path stays simple while logs,
   terminal, event metadata, and recovery controls remain available on demand.

## MVP scope

### Included

- GitHub OAuth through Auth.js.
- Workspaces with owner/member roles.
- One GitHub repository per workspace for the first release.
- Repository cloning and refresh using the native Git CLI.
- One Docker container per agent run.
- A pluggable agent interface with one fully supported runtime. Codex CLI is
  the reference adapter; a mock adapter is retained for tests and demos.
- Shared channel and task thread for human/agent messages.
- Live run status, agent output, command output, and file-change notifications.
- Read-only file tree/viewer plus Monaco diff editor.
- Interactive xterm.js terminal for an active run.
- Stop, timeout, failure, retry-from-clean-checkout, and terminal-state handling.
- Branch, commit, push, and GitHub pull-request creation.
- In-app notifications.
- Audit-friendly timeline derived from persisted domain records and events.
- Basic OpenTelemetry traces, Prometheus metrics, structured logs, and health
  endpoints.

### Deliberately excluded

- Google, GitLab, or Gitea authentication.
- GitLab or Gitea Git-provider implementations.
- More than one production-grade agent adapter.
- Kubernetes, autoscaling, or multi-region operation.
- Deployment orchestration and CI control.
- Agent-to-agent delegation or autonomous agent teams.
- Rich-text documents, BlockNote, Lexical, or collaborative cursors.
- Meilisearch, semantic search, or repository indexing beyond file navigation.
- Mobile authoring; mobile is limited to viewing status and stopping a run.
- Billing, quotas, organization SSO, SCIM, or enterprise policy engines.
- Arbitrary untrusted public repositories or public multi-tenant hosting until
  the isolation controls in the operations guide have been validated.

## Product surfaces

### Workspace list

Shows workspaces, repository, most recent run, and state. It provides create,
open, and archive actions.

### Workspace shell

A desktop-first, resizable layout:

- Left: repository/file tree and workspace navigation.
- Center: thread, code viewer, or diff viewer.
- Bottom or right: collapsible terminal and run logs.
- Right rail: run status, participants, changed files, and PR action.

### Run composer

Lets the user choose an available runtime, enter a task, see the selected
repository/base branch, and start the run. Advanced runtime settings are not
part of the MVP.

### Review and publish

Lists changed files, renders a diff, collects branch/commit/PR information, and
reports each Git operation with actionable errors.

## Actors and permissions

Agents use the same actor model as users but not the same authentication path.
Every message and auditable action has an `actor_id` and `actor_type`.

| Action                      | Owner | Member | Agent                             |
| --------------------------- | ----- | ------ | --------------------------------- |
| View workspace and timeline | Yes   | Yes    | Scoped                            |
| Start/stop a run            | Yes   | Yes    | No                                |
| Send a message              | Yes   | Yes    | During run                        |
| Use run terminal            | Yes   | Yes    | Through runtime                   |
| Commit/push/open PR         | Yes   | Yes    | Only with explicit run capability |
| Manage members/repository   | Yes   | No     | No                                |

Agents receive short-lived, run-scoped credentials and capabilities. They do
not receive a user session.

## Success criteria

The MVP is ready for a private alpha when:

- At least 90% of internal golden-path runs reach a clear terminal state.
- The p95 delay from accepted event to visible UI update is below 1 second
  under the alpha load target.
- Stopping a run prevents new agent commands and terminates its container
  within 10 seconds.
- Refreshing or reconnecting never loses the durable timeline.
- The user cannot access another workspace by changing an ID in an API request
  or NATS/WebSocket subscription.
- A successful run can create a GitHub PR without manual Git commands.
- Failures identify the failed stage and offer a safe next action.

These are initial engineering targets, not promises of production scale.

## Assumptions requiring later validation

- Private alpha users accept a desktop-first interface.
- One repository per workspace is enough to validate the workflow.
- One active run per repository avoids most Git concurrency ambiguity.
- GitHub is sufficient for initial distribution.
- A coding-agent CLI can operate non-interactively and emit output that can be
  normalized without depending on unstable UI scraping.
