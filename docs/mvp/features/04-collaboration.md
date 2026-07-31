# Feature 4: Collaboration Timeline

## Outcome

Humans and agents share a durable, comprehensible thread showing messages,
decisions, run activity, and important Git milestones.

## Build

- Channels, threads, actors, messages, and notification records.
- Plain-text/Markdown composer with safe rendering.
- Timeline read model joining human messages, agent messages, run milestones,
  and Git milestones.
- Cursor pagination and React Virtuoso rendering.
- Message pending/sent/failed states with idempotent resend.
- `@agent` display semantics and notifications for run input requests,
  completion, failure, and PR creation.
- Presence limited to connected/disconnected indicators; it is ephemeral.

## Product rules

- One general channel is enough for the MVP; each task becomes a thread.
- Agents appear with stable workspace identity plus per-run context.
- System events are visually quieter than conversation but remain inspectable.
- Raw logs do not flood the timeline; they live in the log panel.
- Editing/deleting messages is deferred to preserve a simple audit model.
- Mentions notify but do not trigger another agent automatically.

## Acceptance scenarios

1. Two authorized browsers see a new human message promptly.
2. An agent response and its underlying run are clearly attributable.
3. Refresh/reconnect produces the same ordered durable timeline.
4. Duplicate delivery does not create duplicate messages.
5. Long threads remain responsive and paginate without jumps.
6. Markdown cannot execute raw HTML, scripts, or unsafe links.
7. A non-member cannot subscribe to or fetch the thread.

## Not included

Collaborative rich text, typing indicators, read receipts, message editing,
files uploaded through chat, emoji reactions, or agent-to-agent mentions.
