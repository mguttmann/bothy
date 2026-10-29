# claude-code-runner

Status: core (M2-34), follow-ups, stop, error paths and adoption after a crash (M2-35), answer
attachments (M3-12), folder creation of new projects (M4-43).

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
- Follow-ups, stop and error paths: 09 18.5, 18.6 and 18.9.
- Rulings: docs/decisions.d/M2-34.md (`D-M2-34-0` to `D-M2-34-14`) and docs/decisions.d/M2-35.md
  (`D-M2-35-1` to `D-M2-35-21`).

## Layout

| Path | Content |
|---|---|
| `src/claude_code_runner/__main__.py` | Commands `run --config <path>` and `check-config --config <path>` |
| `config.py`, `secrets.py` | TOML configuration with its checks; `secret_ref` values (`file`, `env`) read fresh per use |
| `app.py` | Handler of the SDK run loop (acceptance answers, prepare, execute, follow-ups, stop hook), adoption at start, start and stop |
| `runturn.py` | The turn processes of one run: recovery, follow-ups on stdin, stop, answers, box notes, finish, turn record |
| `ledger.py` | The recovery invariants of a turn (stored offset, answers, inputs, kind, decided end), part of the turn record |
| `failures.py`, `recovery.py` | Recognised CLI texts (`ERROR_TEXTS`) and the classification of a process end; the next action and the box notes |
| `followups.py`, `stop.py`, `adoption.py` | Follow-up check, line and stdin rule; the stop schedule; identity check and adoption decision |
| `channels.py`, `workspace.py` | Cache of the served forums and their bindings; resolution of `workspace_ref`, readiness and the project folder report (09 18.13, M4-44) |
| `provision.py` | Folder creation of a binding with `provision` `init` or `clone`, its creation record and the readiness reasons (09 18.12, M4-43) |
| `toolclass.py`, `commandline.py`, `environment.py` | Tool class order, the argument list of a turn, the allow-list environment |
| `authcheck.py`, `heartbeat.py` | Pre-check `claude auth status`; version probe and the run-time heartbeat members |
| `prompt.py`, `prompts/runner_prompt.md`, `attachments.py` | System prompt file and first stdin line; attachments as local files |
| `locks.py`, `turn.py`, `statedir.py` | File locks, the turn process and `terminate` (through `stop.py`), private state files and retention |
| `streamreader.py`, `livebox.py`, `usage.py`, `title.py` | Stream file reader, live box, cost difference, title proposal |
| `reply_attachments.py`, `reply_upload.py` | Answer attachments: `ATTACH:` lines, roots, denied patterns, the checks of one file and of the set, the notice; staging, upload, sending and the persisted set (M3-12) |
| `tests/unit/`, `tests/scenarios/` | Unit tests; scenarios with Fake-Claude and the SDK FakeServer |
| `tests/fake_claude/` | Fake-Claude (plays `fixtures/*.jsonl`, constructed from the message shapes of 18.4 and the texts of the pinned CLI; see 09 16.1) |
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
- Keep `runner.progress_patch_interval` below `run.stall_after` of the server, else a run whose
  only progress is a long running tool gets the stall notice; the runner cannot read the server
  value.
- Keep `runner.stop_grace` plus `runner.kill_grace` below `agent.cancel_grace` of the server,
  else the server ends a stopped run with `cancel_timeout` before the runner reports
  `cancelled`; the runner cannot read the server value.

## Stream mapping (09 18.4)

`livebox.py` maps each stream message of a turn to changes of the SDK's run patcher, which
throttles them to `agent.run_update_min_interval`.

| Message | Live box |
|---|---|
| `system/init` | `/model` |
| `assistant` | `/last_text_excerpt`; a `tool_call` step per `tool_use`, under the step of `parent_tool_use_id`; `/metrics/context_pct` |
| `user` | the `tool_result` ends its step `completed` or `failed`; the echo of the own prompt is ignored |
| `tool_progress` | counted; `/metrics/progress_events` only when the last change of the turn is older than `runner.progress_patch_interval`, so a long tool is not stalled (09 5.5) |
| `system/task_*` | a step `subagent` (`task_type` `local_agent`), `workflow` (`local_workflow`) or `background_job`, under the step of the tool call its `tool_use_id` names (at the root without one); `usage` (`total_tokens`, `tool_uses`, `duration_ms`) of `task_progress` and `task_notification` becomes `detail.usage` (`tokens`, `tool_uses`, `duration_ms`), never a cost; `completed`, `failed`, `stopped` end the step; `ambient` tasks get none |
| `system/background_tasks_changed` | open `background_job` steps outside the set end `completed`; `/metrics/background_jobs` |
| `system/status` | `compacting` sets `/status_line`, its absence removes it |
| `system/api_retry` | `/metrics/api_retries`, `/metrics/last_api_retry` |

The run graph fields (`tool_use_id`, `task_type`, `usage`, the workflow type `local_workflow`)
were read from the files of the pinned CLI 2.1.259 without a session
(`tests/fixtures/recording/stream_fields_2.1.259.json`, ruling D-M2-51-1); the level-one switch
test holds them against the installed CLI.

## Follow-ups, stop and error paths (M2-35)

- **Follow-ups** (`capabilities.followups = steer`): a follow-up is read with `getMessage` and
  written to the running turn's stdin as one `user` line, `Follow-up from <name> (@<handle>):`, a
  blank line and the text, with its own `uuid`; the runner patches `delivered`, and `absorbed`
  once Claude Code echoes the line (`--replay-user-messages`). At the moment of writing the token
  is checked again (expiry, mode, tool class, run, `trigger_kind`, author and thread); a failed
  check writes nothing and never ends the run. Follow-ups of a run that has not started are held
  and written after the prompt line. After a `result` stdin stays open for
  `runner.followup_grace` while written follow-ups have no echo.
- **Stop** (`run.cancel_requested`): SIGINT to the turn's process group and stdin closed, SIGTERM
  after `runner.stop_grace`, SIGKILL after `runner.kill_grace` (log code `stop_escalated`); the
  run finishes `cancelled` with the request's reason only when no process of the group is left,
  without an answer. A run that has not started is never started. A run the server ended
  (`409 conflict` on a patch, or `401`) has its turn stopped and is not finished.
- **Error paths** (09 18.9): a full context is compacted once (`/compact` as the only input line
  of a process with `--resume`) and the same prompt runs again, with a retry hint in the runner
  prompt when the turn had worked; a missing session is replaced by a new one and reported (with
  `bind_failed` in mode `bind`), fail open; a session that exists at the first turn is resumed; a
  missing login ends `runtime_not_authenticated`; an error `result` ends `runtime_error` with its
  texts in the last step. Box notes are run steps of kind `status` with the codes
  `context_compacted`, `session_missing` and `runtime_error`.
- **Crash of the runner**: turns keep running in their own process group with stdout and stderr in
  files. At start, before the first inbox event and heartbeat, the runner reads the turn records
  and, after checking the identity of each process (`ps` start time and session reference),
  adopts a live turn of a running run, stops a live turn of an ended run, finishes a gone turn
  from the `result` in its stream file, or leaves a gone turn without `result` to the server
  (`interrupted`). It never starts a process or `run.start` for such a run. The turn ledger
  (`ledger.py`, D-M2-35-28) makes every crash point safe, also a second crash of the adopting
  runner: the stored offset never passes the run's last `result` before the finish, a reply's
  `client_msg_id` is stored before the send, an input counts as answered by its own `uuid`, the
  decided end is stored before the costs, and a process keeps its kind.
  `tests/unit/test_crash_points.py` enumerates every first and second crash point.

## Costs

The runtime reports cumulative values per session (`result.modelUsage`), also across
`--resume`. The runner keeps the last values of each session in
`<runner.state_dir>/sessions/<ref>/usage.json` and reports per run only the difference
(docs/spec/09-agents.md 18.7). A session bound with a cost baseline (an imported session or a
rebind, `AgentSession.cost_baseline_micros` with `currency`, docs/spec/22-import-migration.md 9
step 7) already holds amounts the old environment booked. When the runner has no stored values
for such a session, it keeps a remaining baseline in `usage.json` and subtracts it from the
amounts of the next runs, shared across the models in proportion to their amounts and never below
zero per model, until it is used up; a reset of the runtime's counters (any value goes down)
clears it. The rule applies in every session mode, not only `bind`: a first run after a binding
that ended before its values were stored would otherwise book the old amount on the next
`resume` (docs/decisions.d/M8-06.md). A baseline in another currency than US dollars is ignored
with the warning `cost_baseline_currency_ignored` (no conversion). Tokens have no baseline in the
data model, so the token counts of the first run after a binding can include the old session's
tokens.

## Running

```sh
cd plugins/claude-code-runner
uv run --locked claude-code-runner check-config --config /etc/claude-code-runner.toml
uv run --locked claude-code-runner run --config /etc/claude-code-runner.toml
```

`check-config` contacts no server; it prints one code per check (`ok`, `unset` or the problem).
`run` logs JSON lines on stderr (event codes and IDs, never content, tokens or keys) and stops on
SIGINT or SIGTERM; running turns are ended with SIGINT, SIGTERM and SIGKILL to their group.

## Folder creation (M4-43)

A binding with `provision` `init` or `clone` (a project created from the app) asks the runner to
create its folder under `runner.projects_root` (docs/spec/09-agents.md 18.12). The runner:

- creates only a one-segment name (`^[A-Za-z0-9][A-Za-z0-9._-]*$`) directly under the root, and
  clones only an `https` URL without user information, query or fragment; anything else is
  `provision_refused` before any process starts;
- never writes into a folder it did not create: a name that exists (also a link) without an entry
  in `<runner.state_dir>/provisioned.json` is `workspace_exists`; bind such a folder with
  `provision` `none`;
- runs `runner.git_bin` from the cleaned `PATH` with an argument list (`git init --quiet
  [--initial-branch <branch>]` in the new folder, `git clone --quiet --no-recurse-submodules --
  <url> <root>/.provision-<uuid>` and a rename), the allow-list environment plus
  `GIT_TERMINAL_PROMPT=0` and `GIT_ALLOW_PROTOCOL=https`, stdin closed, output discarded, and the
  time limit `runner.provision_timeout`;
- reports `provisioning` meanwhile, then sends a heartbeat at once with `ready` or one of
  `init_failed`, `clone_failed`, `provision_timeout`; a failure removes only what the attempt
  created and is not tried again for the same binding before `runner.provision_retry_after`.

A private repository needs a credential helper of the runner user (`HOME` is kept); `ssh` clones
are not supported.

## Project folder report (M4-44)

Every heartbeat carries `project_folders`: the names of the Git work trees directly under
`runner.projects_root` (docs/spec/09-agents.md 18.13). An entry counts when its name is one
segment of letters, digits, `.`, `_` and `-` that starts with a letter or digit (so hidden folders
and the `.provision-<uuid>` folders never show), it resolves like a `workspace_ref` to a folder
under the root and it holds `.git`. The names go out in code point order, at most
`runner.project_folders_max`; more are cut and logged as `project_folders_truncated` with the
count, and a root that cannot be read sends the empty list and logs `project_folders_unreadable`.
Only names leave the runner. The app lists those without a forum under "Add forums for existing
folders" and binds the chosen ones with `provision` `none`.

## Answer attachments (M3-12)

Claude announces a file with a line `ATTACH: <path>` at column 0, outside fenced code blocks (the
runner prompt says so). The runner removes these lines from the answer and sends the files with
it, following docs/spec/09-agents.md 18.10; the heartbeat reports `capabilities.files_out =
true`, and the owner's switch `supports_files_out` is enforced by the server at the send.

- Roots (`runner.attach_allowed_roots`, by name only, default all three): `project` (the turn's
  resolved working directory), `thread_inbox` (`<runner.state_dir>/files/<thread_id>` of the
  run's thread) and `runner_temp` (`<runner.state_dir>/tmp/<run_id>`, mode `0700`, the turn's
  `TMPDIR`, removed when the run ends). An absolute path or another name aborts the start; an
  operator can narrow the list, never widen it. The rest of `runner.state_dir` and
  `runner.projects_root` are never roots.
- Denied patterns (`runner.attach_denied_patterns`): `fnmatch` patterns without `/`; the list
  must contain `.env*`, `*.pem`, `*.key`, `id_*`, `.netrc`, `.ssh`, `.claude`, `.config`,
  `.aws`, `.azure` and `.gnupg` and may add a project's own secret names. Matching is
  case-insensitive, on every component of the real path below its root and on the announced
  name.
- Checks per file, in order: `realpath` (links followed), strictly below a root, no denied
  pattern, a regular file with one link and at least one byte. Then the set: the limits of
  `getInstance` (`message_max_attachments`, `file_max_upload_bytes`); any rejection sends no
  file. Accepted files are copied with `O_NOFOLLOW` and an inode check into
  `<runner.state_dir>/outgoing/<run_id>/` (mode `0600`) and uploaded from the copies, one after
  the other, through `createUpload`, `uploadPart` and `completeUpload` inside the run binding
  (07 SR9); `/status_line` is `uploading_attachments` meanwhile.
- All or none: one rejected file, a failed upload or a refused attachment
  (`403 feature_disabled`, `403 forbidden`, `409 conflict` with `file_not_attachable`) sends the
  answer without any attachment and with the notice `Attachments not sent:` and one line per
  file, its base name and a code of the closed set `not_found`, `outside_allowed_roots`, `denied_pattern`,
  `not_a_regular_file`, `hard_link`, `empty_file`, `changed_during_check`, `too_large`,
  `too_many`, `limits_unavailable`, `upload_refused`, `upload_failed`, `not_attached`,
  `set_not_sent`. A failed set never fails the run.
- Crash and stop: the progress of a set is kept in
  `sessions/<ref>/<run_id>.attach-<answer>.json` (keys before their requests, file IDs right
  after their `201`), so an adopting runner resumes it without a second upload session or a
  second answer. A stop cancels the request in flight, aborts the open sessions and sends
  nothing. Staged copies and records go once the answer is settled; leftovers fall to the
  retention sweep (`runner.stream_log_retention`).
- Residuals: the checks protect the paths the runner sends, not what a turn of class `host` can
  read or copy on its own (T-DEV-01, SR2); a secret file under a name no pattern knows is sent
  when it lies under a root, so add your project's secret names to
  `runner.attach_denied_patterns`; content checks are the server's (scan, 12 5). Uploaded files
  that end up unattached (a failed set, a stop) stay readable only by the agent until the
  cleanup of never attached files (M3-08) removes them.

## Tests

```sh
cd plugins/claude-code-runner && uv run --locked pytest
```

Every test uses Fake-Claude through `runner.claude_bin` and a fake `HOME`; no installed CLI and
no login of the developer is touched. The real CLI runs only behind switches:

- Level one, free and without credentials: `CLAUDE_CODE_RUNNER_REAL_CLI=1 uv run --locked pytest
  tests/real_cli` checks the installed version against the pin and the flags of `--help`, and
  that every recognised error text of `ERROR_TEXTS` and every stream type and member the runner
  reads occur in the files of the installed CLI (`test_pinned_cli.py`); a missing CLI fails. The
  pinned CLI can be installed into a temporary folder from the npm package of the platform
  (`@anthropic-ai/claude-code-<platform>` at the pinned version) without a login.
- Level two, additionally `CLAUDE_CODE_RUNNER_REAL_TURN=1` and `CLAUDE_CODE_RUNNER_REAL_KEY_FILE`:
  `auth status` per mode and two short real turns (one `--resume`). It uses the operator's key or
  login and costs money; running it is the owner's decision.

## To check before the build

- Does `system/init` come before the first model request with `--input-format stream-json`?
  Open; until it is checked with level two, the pre-check is binding and the counter-check at
  `system/init` interrupts the turn with SIGINT.
- Is `/compact` available in print mode of the pinned version? Yes: the command `compact` of
  Claude Code 2.1.259 has `supportsNonInteractive: true` in the files of the pinned CLI
  (retrieved 2026-10-24), and the Agent SDK page on slash commands sends `/compact` as a prompt
  and reports it with a `system` message `compact_boundary`
  (https://code.claude.com/docs/en/agent-sdk/slash-commands, retrieved 2026-10-24). That the CLI
  takes it as the content of a stream-json `user` line is not run with a real session yet (level
  two would need a paid run).
