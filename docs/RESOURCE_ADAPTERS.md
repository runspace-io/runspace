# Resource adapters

Runspace supports two Resource placements:

- **Runspace plugins** execute as built-in provider integrations and stay
  available when the person who connected them is offline.
- **Local connectors** expose bounded capabilities from a user's computer for
  local files, CLIs, private networks, and coding agents.

## First supported integrations

| Plugin       | Native authentication | Local fallback         | Capabilities                          |
| ------------ | --------------------- | ---------------------- | ------------------------------------- |
| GitHub       | Personal access token | `gh` login             | Search repositories and pull requests |
| PostgreSQL   | Connection URL        | `psql` service profile | Explore non-system schemas and tables |
| DigitalOcean | API token             | `doctl` context        | List/search Apps and Droplets         |

Native credentials are encrypted with the configured Runspace secret key.
Only ciphertext is persisted in `resource_connections`; graph nodes contain a
connection ID, placement, owner, access mode, and capability metadata.
Credentials are never returned from HTTP or MCP APIs.

All initial capabilities are read-only queries. PostgreSQL uses a fixed
`information_schema` statement and cannot receive arbitrary SQL. Native provider
responses and local command output are reduced to allowlisted structured fields.

## Ownership and storage

- Native bindings belong to a workspace and remain available independently of
  the connection owner's device.
- Native credentials are encrypted at rest in the server-side connection vault.
- Local bindings live in the signed-in user's Host Agent config JSON.
- Local CLI tokens, database passwords, and provider config remain in their
  native stores.
- The Resource Graph lets permitted members and agents discover either
  placement through one normalized contract.
- Native queries route to built-in plugins; local queries route to the Resource
  owner's Host Agent.

## Lazy availability

Runspace does not continuously poll providers or index their contents.

1. The Resource Center lists graph metadata without contacting the provider.
2. Opening a native Resource performs a lightweight provider health request.
   Opening a local Resource asks its owner Host Agent.
3. Availability is cached for 15 seconds.
4. Provider contents load only after a user or permitted MCP client invokes
   `query_resource`.

This keeps idle CPU, memory, network usage, and provider API consumption low.
An unavailable provider, owner machine, or CLI is shown as Resource
availability and is not treated as deletion.

## MCP contract

ACP coding agents receive the workspace/channel MCP connection and can use:

- `search_resources` to find shared graph metadata.
- `read_context` to understand relationships and provenance.
- `query_resource` with a graph node ID, advertised capability, query text, and
  result limit.
- `request_access` when a capability has not been granted.

MCP never accepts a raw shell command, credential, or arbitrary SQL. Future
mutation capabilities must use a separate plan/approval/invoke lifecycle.
