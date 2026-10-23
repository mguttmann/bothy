# claude-code-runner

Status: Core (M2-34). Follow-ups over stdin, the stop, compaction and the error paths of
docs/spec/09-agents.md 18.9 follow with M2-35, answer attachments with M3-12.

## Purpose

Reference adapter for development forums: a forum with a project binding is a project, every
thread in it is one Claude Code session. The runner is one process per agent token on the host
that holds the projects. For every run it starts exactly one `claude -p` turn with stream-json in
the resolved project folder, fills the live box from the stream, answers in the thread, proposes
the title once per new session and reports the cost of the run.

## Contract

- General: docs/spec/09-agents.md sections 4 to 16 (token check, inbox, run loop, context with
  trust marks, heartbeat); the SDK (`sdk/python`) builds them and the runner is its handler.
- Runner: docs/spec/09-agents.md section 18 (project and thread, process contract with the tool
  class table, mode per run, live box, costs) and section 21 (configuration keys).
- Backend policy per run: docs/spec/10-backend-policy.md 7 and 10.2.
- Rulings: docs/decisions.d/M2-34.md, rows `D-M2-34-0` to `D-M2-34-12`.

## Layout

| Path | Content |
|---|---|
| `src/claude_code_runner/__main__.py` | Commands `run --config <path>` and `check-config --config <path>` |
| `config.py`, `secrets.py` | TOML configuration with its checks; `secret_ref` values (`file`, `env`) read fresh per use |
| `app.py` | Handler of the SDK run loop (acceptance answers, prepare, execute), start and stop |
| `channels.py`, `workspace.py` | Cache of the served forums and their bindings; resolution of `workspace_ref` and readiness |
| `toolclass.py`, `commandline.py`, `environment.py` | Tool class order, the argument list of a turn, the allow-list environment |
| `authcheck.py`, `heartbeat.py` | Pre-check `claude auth status`; version probe and the run-time heartbeat members |
| `prompt.py`, `prompts/runner_prompt.md`, `attachments.py` | System prompt file and first stdin line; attachments as local files |
| `locks.py`, `turn.py`, `statedir.py` | File locks, the turn process and `terminate`, private state files and retention |
| `streamreader.py`, `livebox.py`, `usage.py`, `title.py` | Stream file reader, live box, cost difference, title proposal |
| `tests/unit/`, `tests/scenarios/` | Unit tests; scenarios with Fake-Claude and the SDK FakeServer |
| `tests/fake_claude/` | Fake-Claude (plays `fixtures/*.jsonl`, constructed from the message shapes of 18.4) |
| `tests/real_cli/` | Tests against the installed CLI, only behind switches (see below) |

## Configuration

One TOML file; keys are nested by their dots (`runner.stop_grace` is `stop_grace` in `[runner]`).
`config.example.toml` lists every key of the runner with its default. Besides the `runner.*` keys
the file may set `adapter.invocation_token_leeway`, `adapter.jwks_refresh_min_interval`,
`agent.run_update_min_interval` and the `sdk.*` keys; the reference is
docs/operations/configuration.md (generated from 09 21). Operator rules:

- `runner.state_dir` is created with mode `0700` and refused when group or others have a right on
  it; every file there has mode `0600`. It holds the SDK inbox with invocation tokens,
  transcripts, prompt files and downloaded attachments.
- `adapter.invocation_token_leeway` must be at most `auth.clock_leeway` of the server; the runner
  cannot check this itself.
- `runner.anthropic_api_key_ref` enables `organization`; without it such runs end
  `runtime_not_authenticated` and the CLI is never called. The runner's own configuration folder
  `<runner.state_dir>/claude-config/organization` stays without any login.
- Enable `runner.allow_personal` only when the runner runs as the account of the agent's
  `credential_holder`: `personal` uses the login under that account's `HOME`, and I1 and I2 then
  guarantee that only this person triggers (docs/spec/21-dev-isolation.md 1).
- After a switch of the mode (I4) `--resume` does not find the earlier session, because sessions
  of `organization` live in the runner's own configuration folder and those of `personal` under
  `HOME`; M2-35 handles a missing session (fail open).
- Install exactly the pinned Claude Code version (`PINNED_CLAUDE_CODE_VERSION` in
  `heartbeat.py`, 2.1.259) and keep automatic updates of the CLI off; another version reports
  `runtime_version_untested`, an older one `runtime_version_unsupported` and refuses runs.
- `runner.forward_ssh_agent` gives every turn the runner user's SSH keys; keep it off unless the
  projects need it.

## Running

```sh
cd plugins/claude-code-runner
uv run --locked claude-code-runner check-config --config /etc/claude-code-runner.toml
uv run --locked claude-code-runner run --config /etc/claude-code-runner.toml
```

`check-config` contacts no server; it prints one code per check (`ok`, `unset` or the problem).
`run` logs JSON lines on stderr (event codes and IDs, never content, tokens or keys) and stops on
SIGINT or SIGTERM; running turns are ended with SIGINT, SIGTERM and SIGKILL to their group.

## Tests

```sh
cd plugins/claude-code-runner && uv run --locked pytest
```

Every test uses Fake-Claude through `runner.claude_bin` and a fake `HOME`; no installed CLI and
no login of the developer is touched. The real CLI runs only behind switches:

- Level one, free and without credentials: `CLAUDE_CODE_RUNNER_REAL_CLI=1 uv run --locked pytest
  tests/real_cli` checks the installed version against the pin and the flags of `--help`; a
  missing CLI fails.
- Level two, additionally `CLAUDE_CODE_RUNNER_REAL_TURN=1` and `CLAUDE_CODE_RUNNER_REAL_KEY_FILE`:
  `auth status` per mode and two short real turns (one `--resume`). It uses the operator's key or
  login and costs money; running it is the owner's decision.

## To check before the build

- Does `system/init` come before the first model request with `--input-format stream-json`?
  Open; until it is checked with level two, the pre-check is binding and the counter-check at
  `system/init` interrupts the turn with SIGINT.
- Is `/compact` available in print mode of the pinned version? Open; M2-35 needs it.
