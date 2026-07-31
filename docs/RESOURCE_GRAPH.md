# Runspace Resource Graph

Runspace is the coordination and resource layer shared by humans and AI agents.
It does not replace the systems that own code, issues, deployments, logs, or
local agent sessions. It stores share-safe metadata and relationships above
those systems.

```text
GitHub ───────┐
Linear ───────┤
Kubernetes ───┼── Runspace Resource Graph
Datadog ──────┤
Local agents ─┤
Local folders ┘
```

## Core model

The durable graph has eight node kinds:

```text
Resource  Task  Artifact  Action
Discussion  Identity  Policy  Event
```

Edges are typed relationships such as `discussed_in`, `produced_by`, and
`uses`. IDs are namespaced:

```text
resource:repo_123
discussion:thread_123
task:local_session_123
artifact:message_123
```

Provider-specific concepts stay in `type` and `metadata`, which keeps the core
model stable as integrations are added.

## Privacy boundary

The graph may contain:

- Display names, provider type, branch/status summaries, and external IDs.
- Shared task titles and status.
- Explicitly published artifact titles, summaries, and references.
- Relationships, ownership, timestamps, and policy state.

The graph must not contain:

- Host filesystem paths.
- Agent prompts, transcripts, native session IDs, or context windows.
- Terminal input/output.
- Credentials, environment values, or provider tokens.
- Unpublished model output.

Local resources and ACP configuration remain in the signed-in user's Host Agent
configuration. PostgreSQL contains only collaboration metadata that the user
explicitly connects or shares.

## Automatic projections

The gateway projects existing domain operations into the graph:

| Domain operation       | Graph result                                     |
| ---------------------- | ------------------------------------------------ |
| Connect resource       | `Resource` node                                  |
| Create channel thread  | `Discussion` node                                |
| Share local agent chat | `Task` node with `discussed_in` and `uses` edges |
| Share task artifact    | `Artifact` node with a `produced_by` edge        |

Startup migration backfills resources, discussions, shared agent tasks, and
their relationships from existing metadata.

## HTTP API

All endpoints require `X-User-ID` and enforce workspace membership.

```text
GET  /api/v1/workspaces/{workspaceID}/graph/nodes
GET  /api/v1/workspaces/{workspaceID}/graph/nodes/{nodeID}
POST /api/v1/workspaces/{workspaceID}/graph/nodes
POST /api/v1/workspaces/{workspaceID}/graph/edges
```

Node listing supports `kind`, `type`, `q`, `thread_id`, and `limit` query
parameters. Reading one node returns the node plus incoming and outgoing edges,
which gives clients evidence for every relationship they display.

## MCP boundary

Each workspace exposes a stateless JSON-RPC MCP endpoint:

```text
POST /api/v1/workspaces/{workspaceID}/mcp
POST /api/v1/workspaces/{workspaceID}/channels/{channelID}/mcp
```

It supports `initialize`, `ping`, `tools/list`, and `tools/call`. Initial tools:

```text
search_resources
read_context
read_discussion
send_message
list_tasks
create_task
publish_artifact
request_access
```

`create_task` is a coordination operation; it does not start an AI chat.
`request_access` creates a pending `Policy` node for a human decision. Tools
operate only within the workspace encoded in the endpoint URL and the identity
provided by authenticated gateway middleware.

The endpoint follows the Streamable HTTP JSON-RPC message shape without
requiring server-side protocol sessions. Transport negotiation, notifications,
and experimental MCP Tasks can be added without changing the graph service.

## Automatic ACP injection

The Host Agent automatically adds Runspace MCP to every new or resumed ACP
session. It does not modify global Codex, Claude, OpenCode, or Gemini
configuration.

```text
ACP session/new or session/resume
└── mcpServers
    └── Runspace
        └── host-agent mcp-proxy
            └── {gateway}/workspaces/{workspaceID}/mcp
```

ACP guarantees command-based stdio MCP support, so the Host Agent launches
itself in `mcp-proxy` mode as a universal relay. The relay forwards JSON-RPC to
the workspace HTTP endpoint and injects the signed-in Runspace identity.

When the agent chat originates inside a channel, the injected URL includes its
discussion thread:

```text
{gateway}/workspaces/{workspaceID}/mcp?thread_id={threadID}&agent_id={agentID}
```

`read_discussion` reads the real, permission-checked workspace messages for
that thread. It returns the most recent 50 messages by default (up to 200)
without copying transcripts into local agent configuration or duplicating
message bodies in the resource graph.

`send_message` publishes into that discussion as the connected ACP agent. The
tool accepts only the message body: user, workspace, thread, and agent identity
come from the injected connection. The Agent Registry verifies that the agent
belongs to the signed-in user before collaboration persistence accepts it, so
tool arguments cannot spoof another member or agent.

`list_tasks`, `create_task`, `publish_artifact`, access requests, and typed
searches then default to that thread. Agents can still search workspace-wide
Resources. An explicit `thread_id` tool argument overrides the session default.

The MCP configuration is derived at session start from the resource binding in
the logged-in user's local `runspace-local.json`. If a legacy binding has no
gateway or workspace metadata, the session starts without injection rather than
guessing or editing provider configuration.

## Product mapping

```text
ACP controls agents.
MCP lets agents use Runspace.
Runspace connects the team's resources, knowledge, work, and actions.
```

Channels are `Discussion` context. Their navigation tree shows related shared
work rather than owning separate feature silos. Agent chats remain private
execution context until their owner shares the task or publishes an artifact.

## Resource Center and provenance

The Resource Center is the workspace-wide view over the permission-checked
resource graph. It organizes connected resources, published artifacts, tasks,
actions, and discussions without moving provider-owned data into Runspace.
Every item retains `owner_id` and is presented as `Shared by {member}`.

A local resource remains on its owner's Host Agent. Sharing publishes only its
workspace representation and governed capabilities. Workspace members—and ACP
agents acting through a member's Runspace identity—can search and read that
shared representation through MCP. They do not receive direct access to
another member's local filesystem, credentials, or private agent transcript.
