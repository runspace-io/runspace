<p align="center">
  <img src="https://raw.githubusercontent.com/runspace-io/runspace/main/apps/web/public/brand/runspace-logo.svg" alt="Runspace" width="520" />
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License: MIT" /></a>
  <a href="https://runspace.io"><img src="https://img.shields.io/badge/website-runspace.io-blue" alt="Website" /></a>
</p>

---

AI agents should work beside you, not chat from a distance. Runspace gives every
agent a sandboxed environment. You watch the work happen live through a shared
timeline, terminal, diffs, and pull requests. All on your own infrastructure.

---

## Features

- **Sandboxed agents** run in isolated Docker containers with bounded file access
- **Live timeline** streams agent messages, logs, and file changes in realtime
- **Built-in code review** with Monaco diff viewer and direct PR creation
- **Multi-resource channels** let you attach repos and local folders to the same conversation
- **Host integration** connects local folders without uploading, via a loopback agent
- **Git-native** branches, changes, commits, and PR publishing without leaving the workspace
- **Self-hosted** under the MIT license, runs on your own infrastructure with Docker Compose

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

| Layer       | Technology                                   |
| ----------- | -------------------------------------------- |
| Frontend    | Next.js, TypeScript, Monaco Editor, xterm.js |
| Gateway     | Go, NATS, WebSocket                          |
| Persistence | PostgreSQL                                   |
| Runtime     | Docker (isolated per-agent containers)       |

## Links

- [Website](https://runspace.io)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Code of conduct](CODE_OF_CONDUCT.md)
