# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in Runspace, **do not** open a public
issue. Instead, email **dev@runspace.io** with:

- A description of the vulnerability
- Steps to reproduce
- Affected versions
- Any potential mitigations you've identified

We aim to acknowledge reports within 48 hours and provide an initial assessment
within 5 business days.

## Supported versions

Only the latest release on `main` receives security patches. We recommend
running the most recent version in production.

## Security model

- **Authentication** — GitHub OAuth with optional local auth for development.
  Secrets in `.env` are never committed.
- **Sandboxing** — Agent runs execute in isolated Docker containers with
  bounded file access.
- **Secrets** — Channel secrets are stored as AES-GCM ciphertext in PostgreSQL.
  Values are never returned in list responses.
- **Host Agent** — Listens only on `127.0.0.1:7799`. Never exposes file
  contents without explicit user approval per directory.
