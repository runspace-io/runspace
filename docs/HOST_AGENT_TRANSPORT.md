# Host Agent RPC, Presence, and Transport

Status: Accepted architecture, transport contracts started

## Boundary

The Host Agent/app is the authority for local Resources, ACP executables,
credentials, model and permission preferences, terminals, process state, and
native session IDs. These live in the versioned, user-partitioned
`runspace-local.json`, which is explicitly exportable and importable.

PostgreSQL contains collaboration metadata only: opaque IDs, owner IDs,
workspace/channel attachment, display names, capabilities, grants, desired
collaboration mode, safe health state, and last-seen timestamps. Paths,
commands, credentials, model/YOLO settings, and native session IDs are
prohibited fields.

## Topology

```text
caller → gateway authorization → RPC envelope
                              ├─ same PC: loopback Host Agent
                              ├─ reachable peer: encrypted direct stream
                              └─ fallback: outbound Host Agent ↔ relay stream
                                                       │
owner Host Agent policy → Resource / terminal / persistent ACP session
```

Every Host Agent maintains an outbound relay connection, so inbound firewall
rules are unnecessary. Direct connectivity is an optimization. The first
production P2P adapter should use go-libp2p because it supplies encrypted
streams, peer identity, AutoNAT, hole punching, and Circuit Relay v2 behind one
transport abstraction. The relay adapter remains mandatory for deterministic
reachability.

## RPC envelope

All routes carry the same `hosttransport.Envelope`. Required fields are request
ID, method, authenticated caller and owner user IDs, workspace, opaque target,
capability, grant ID, idempotency key, and deadline. Payloads may contain
logical paths relative to a granted Resource, never absolute host paths.

Initial methods:

- `task.create`
- `task.input`
- `task.get`
- `task.list`
- `task.cancel`
- `task.pause`
- `task.resume`
- `task.share_artifact`
- `task.subscribe_events`
- `capability.request`
- `capability.respond`
- `resource.tree`
- `resource.read`
- `terminal.open`
- `host.health`

Session summaries are owner-approved projections: goal, decisions, current
task, changed files, blockers, context pressure, and checkpoint lineage. They
are not raw context dumps.

The browser-facing task surface never calls cross-device methods directly. It
calls the gateway, which verifies the task grant and wraps the request in a
signed `hosttransport.Envelope`. Same-device owner work may select the loopback
adapter without passing private content through PostgreSQL.

## Authorization

1. Gateway proves the caller is a workspace member and resolves the target
   owner from metadata.
2. Gateway issues or checks a narrow capability grant.
3. The envelope is signed, bounded by a deadline, and replay-protected with its
   idempotency key.
4. The owner Host Agent independently checks the owner user partition, target,
   grant, local policy, permission mode, and requested method.
5. Cross-user terminal or write access always requires a specific grant and may
   require an owner prompt. YOLO affects the owner’s agent actions; it does not
   bypass cross-user authorization.

The relay routes opaque encrypted frames and enforces connection quotas. It
cannot decrypt local RPC payloads.

## Presence and health

Presence uses a renewable lease, not a permanent database flag. A Host Agent
sends a signed heartbeat every 15 seconds with a 45-second lease. Missing the
lease means `offline`; repeated probe failure while connected means `degraded`.

Health dimensions remain separate:

| Dimension | Example states                                          |
| --------- | ------------------------------------------------------- |
| Device    | online, degraded, offline                               |
| Route     | loopback, direct, relay                                 |
| Agent     | ready, busy, waiting_approval, adapter_missing, crashed |
| Session   | active, resumable, stalled, context_pressure            |
| Resource  | reachable, missing, revoked                             |

PostgreSQL stores the latest safe projection and timestamp for discovery.
Detailed errors and process diagnostics stay local; metadata exposes stable
error codes only.

## Routing and failure

The selector prefers loopback, then a verified direct stream, then relay.
Calls may retry on another route only when the method is idempotent or carries
an idempotency key. A route change never changes authorization.

The gateway queues a bounded request only when a capability explicitly permits
offline delivery. Interactive prompts and terminals fail fast when the owner is
offline. Session-summary requests may be satisfied by the latest owner-approved
checkpoint, clearly marked with its age.

## Delivery slices

1. Contract and local boundary: versioned per-user config, safe metadata,
   transport-neutral envelopes, health vocabulary.
2. Relay presence: outbound Host Agent WebSocket, leases, signed requests,
   request/response correlation, quotas.
3. Persistent local ACP: resume/create sessions, prompt and summary RPC,
   local model/permission translation, process supervision.
4. Direct transport: go-libp2p identity, AutoNAT, relay reservation, DCUtR
   upgrade, route telemetry.
5. Collaboration policy: capability grants, owner approval inbox, offline
   checkpoint queries, agent-to-agent budgets and loop prevention.
