# Feature 3: Agent Runs

## Outcome

The user gives a real coding agent a task, watches normalized live activity,
sends follow-up input, and reliably stops or retries the run.

## Build

- `Runtime` interface and conformance suite.
- Codex CLI reference adapter plus retained deterministic mock adapter.
- Native ACP JSON-RPC adapter over stdio (`ACP_COMMAND`) for any ACP-compatible runtime.

When `ACP_COMMAND` is set, the gateway launches one ACP peer per run and maps
`session/update` notifications to the shared `agent.output` stream. Leave it
empty for the deterministic local mock runtime used by the Compose smoke test.
The gateway derives the ACP session working directory from the repository ID
after verifying that repository belongs to the workspace; filesystem paths are
never accepted from or returned to the browser.

- Run composer with task, runtime, base branch/SHA, and visible limits.
- Queue claim, prepare, start, input, stop, timeout, and reconciliation logic.
- Output normalization into message, log, command, usage, and input-request
  records.
- Live run status, activity indicator, command cards, raw-log panel, and stop.
- Secret redaction before persistence/publication.
- Complete run lifecycle event catalog and metrics.

The gateway exposes `POST /api/v1/runs/{runID}/input` for follow-up runtime
input. Runtime implementations may opt into the `InputAgent` boundary; the
deterministic mock supports it for tests while unsupported adapters return a
clear capability error.

## Product rules

- Starting a run snapshots the base SHA and task.
- Each run creates a dedicated agent actor displayed like a collaborator.
- Unknown runtime output remains available as raw logs; it never crashes the
  parser.
- Stop is always visible during non-terminal states.
- Retry creates a new run with the same workspace, channel, thread, repository,
  prompt, and agent configuration. The repository directory is authorized and
  resolved again, so retry remains correct after a gateway restart.
- The runtime cannot publish a PR unless the user initiates the publish flow.

## Acceptance scenarios

1. A real runtime changes a fixture repository from a natural-language task.
2. Status progresses through queued, preparing, running, and one terminal state.
3. Commands, messages, and raw output remain ordered after reconnect.
4. Follow-up input reaches a waiting runtime.
5. Stop is idempotent and kills further command execution within the target.
6. Malformed adapter output is preserved and the run ends clearly.
7. Runtime crash, timeout, quota error, and image failure map to distinct safe
   user-facing states.

## Not included

Agent teams, runtime marketplace, user-defined adapter code, scheduled runs,
background autonomous tasks, or automatic PR publishing.
