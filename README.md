<p align="center">
  <img src="https://raw.githubusercontent.com/runspace-io/runspace/main/apps/web/public/brand/runspace-wordmark.svg" alt="Runspace" width="400" />
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License: MIT" /></a>
  <a href="https://runspace.io"><img src="https://img.shields.io/badge/website-runspace.io-blue" alt="Website" /></a>
</p>

---

AI agents shouldn't feel like detached chatbots. Runspace gives every AI agent
its own sandboxed environment while keeping you in the loop through a shared
realtime timeline, terminal, diffs, and pull requests — all on your own
infrastructure.

<p align="center">
  <em>Screenshot coming soon</em>
</p>

---

## Features

- **Sandboxed agents** — each agent runs in an isolated Docker container with bounded file access
- **Live timeline** — agent messages, logs, and file changes stream in realtime as they happen
- **Built-in code review** — inspect diffs with Monaco editor and open PRs directly
- **Multi-resource channels** — attach multiple repos and local folders to the same conversation
- **Host integration** — connect local folders without uploading, via a loopback Host Agent
- **Git-native** — branches, changes, commits, and PR publishing without leaving the workspace
- **Self-hosted** — MIT licensed, runs on your own infrastructure with Docker Compose

## Quick start

```bash
git clone https://github.com/runspace-io/runspace.git
cd runspace
cp .env.example .env
docker compose up -d --build
```

Open `http://localhost:3000` and sign in with `admin` / `admin`.

## Development

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
pnpm quality
```

## Stack

| Layer | Technology |
|-------|------------|
| Frontend | Next.js, TypeScript, Monaco Editor, xterm.js |
| Gateway | Go, NATS, WebSocket |
| Persistence | PostgreSQL |
| Runtime | Docker (isolated per-agent containers) |

## Links

- [Website](https://runspace.io)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Code of conduct](CODE_OF_CONDUCT.md)
