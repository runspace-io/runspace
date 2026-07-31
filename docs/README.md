# Documentation

The documentation is organized around the MVP's single golden path. Read it in
this order:

1. [Product specification](mvp/PRODUCT_SPEC.md) — users, scope, success, and
   release boundary.
2. [Architecture](mvp/ARCHITECTURE.md) — services, responsibilities, and
   important decisions.
3. [Event model](mvp/EVENT_MODEL.md) — envelope, subjects, delivery rules, and
   initial catalog.
4. [Data, API, and realtime](mvp/DATA_API_REALTIME.md) — core records and
   external contracts.
5. [Operations](mvp/OPERATIONS.md) — security, isolation, observability, and
   test strategy.
6. [Feature guides](mvp/features/README.md) — the build sequence and definition
   of done for every vertical slice.

The proposed next architecture slice is
[Persistent Agent Collaboration](PERSISTENT_AGENT_COLLABORATION.md), covering
durable ACP collaborators, supervised sessions, context rollover, orchestration,
and safe multi-agent repository access.

The normalized collaboration layer is documented in
[Resource Graph](RESOURCE_GRAPH.md), including graph primitives, privacy
boundaries, automatic projections, APIs, and the workspace MCP endpoint.

GitHub, PostgreSQL, and DigitalOcean integrations are covered in
[Resource adapters](RESOURCE_ADAPTERS.md), including native and local placement,
lazy availability, credential boundaries, and MCP queries. The server-hosted
execution model is specified in
[Native Resource plugins](NATIVE_RESOURCE_PLUGINS.md).

Agent-authored structured UI and the safe D3/action boundary are documented in
[Interactive artifacts](INTERACTIVE_ARTIFACTS.md).

The executable checklist is in
[`.design/mvp-engineering-workspace/TASKS.md`](../.design/mvp-engineering-workspace/TASKS.md).

## Decision policy

The MVP optimizes for a cohesive workflow, not a large capability matrix.
Interfaces are created where replacement is expected—agent runtimes and Git
providers—but only one production implementation is required before launch.

An item belongs in the MVP only if it is necessary to complete or safely operate
the golden path:

> GitHub sign-in → workspace → repository → agent run → live inspection →
> review changes → pull request.
