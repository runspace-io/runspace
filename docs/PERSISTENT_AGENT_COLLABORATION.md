# Persistent Agent Collaboration

Status: Implemented foundation; transport and native pause are proposed  
Audience: Product, frontend, platform, and agent-runtime engineers

> **Local authority amendment:** Resources and ACP runtimes are user-owned
> Host Agent data. PostgreSQL stores collaboration metadata only. Absolute
> paths, commands, credentials, model/permission preferences, process details,
> and native ACP session IDs live in the user-partitioned, exportable Host Agent
> config. Cross-device requests use the transport design in
> [HOST_AGENT_TRANSPORT.md](HOST_AGENT_TRANSPORT.md).

## Implemented Agent Chats foundation

Runspace now presents local agent work as private **Agent Chats**, with a task
control plane underneath:

- **My local chats** in the Agent sidebar lists the signed-in user's chats from
  this device and workspace.
- There is no separate Tasks tab. Opening a chat temporarily replaces the
  channel timeline with its private working surface; closing it returns to the
  channel.
- Sharing a chat creates a child node beneath that channel in the left
  navigation tree. Private chats remain in the user sidebar and never appear in
  the channel tree.
- Every chat is titled from its work objective (for example,
  **Fix invisible terminal input**), never from an ACP implementation label such
  as `ACP session 1`.
- Instructions go directly to the user-owned Host Agent and persist in that
  user's exportable local config.
- The shared channel receives only fixed-vocabulary activity projections:
  `started`, `completed`, `failed`, `cancelled`, and `waiting_approval`.
- Agent output crosses into the shared channel only when a user explicitly
  selects **Share**.
- Task grants are safe PostgreSQL metadata. An owner can grant a workspace
  member `viewer`, `contributor`, `operator`, or `approver`; the gateway derives
  capabilities from the role and ignores client-supplied permission lists.

The current Host Agent reports `pause_support: cancel-only`. Native pause and
process-suspend support remain negotiated capabilities rather than assumed UI
features.

### Control-plane vocabulary

```text
CreateTask()
PauseTask()
ResumeTask()
CancelTask()
GetTask()
ListTasks()
ShareArtifact()
SubscribeEvents()
RequestCapability()
RespondCapability()
```

`CreateTask` does not create a public chat thread. It creates a supervised unit
of work bound to an agent, Resource, owner device, and permission policy.
Follow-up instructions are private task inputs. `ShareArtifact` is the explicit
bridge to collaboration.

### Implemented HTTP boundaries

```text
Host Agent (user-owned local data)
GET  /v1/agent-chats?workspace_id={workspaceId}
GET  /v1/agents/{agentId}/session
POST /v1/agents/{agentId}/prompt
POST /v1/agents/{agentId}/session/cancel

Gateway (safe collaboration metadata)
POST /threads/{threadId}/agent-activity
POST /threads/{threadId}/agent-messages
PUT  /agent-tasks/{taskId}
GET  /workspaces/{workspaceId}/agent-tasks?thread_id={threadId}
POST /agent-tasks/{taskId}/input
POST /agent-tasks/{taskId}/cancel
POST /agent-tasks/{taskId}/artifacts
GET  /agent-tasks/{taskId}/grants
PUT  /agent-tasks/{taskId}/grants/{principalId}
```

The `agent-messages` endpoint is used only for an explicit shared artifact,
never as an automatic sink for local ACP output.

## Summary

Runspace should treat coding agents as persistent channel collaborators, not as
commands that disappear when a run ends. A user connects Codex, Claude Code,
OpenCode, or another ACP-compatible agent once, attaches it to one or more
channels and repositories, and can continue working with the same collaborator
across prompts, runs, reconnects, and context-window rollovers.

The design combines three ideas:

- ACP is the interoperability boundary between Runspace and external agents.
- Zed's external-agent model informs installation, native authentication, and
  resumable sessions.
- OpenHands' runtime model informs workspace isolation and supervised execution.

Runspace owns shared channel history, collaboration metadata, grants,
orchestration, and safe checkpoints. The user-owned Host Agent owns private
task instructions and output, Resource resolution, model selection, provider
authentication, tools, permission mode, native sessions, process lifecycle,
and detailed recovery state.

## Problem

Today the gateway starts a new ACP process and session for each run. Follow-up
input works only while that run is active, and the client is closed when its
notification stream ends. There is no stable agent identity, session restore,
channel membership, installation registry, capability record, or recovery
after an agent exhausts its context.

This makes agents feel like disposable jobs rather than teammates. It also
forces users to repeatedly configure commands and loses continuity between the
channel conversation, repository state, and the agent's internal session.

## Experience Principles

1. **Private chats, supervised execution** — connect an agent once and resume
   local conversations without turning the shared channel composer into a model
   prompt box. Task-control semantics remain underneath the chat UI.
2. **Useful presence over noise** — agents work in private threads and publish
   concise results into shared conversation.
3. **Recoverable over magical** — session, permission, context, and failure
   state must be visible and restartable.

## User Experience

### Connect an agent

The channel's right sidebar contains an **Agents** section alongside
repositories and other channel configuration. It shows connected agents,
presence, current mode, active repository, and session health.

**Add agent** opens the shared connection modal used by repository and agent
configuration. The flow:

1. Select an installed provider from the ACP registry or choose **Custom ACP
   command**.
2. If required, install or authenticate using the provider's native flow.
3. Choose a display name and default repository access.
4. Review executable, working directory, environment access, network access,
   and approval policy.
5. Connect in `mention-only` mode.

Codex, Claude Code, and OpenCode are the first verified integrations. Any ACP
v1-compatible command may be added as an unsupported custom integration.

### Collaborate around an Agent Chat

Each connected agent has:

- a stable identity and avatar;
- a presence state: `offline`, `starting`, `ready`, `working`, `waiting`,
  `degraded`, or `stopped`;
- a channel mode: `observer`, `mention-only`, `active`, or `lead`;
- a private task log on the owner Host Agent containing instructions and
  detailed agent output;
- a shared-channel trail of assignments, questions, decisions, checkpoints,
  and results.

Channel messages do not wake or prompt an agent. Work starts from an explicit
task action. A later orchestrator may propose or queue work from channel
events, but it must still create an auditable task under user policy and cannot
silently copy the channel into a provider session.

Agent cards expose **Open task**, **Assign**, **Pause** when supported,
**Restart session**, **Access**, and **Disconnect**. Process logs and raw ACP
traffic are available in diagnostics, not mixed into normal chat.

### Team collaboration

Members collaborate through scoped authority over a task, never blanket access
to the owner's computer. Initial roles map to server-derived permissions:

| Role        | Permissions                     |
| ----------- | ------------------------------- |
| Viewer      | `task.view`, `artifact.view`    |
| Contributor | Viewer plus `task.contribute`   |
| Operator    | Contributor plus `task.control` |
| Approver    | Viewer plus `task.approve`      |

Resource read/write, terminal access, network access, and elevation are
separate capabilities. An operator does not implicitly receive any of them.
Cross-device task inputs travel through the authorized Host Agent RPC
transport and are not stored as raw PostgreSQL prompt rows.

### Work with repositories

An agent membership grants access to an explicit set of workspace repository
connections. Git is optional for adding and browsing a folder, but it changes
the safe concurrency strategy:

- Git repositories use an isolated worktree and agent branch for every editing
  agent. The base checkout remains the user's view.
- Plain folders use a shared mirror with one writer lease per path, collision
  warnings, and visible ownership. A conflicting write pauses for user review.
- Read-only research does not require an editing lease.

The agent's current repository, worktree/branch, changed files, terminal
sessions, and pending approvals are visible from its card and private thread.

### Session continuity

Closing the browser does not stop a connected agent. A gateway or host-agent
restart restores durable memberships and attempts to resume native ACP
sessions. If an integration cannot restore a session, Runspace starts a
successor using the latest durable checkpoint.

Before context exhaustion, on important milestones, and before a controlled
restart, Runspace stores a structured checkpoint containing:

- current goal and user intent;
- accepted decisions and constraints;
- completed work and validation;
- active tasks and owners;
- unresolved questions and blockers;
- repository, branch, worktree, and relevant file references;
- last consumed channel-event cursor;
- predecessor session and checkpoint lineage.

The channel timeline is the source of truth. A provider session is a replaceable
cognitive worker, not the only copy of project memory.

## Architecture

```text
Browser
  │ REST + realtime events
  ▼
Gateway / collaboration service
  ├── durable channels, messages, metadata, grants, safe checkpoints
  ├── deterministic orchestration policy
  └── supervisor commands
          │
          ▼
User-owned Host Agent supervisor
  ├── installation and capability registry
  ├── process lifecycle and health checks
  ├── ACP v1 session create/load/prompt/cancel
  └── output normalization
          │
          └── local Resource → approved folder/worktree/terminal
```

The gateway owns collaborative desired-state metadata. The Host Agent owns and
reconciles executable desired state with live processes. ACP processes always
run near the user-owned filesystem. Process ownership must not depend on a
browser connection; loopback, direct peer, and relay routes share one
capability-scoped RPC envelope.

### Core records

| Record                | Purpose                                                                            |
| --------------------- | ---------------------------------------------------------------------------------- |
| `agent_definitions`   | Provider identity, registry source, executable template, version, and trust level. |
| `agent_installations` | Safe owner, provider, display, capabilities, placement, and last-seen metadata.    |
| Local config process  | Executable, auth reference, model/policy, process handle, and restart generation.  |
| Local config session  | Native ACP session ID, Resource path resolution, resume state, and local lineage.  |
| `agent_memberships`   | Agent installation attached to a channel with mode, repository grants, and policy. |
| `agent_checkpoints`   | Structured recovery context, event cursor, token estimate, and predecessor link.   |
| `agent_assignments`   | Durable unit of work, owner, status, thread, repository, and optional run link.    |

External session IDs, process IDs, and filesystem paths never act as public
identifiers. API resources use Runspace IDs and resolve sensitive details on
the trusted side.

### ACP adapter

Extend the current ACP boundary from per-run transport to a capability-aware
session client:

```go
type ACPClient interface {
    Initialize(ctx context.Context) (Capabilities, error)
    NewSession(ctx context.Context, req NewSessionRequest) (SessionRef, error)
    LoadSession(ctx context.Context, ref SessionRef) error
    Prompt(ctx context.Context, sessionID string, prompt Prompt) error
    Cancel(ctx context.Context, sessionID string) error
    SetMode(ctx context.Context, sessionID string, mode SessionMode) error
    Notifications() <-chan ACPNotification
    Close() error
}
```

Initialization negotiates ACP protocol version and capabilities. Unsupported
optional methods degrade cleanly: native load falls back to checkpoint
recovery, and unsupported mode changes remain Runspace-side policy.

Agent output is normalized into conversation messages, tool/command lifecycle
events, file changes, permission requests, plans, usage, and raw diagnostics.
Unknown notifications are retained as bounded diagnostics and cannot crash the
session.

### Supervisor and reconciliation

The supervisor implements a desired-state loop:

1. Load enabled agent memberships.
2. Resolve the installation and authorized runtime placement.
3. Start or reconnect to the ACP process.
4. Negotiate capabilities and resume the newest viable session.
5. Replay relevant durable events after the session's cursor.
6. Mark the collaborator ready, degraded, or waiting for user action.

Processes use exponential restart backoff and a bounded crash budget. Repeated
failure moves the membership to `degraded` and requires user action. Restarts
never duplicate a completed assignment because assignment and event IDs are
idempotency keys.

### Orchestrator

The first orchestrator is deterministic. It subscribes to durable channel,
assignment, repository, session, and approval events and decides which
collaborator should wake.

Rules:

- mentions and direct assignments always route to the named agent;
- `observer` never wakes automatically;
- `mention-only` wakes only for a mention, assignment, or requested follow-up;
- `active` may wake for relevant repository and assignment events;
- `lead` additionally decomposes work and proposes delegation;
- agent-authored messages cannot recursively wake another agent unless a human
  approved the delegation or an existing assignment targets that agent;
- duplicate event IDs and already-consumed cursors are ignored;
- per-channel turn, time, token, and cost budgets stop runaway work.

A configurable small routing model may later rank relevance, summarize noisy
events, and draft checkpoints. It does not own permissions, process lifecycle,
budget enforcement, or durable state transitions.

### Context rollover

Rollover begins when the provider reports context pressure, a configured token
threshold is reached, the session becomes incoherent, or a user selects
**Restart session**.

Runspace asks the current session for a checkpoint while it is still usable,
validates and persists the structured result, then either loads the native
session or starts a successor session. The successor receives the checkpoint,
current assignment, relevant recent messages, repository status, and event
cursor. Both sessions remain linked for audit and debugging.

If checkpoint generation fails, Runspace builds a deterministic fallback from
durable assignments, decisions, recent messages, repository state, and tool
results. The rollover is shown as a quiet system event.

## Public Interfaces

Initial REST resources:

```text
GET    /api/v1/agent-definitions
POST   /api/v1/workspaces/{workspaceId}/agent-installations
GET    /api/v1/workspaces/{workspaceId}/agent-installations
POST   /api/v1/channels/{channelId}/agents
PATCH  /api/v1/channels/{channelId}/agents/{membershipId}
DELETE /api/v1/channels/{channelId}/agents/{membershipId}
POST   /api/v1/channels/{channelId}/agents/{membershipId}/assignments
POST   /api/v1/channels/{channelId}/agents/{membershipId}/pause
POST   /api/v1/channels/{channelId}/agents/{membershipId}/resume
POST   /api/v1/channels/{channelId}/agents/{membershipId}/rollover
GET    /api/v1/agent-sessions/{sessionId}
GET    /api/v1/agent-sessions/{sessionId}/diagnostics
```

New durable events:

```text
agent.installation.changed
agent.membership.changed
agent.process.state_changed
agent.session.started
agent.session.ready
agent.session.waiting
agent.session.degraded
agent.session.rolled_over
agent.assignment.created
agent.assignment.started
agent.assignment.completed
agent.assignment.failed
agent.checkpoint.created
agent.approval.requested
agent.approval.resolved
```

Realtime clients receive these through the existing workspace subscription.
High-volume ACP output and terminal bytes remain separate bounded streams.

## Security and Permissions

- Registry entries are metadata, never implicitly trusted executables.
- Installation shows the exact command/package, source, version, and requested
  capabilities before execution.
- Provider authentication stays in the agent's native flow or host credential
  store; Runspace stores references and redacted status, not plaintext secrets.
- Repository, terminal, network, environment, and host-user/admin permissions
  are explicit per installation and may be narrowed per channel membership.
- Host execution defaults to the signed-in OS user. Elevated execution requires
  an explicit launch choice and visible persistent badge.
- Every tool action is attributed to agent, session, assignment, repository,
  and approving actor.
- Disconnecting an agent revokes future access, stops its managed processes,
  releases leases, and preserves its audit history.

## UI Components

Extend the existing workspace sidebar, modal, dialog, channel timeline, and
repository-tool patterns. Keep the current dark, compact developer-tool visual
language using Inter, JetBrains Mono, existing CSS variables, Lucide icons, and
keyboard-first interactions.

| Component               | Status | Responsibility                                                   |
| ----------------------- | ------ | ---------------------------------------------------------------- |
| Channel agents section  | Modify | Connected agents, presence, mode, repository, and quick actions. |
| Connection dialog       | Modify | Reusable repository/agent connection shell with step content.    |
| Agent provider picker   | New    | Registry search, installed state, custom ACP option.             |
| Agent connection form   | Modify | Installation, auth, runtime placement, grants, and policy.       |
| Agent collaborator card | New    | Health, assignment, worktree, costs, and actions.                |
| Agent Chats catalog     | Done   | User-local chats, titles, status, and explicit channel sharing.  |
| Channel chat child      | Done   | Shared chat reference nested beneath its channel in navigation.  |
| Agent working surface   | Done   | Private conversation and normalized activity.                    |
| Approval card           | New    | Scoped permission request with allow-once/deny/policy actions.   |
| Session health popover  | New    | Process, ACP, context, checkpoint, and restart diagnostics.      |

On narrow layouts, the right sidebar becomes an accessible drawer. Dialogs trap
focus, restore focus to their trigger, support keyboard completion, announce
process-state changes, and do not rely on color alone.

## Delivery Plan

### 1. Durable collaborator foundation

- Add definitions, installations, memberships, sessions, checkpoints, and
  assignments.
- Separate durable agent identity from the existing run actor.
- Add sidebar membership management and mention-only collaboration.
- Keep existing per-run ACP behavior as a compatibility adapter.

### 2. Supervised persistent ACP

- Add capability negotiation, process health, reconciliation, native session
  load where supported, and checkpoint fallback.
- Verify Codex, Claude Code, and OpenCode adapters plus custom ACP commands.
- Restore collaborators after gateway, supervisor, and browser restarts.

### 3. Safe concurrent work

- Provision per-agent Git worktrees and branches.
- Add plain-folder writer leases and collision review.
- Surface file changes, terminal sessions, worktree, and branch per agent.

### 4. Orchestration and rollover

- Add modes, assignments, deterministic wake rules, budgets, loop prevention,
  checkpoints, and successor-session lineage.
- Add one active lead per channel and human-approved delegation.
- Introduce model-assisted routing only behind an optional feature flag.

## Acceptance Scenarios

1. Connect Codex, Claude Code, or OpenCode from the sidebar without manually
   recreating its command for each task.
2. Mention a connected agent, refresh or close the browser, return later, and
   continue the same conversation and repository work.
3. Restart the gateway or supervisor and recover the membership, assignment,
   session, event cursor, and working tree without duplicate output.
4. Exhaust or restart an agent session and continue through a checkpoint-backed
   successor with visible lineage.
5. Run two editing agents against a Git repository without either modifying the
   other's worktree.
6. Detect conflicting writes in a plain folder and pause before overwriting.
7. Prevent two active agents from creating an autonomous reply loop.
8. Deny an ungranted repository, terminal, network, or elevated-host action and
   preserve an auditable approval record.
9. Handle malformed ACP output, process crashes, missing native resume, and
   unsupported capabilities without losing channel history.
10. Navigate, configure, pause, and diagnose agents using keyboard and screen
    reader controls on desktop and narrow layouts.

## Out of Scope

- Implementing a new model provider or replacing the internal reasoning of
  Codex, Claude Code, or OpenCode.
- A public executable marketplace with automatic trust.
- Fully autonomous cross-agent conversations without human policy.
- Automatic merge, push, pull request, deployment, or destructive host action.
- Distributed worktree synchronization across multiple physical hosts in the
  first release.

## Decisions and Defaults

- Conversation topology: private agent working threads plus concise shared
  channel updates.
- New membership mode: `mention-only`.
- Concurrent Git editing: isolated worktree and branch per agent.
- Plain-folder editing: shared mirror with writer leases and collision review.
- Runtime placement: hybrid container/host supervisor, chosen by repository
  location and explicit grants.
- Recovery: native ACP resume when available, checkpoint-backed successor
  otherwise.
- Orchestration: deterministic policy first; optional small routing model later.
- Authentication: provider-native; no plaintext provider credentials in
  Runspace.
- First verified agents: Codex, Claude Code, and OpenCode.
