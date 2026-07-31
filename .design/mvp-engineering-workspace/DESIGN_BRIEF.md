# Design Brief: MVP Agent Engineering Workspace

## Problem

Developers can ask coding agents to change a repository, but the experience is
fragmented across chat, terminals, local clones, logs, and Git hosting. They
struggle to see what an agent is doing, intervene safely, understand the real
repository changes, and hand the result into normal human review.

## Solution

Provide one desktop engineering workspace where a developer starts an isolated
agent run against a GitHub repository, follows its work through a shared
timeline and terminal, inspects real files and diffs, and deliberately publishes
the result as a pull request. Agents appear as accountable collaborators with
identity, permissions, status, and history.

## Experience Principles

1. **Control over autonomy** — status, permissions, limits, and stop are always
   visible when an agent is active.
2. **Repository truth over chat theater** — files, diffs, commands, commits, and
   PR state carry more visual weight than conversational flourish.
3. **Progressive complexity** — the common path is calm and direct; raw logs,
   event details, and recovery actions are available without dominating it.

## Aesthetic Direction

- **Philosophy**: calm technical instrument; dense, precise, and quietly
  opinionated.
- **Tone**: trustworthy, focused, collaborative, and never anthropomorphically
  cute.
- **Reference points**: VS Code panel behavior, GitHub review clarity, Linear's
  restrained density, and modern terminal tools such as OpenCode.
- **Anti-references**: consumer messenger clones, oversized AI gradients,
  dashboard card grids, novelty bot avatars, and interfaces that hide the actual
  Git state behind summaries.

## Existing Patterns

The repository was empty when this brief was written, so there are no inherited
patterns.

- Typography: new; use a highly legible sans UI face and a dedicated monospace
  face for code/terminal.
- Colors: new; neutral dark and light surfaces with semantic status colors.
- Spacing: new; compact 4px-based scale.
- Components: new; begin with shadcn/ui and Radix primitives.

## Component Inventory

| Component                     | Status | Notes                                  |
| ----------------------------- | ------ | -------------------------------------- |
| App shell                     | New    | Responsive resizable desktop panels    |
| Workspace switcher            | New    | Command-palette compatible             |
| Repository tree               | New    | Lazy expanded; virtualize if measured  |
| Thread timeline               | New    | ChatScope; virtualize if measured      |
| Message composer              | New    | Plain text/Markdown in MVP             |
| Agent/run badge               | New    | Actor, runtime, status, and duration   |
| Run controls                  | New    | Start, stop, retry, and failure action |
| Activity/command card         | New    | Compact summary, expandable details    |
| Code viewer                   | New    | Monaco read-only                       |
| Diff viewer                   | New    | Monaco Diff Editor                     |
| Terminal panel                | New    | xterm.js                               |
| Changed-files list            | New    | Status, counts, selected state         |
| Publish dialog                | New    | Branch, commit, PR fields and progress |
| Notification center           | New    | Minimal in-app list plus sonner        |
| Dialog/button/menu primitives | New    | Reuse shadcn/ui                        |

## Key Interactions

- Creating a workspace connects a GitHub repository and lands in its general
  channel.
- Submitting a task creates a thread and queued run immediately; the run status
  advances without blocking the composer.
- New durable events append to the timeline while raw output streams in a
  dedicated log/terminal surface.
- Selecting a changed file opens its diff without losing thread or run context.
- Stopping a run requires a clear action, updates immediately to `stopping`, and
  resolves to a durable terminal state.
- Publishing opens a focused dialog and displays the branch → commit → push →
  PR stages, including stage-specific recovery.
- Refresh and reconnect restore the same selection, timeline, and current run
  from durable state.

## Responsive Behavior

- Desktop (`>= 1024px`) is the authoring target and supports resizable panels.
- Tablet collapses the left and right rails into drawers and uses tabs for
  thread/code/diff.
- Mobile is view-and-control only: timeline, run status, notifications, and stop
  remain available; terminal, code review, and publish instruct the user to use
  a larger screen.
- Panel size preferences persist per user, with a reset action.

## Accessibility Requirements

- Meet WCAG 2.2 AA contrast for text, focus, controls, and semantic statuses.
- Every action and panel is keyboard reachable; resizing has keyboard controls.
- Use a visible focus ring and restore focus after dialogs/drawers close.
- Announce run state changes and input requests without announcing every output
  chunk.
- Terminal and Monaco retain their native accessibility modes with explicit
  labels and escape routes.
- Color never carries run, file, or Git status alone.
- Respect reduced motion and avoid auto-scrolling when the user has moved away
  from the live edge.

## Out of Scope

- A general-purpose browser IDE or direct file-editing replacement.
- Mobile code editing and terminal use.
- Rich-text collaborative documents.
- Multiple simultaneous agents in one run.
- Deployment, CI, merge, billing, and enterprise administration.
- GitLab/Gitea UI, even though provider interfaces anticipate them.
- Multiple production-ready runtime adapters.
