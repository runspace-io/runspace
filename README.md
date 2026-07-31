# Runspace

[![License: MIT](https://img.shields.io/badge/license-MIT-green)](./LICENSE)
[![Website](https://img.shields.io/badge/website-runspace.io-blue)](https://runspace.io)

AI agents shouldn't feel like detached chatbots. They should sit beside you in
the same workspace — reading the repository, running commands, streaming their
work live, and letting you intervene when needed.

Runspace gives every agent its own sandboxed environment while keeping you in
the loop through a shared realtime timeline, terminal, diffs, and pull requests.
It's open source and runs on your own infrastructure.

## Run locally with Docker Compose

Install Docker Desktop, then start the complete local stack:

```bash
cp .env.example .env
docker compose up -d --build
```

Open the web app at `http://localhost:3000`. The gateway health endpoint is
`http://localhost:8080/healthz`; NATS monitoring is at `http://localhost:8222`.
Traefik owns the browser-facing port and routes same-origin `/gateway` REST and
WebSocket traffic to the Go gateway; Next.js and Go remain private Compose
services behind that boundary.
Run `./scripts/smoke-test.sh` (or `./scripts/smoke-test.ps1` on PowerShell) to
check HTTP health and publish/consume a sample `chat.message` event. Stop the
stack with `docker compose down`.

Local authentication defaults to `admin` / `admin`; an `alice` / `alice`
collaborator is also available for multi-session development. Override both with
`LOCAL_AUTH_USERS` using comma-separated `username:password` pairs.

For hot reload during development, use the development override. The frontend
runs Next.js in watch mode and the Go gateway runs Air with the source mounted:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

Use `docker compose -f docker-compose.yml -f docker-compose.dev.yml down` to
stop the development stack.

## Connect a host resource

Runspace can work from a normal folder anywhere on the host without uploading
it, indexing it recursively, or bind-mounting it into Docker. The small
loopback-only Host Agent exposes bounded, lazy file browsing: Runspace reads
only the directory the user expands and the file the user opens.

1. Start the Compose stack.
2. Start the companion in another terminal:

   ```bash
   go run ./cmd/host-agent
   ```

3. In a channel's right sidebar, choose **Connect resource → Local folder**
   and enter the absolute host path.

Git is optional. The modal identifies a plain folder as ready and offers
**Initialize Git (optional)** without blocking the connection. Plain folders
support files, agents, and terminals; Git repositories additionally expose
branch, changes, commit, and publish capabilities. When an `origin` exists,
Runspace clones it once to preserve its history; when Git exists without a
remote, it initializes independent Git metadata inside Docker. `.git` is
deliberately excluded from file synchronization to prevent index and lock-file
corruption. For private remotes, the gateway needs credentials only when cloning
the optional origin.

Channels can attach multiple workspace resources, including remote Git sources,
local Git mirrors, and plain folders. Resource-scoped Code, Changes, and
Terminal actions live beside the selected resource in the channel rail. The
terminal workspace keeps multiple tabbed shells open, including several shells
for the same resource.

Opening a terminal always prompts for its execution boundary. **Workspace
terminal** runs in the Docker sandbox. Local resources additionally offer **Host
terminal · User**, or **Host terminal · Administrator** when the Host Agent was
itself started with elevated privileges. Host shells are restricted to folders
previously approved through a local-resource connection. Runspace never elevates
silently: restart the companion as Administrator on Windows or root on
macOS/Linux if that capability is intentionally required. Windows host terminals
use PowerShell; macOS and Linux use `$SHELL` with `/bin/sh` as the fallback.

The local-resource field asks the Host Agent for directory-only path suggestions
and supports queueing multiple folders before connecting the batch.

The Host Agent listens only on `127.0.0.1:7799`. Folder approvals persist in the
current user's config directory and are restored when the agent restarts.

## Quality gates

Install `staticcheck` and `golangci-lint`, then run the complete local gate:

```bash
pnpm quality
```

It enforces Prettier, the 300-line focused-file limit, ESLint complexity and
function-size rules, strict TypeScript, Vitest, the production Next.js build,
Go tests, `go vet`, Staticcheck, and golangci-lint. With the development stack
running, `pnpm quality:e2e` exercises the contained browser collaboration path.

For GitHub pull-request publishing, set `GITHUB_TOKEN` in `.env`. Repository
checkouts are mounted in the explicit `runspace-repository-data` Docker volume
shared by the gateway, terminal, and agent containers, then exposed through
bounded, read-only tree/file endpoints.

The browser uses ChatScope primitives for the event-driven timeline, Monaco for
code viewing, and xterm.js for the Docker-backed terminal. The Compose gateway
mounts the Docker socket for local development; production deployments should
move terminal execution behind a dedicated executor boundary. Deployment orchestration and OpenTelemetry exporters are on the roadmap.

Channels support per-channel secrets through
`PUT /api/v1/channels/{channelID}/secrets/{name}`. Secret values are stored
durably in PostgreSQL as AES-GCM ciphertext, and list responses only return
names and timestamps. Child channels inherit parent configuration,
repositories, and secrets; child values override parent values. Set
`CHANNEL_SECRET_KEY` to a stable local value. ACP connection metadata belongs
in `config.agent`; credentials belong in the secret store and are never
returned.
