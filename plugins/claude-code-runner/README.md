# claude-code-runner

Status: planned. This folder contains no code yet.

## Purpose

Reference adapter for dev forums: a forum with a project binding is a project, and every thread
in it is a Claude Code session. The runner is a standalone process on the host where the
projects live. Per turn it starts `claude -p` with stream-json, fills the live box from the
stream and passes follow-ups through stdin into the running turn.

## Contract

- General: docs/spec/09-agents.md sections 4 to 16.
- Runner-specific: docs/spec/09-agents.md section 18 (process contract, environment as an
  allow-list, own process group, stop by SIGINT, lock per session, live box from the
  stream, follow-ups, costs from `modelUsage`, error paths, file rules).
- Backend policy per run: docs/spec/10-backend-policy.md section 10.2.
- Isolation for several people: docs/spec/21-dev-isolation.md.

## Fixed rules

- Project folders only under `runner.projects_root`; the server never names an absolute path.
- The environment of the child process is an allow-list; login variables only according to the
  configured mode of the invocation token.
- Check the reported mode before the start with `claude auth status` in the run environment,
  cross-check `apiKeySource`; on a mismatch, no turn.
- The runner never reads, copies or writes login files of a runtime.
- No automatic retry of an aborted turn.
- `--dangerously-skip-permissions` only with tool class `full` and only when the guard rules from
  docs/spec/07-permissions.md allow it.

## Planned structure

| Part | Content |
|---|---|
| Main process | gateway connection, check the token, distribute runs, heartbeat with project readiness |
| Turn process | one `claude -p` per turn, detached, own process group |
| Stream reader | mapping of the stream messages to the run document |
| State | `runner.state_dir`: sessions, locks, recordings, input files |
| Tests | fake Claude (replayed fixtures); test against the installed, pinned CLI version for recognized events and error texts; real runs only behind a switch |

## To check before the build

- Does `system/init` arrive with `--input-format stream-json` before the first model request?
- Is `/compact` available in print mode of the pinned version?
