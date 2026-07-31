# Design Brief: Local Agent Network

## Problem

People can connect Resources and coding agents on their own computers, but
collaborators cannot reliably tell whether those agents are available or ask
them for help without exposing private paths, credentials, and working-session
state to the server.

## Solution

Runspace presents every user-owned Host Agent as a permissioned collaborator.
Members can see safe presence and capability metadata, request a task or a
session summary, and follow its progress. The owner’s Host Agent chooses a
loopback, direct peer, or encrypted relay route and remains the final authority
for every local operation.

## Experience Principles

1. **Local authority over remote convenience** -- metadata enables discovery,
   but the owner’s PC authorizes and executes every request.
2. **Health over green dots** -- show process, adapter, session, Resource, and
   connection health rather than reducing availability to socket state.
3. **Explicit scope over ambient access** -- every request names its workspace,
   target, capability grant, caller, and deadline.

## Aesthetic Direction

- **Philosophy**: Functionalist developer tool.
- **Tone**: Calm, technical, and accountable.
- **Reference points**: Zed collaborator presence and terminal diagnostics.
- **Anti-references**: Consumer chat presence, opaque “AI is thinking” states,
  and permission settings buried in a global settings page.

## Existing Patterns

- Typography: Inter for UI and JetBrains Mono/Cascadia Code for technical state.
- Colors: Existing dark surfaces, magenta accent, blue/amber/red state tokens.
- Spacing: Compact 6–14 px control rhythm and 4–6 px radii.
- Components: Right context rail, reusable connection modal, agent connection
  form, channel timeline, terminal tabs, Lucide icons.

## Component Inventory

| Component              | Status | Notes                                                     |
| ---------------------- | ------ | --------------------------------------------------------- |
| Agent connection modal | Modify | Local discovery, model, permission mode, owner boundary.  |
| Host presence badge    | New    | Route, lease state, last seen, degraded reason.           |
| Agent health card      | New    | Adapter, auth, process, sessions, context pressure.       |
| Remote request card    | New    | Caller, requested capability, scope, approve/deny.        |
| Session context view   | New    | Owner-approved summary and checkpoint, never raw secrets. |
| Transport diagnostics  | New    | Loopback/direct/relay route and reconnect history.        |

## Key Interactions

- Opening **Connect agent** scans the current PC, reconciles safe metadata, and
  defaults to a ready installation.
- Selecting YOLO requires an explicit danger acknowledgement; the preference is
  written only to the user partition of the Host Agent config.
- A collaborator requests an agent action. Runspace shows the owner, scope, and
  capability. The owner’s local policy auto-allows, prompts, or denies it.
- Presence expires when its signed lease is not renewed. Agent and Resource
  health can independently remain degraded even while the device is online.
- Transport starts loopback on the same device, prefers a verified direct route
  across devices, and falls back to the relay without changing the request.

## Responsive Behavior

The right rail becomes a focus-managed drawer on narrow layouts. Health cards
collapse to the worst state and route; detailed probes move into a modal.

## Accessibility Requirements

State never relies on color alone. Presence changes use polite live regions.
All cards and approval actions are keyboard reachable, danger confirmation is
explicitly labelled, and modal focus returns to its trigger.

## Out of Scope

- Exposing raw local paths, credentials, command lines, or native ACP session
  identifiers to other users or PostgreSQL.
- Unrestricted remote shell access.
- Fully autonomous cross-agent loops without assignments, budgets, and grants.
