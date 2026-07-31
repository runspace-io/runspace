# Native Resource plugins

Native plugins are Runspace-owned integrations that normalize external systems
into Resource Graph nodes and governed capabilities. Unlike personal Host Agent
Resources, they remain available when the connection owner is offline.

## Connection model

```text
Plugin manifest
  -> Workspace connection
  -> Encrypted credential
  -> Resource Graph node
  -> Lazy availability/query
  -> UI and MCP
```

A connection has a plugin ID, workspace, owner, placement, authentication
method, access mode, safe configuration, encrypted credential, and advertised
capabilities. Plain credentials never appear in connection responses or graph
metadata.

## Built-in plugins

- GitHub uses its REST API with a personal access token.
- DigitalOcean uses its v2 API with an API token.
- PostgreSQL uses a connection URL and a fixed `information_schema` query.

The current slice intentionally exposes query capabilities only. The stored
access mode prepares the model for separately governed mutation capabilities;
it is not treated as arbitrary HTTP, SQL, CLI, or shell access.

## Placement

`runspace` executes in the gateway's trusted built-in plugin runtime.
`connector` is reserved for the team-operated connector transport needed by
private networks and on-premise services. `host` remains the personal Host Agent
fallback. The Resource Graph routes capabilities by placement while keeping one
MCP contract.

## Security boundary

- Built-in plugins use fixed provider origins and fixed query implementations.
- Provider bodies are capped at 1 MiB and decoded as structured JSON.
- PostgreSQL cannot receive arbitrary SQL.
- Availability and provider data load only on browse or query.
- Production must supply `CHANNEL_SECRET_KEY`; the development fallback is not
  suitable for deployed environments.
- Future third-party plugins require signed manifests, isolated workers,
  explicit egress policy, per-capability grants, and an audited
  plan/approve/invoke action lifecycle.
