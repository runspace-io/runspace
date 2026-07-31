# Contributing to Runspace

Thanks for your interest in contributing. Runspace is an open-source engineering
workspace where humans and AI agents collaborate — and we welcome contributions
that improve that experience.

## Getting started

```bash
git clone https://github.com/runspace-io/runspace.git
cd runspace
cp .env.example .env
docker compose up -d --build
```

Open `http://localhost:3000` and sign in with `admin` / `admin`.

## Development workflow

1. **Find or create an issue** — check existing issues before starting work.
2. **Fork the repo** and create a feature branch from `main`.
3. **Run quality gates** before pushing:

   ```bash
   pnpm quality      # Prettier, lint, typecheck, test, build
   ```

4. **Open a pull request** with a clear description of the change.

## Code conventions

- TypeScript uses strict mode and ESLint with complexity limits.
- Go uses `gofmt`, `go vet`, Staticcheck, and golangci-lint.
- Keep files focused — the repo enforces a 300-line limit per file.
- Write tests for new behavior.

## Architecture

The stack is structured as:

- **`apps/web/`** — Next.js frontend (TypeScript, Monaco, xterm.js)
- **`internal/`** — Go gateway, agent orchestration, event system
- **`migrations/`** — PostgreSQL schema
- **`docker-compose.yml`** — Local development stack (NATS, PostgreSQL, Traefik)

See [docs/](docs/) for the full architecture, event model, and API contracts.

## License

By contributing, you agree that your contributions will be licensed under the
MIT License.
