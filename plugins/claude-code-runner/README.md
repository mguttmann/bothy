# claude-code-runner

Status: core (M2-34), follow-ups, stop, error paths and adoption after a crash (M2-35), answer
attachments (M3-12), folder creation of new projects (M4-43), live reload of channels and
optional transcription of voice messages (RUNNER-LIVE-RELOAD), Git optional, a default folder for
forums without a binding and the cause of every refusal in the thread (RUNNER-NO-GIT), hosting of
built-in agents behind `runner.hosted_agents` (A1-04) and runs their turns from their
definitions (A1-05).

## Purpose

Reference adapter for development forums: a forum with a project binding is a project, every
thread in it is one Claude Code session. The project folder need not be a Git repository, and a
forum without a binding works in `runner.default_workspace` when that is set (see "Forums
without Git or without a folder"). The runner is one process per agent token on the host
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
- Hosting of built-in agents: docs/spec/26-builtin-agents.md 3 and 5.6 item 4, 09 18.18.
- Rulings: docs/decisions.d/M2-34.md, docs/decisions.d/M2-35.md and docs/decisions.d/A1-04.md.

## Layout

| Path | Content |
|---|---|
| `src/claude_code_runner/__main__.py` | Commands `run --config <path>`, `check-config --config <path>` and `stop --turns --config <path>` |
| `config.py`, `secrets.py` | TOML configuration with its checks; `secret_ref` values (`file`, `env`) read fresh per use |
| `app.py` | Handler of the SDK run loop (acceptance answers, prepare, execute, follow-ups, stop hook), adoption at start, start and stop |
| `runturn.py` | The turn processes of one run: recovery, follow-ups on stdin, stop, answers, box notes, finish, turn record |
| `ledger.py` | The recovery invariants of a turn (stored offset, answers, inputs, kind, decided end), part of the turn record |
| `failures.py`, `recovery.py` | Recognised CLI texts (`ERROR_TEXTS`) and the classification of a process end; the next action and the box notes |
| `followups.py`, `background.py`, `stop.py`, `adoption.py` | Follow-up check, line and stdin rule; the background work that holds stdin open; the stop schedule; identity check and adoption decision |
| `channels.py`, `workspace.py` | Cache of the served forums and their bindings, reloaded while running (09 18.1); resolution of `workspace_ref` and `runner.default_workspace`, readiness and the project folder report (09 18.13, M4-44) |
| `causes.py` | The sentence for the thread that names the cause of a refused run (09 18.9, RUNNER-NO-GIT) |
| `hosting.py` | The hosted agents of `runner.hosted_agents`: the hosted set, a token in memory per agent and its renewal, one connection, channel cache, heartbeat and state folder per agent (26 3.2, 3.3), and their turns from the definition (26 5) |
| `definition.py` | The runner's own check of a definition (26 4.2, 4.3), held equal to the server's by the corpus `tests/agent-definitions`; the class a definition needs and the tool intersection (26 4.4) |
| `materialize.py` | The agent and control folders: every file of 26 5.2, the runner settings of 26 5.4, the rebuild before each turn and the per-turn MCP file (26 5.6) |
| `channel_skills.py` | Channel skills behind `runner.channel_skills_dir`, only in `host` turns (26 5.9) |
| `transcribe.py` | The local transcriber of audio attachments and the note without a transcript (09 18.14) |
| `searchassist.py` | The search assistant: one call without tools per question, the reply parsed into picks and terms (09 18.17) |
| `provision.py` | Folder creation of a binding with `provision` `init` or `clone`, its creation record and the readiness reasons (09 18.12, M4-43) |
| `trash.py` | The folder check of the bound forums, the move of a deleted forum's folder to the host trash and its record (09 18.15) |
| `toolclass.py`, `commandline.py`, `environment.py` | Tool class order, the argument list of a turn, the allow-list environment |
| `authcheck.py`, `heartbeat.py` | Pre-check `claude auth status`; version probe and the run-time heartbeat members |
| `prompt.py`, `prompts/runner_prompt.md`, `attachments.py` | System prompt file and first stdin line; attachments as local files |
| `locks.py`, `turn.py`, `statedir.py` | File locks and the slot pool shared by every agent of the process, the turn process and `terminate` (through `stop.py`), private state files (also of each hosted agent) and retention |
| `streamreader.py`, `livebox.py`, `usage.py`, `title.py` | Stream file reader, live box, cost difference, title proposal |
| `reply_attachments.py`, `reply_upload.py` | Answer attachments: `ATTACH:` lines, roots, denied patterns, the checks of one file and of the set, the notice; staging, upload, sending and the persisted set (M3-12) |
| `tests/unit/`, `tests/scenarios/` | Unit tests; scenarios with Fake-Claude and the SDK FakeServer |
| `tests/fake_claude/` | Fake-Claude (plays `fixtures/*.jsonl`, constructed from the message shapes of 18.4 and the texts of the pinned CLI; see 09 16.1) |
| `tests/real_cli/` | Tests against the installed CLI, only behind switches (see below) |
| `tests/realserver/` | The end-to-end suite: the runner with Fake-Claude against the server of this tree (`make adapters-e2e`) |
| `tests/fixtures/baseline/`, `tests/unit/test_byte_equal_baseline.py` | The command line, heartbeats and served channels of the runner without the keys of stage A1, recorded before A1-04 and compared byte for byte (BA-T20) |
| `keeper.py`, `launch.py` | With `runner.turn_handover`: the keeper program of one launch, and the runner's side of the launch handshake, the group of keeper and CLI, the named pipe writer and the cases of a `launching` record (09 18.19.4, 18.19.5) |
| `processstate.py` | `ProcessState`: every stream-derived value of the stdin and stop rule, and its replay (09 18.19.7) |
| `handover.py` | The handover at SIGTERM (exit 75) and `stop --turns` (09 18.19.3) |
| `tests/unit/test_keeper.py`, `test_launch.py`, `test_processstate.py`, `tests/scenarios/test_handover.py`, `test_launch_crash_points.py` | The keeper, the handshake, the replay with the class test over every Fake-Claude stream and checkpoint, AR-T02, AR-T03 and AR-T04 against real runner processes |
| `tests/hosting_world.py` | The scenario world of a hosting runner: the runner's FakeServer with the hosting operations and one FakeServer per hosted agent |

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
  `heartbeat.py`, 2.1.288) and keep automatic updates of the CLI off; another version reports
  `runtime_version_untested`, an older one `runtime_version_unsupported` and refuses runs.
- The API refuses a model to a CLI below the model's minimum version ("Claude Code <v> does not
  support this model; version <v> or newer is required", or the minimum of an organization's
  policy). The run then ends `runtime_error` with a box note that names the required version and
  says to update Claude Code; the runner raises its minimum to that version, reports `degraded`
  with `runtime_version_unsupported` at once and refuses runs until a version probe (before
  every heartbeat) finds a CLI at that version. The raised minimum lives in memory: a restart
  of the runner forgets it, and the next refused turn raises it again.
- `runner.default_model` and the binding's `runner_config.model` may use the CLI's notation with
  a selector in square brackets, such as a context window (`<model>[1m]`): the CLI gets the
  value as it is, `startRun` and `agent_session.report` get the model name without the suffix,
  because the server's model name (09 2.2) has no `[`.
- `runner.forward_ssh_agent` gives every turn the runner user's SSH keys; keep it off unless the
  projects need it.
- Keep `runner.progress_patch_interval` below `run.stall_after` of the server, else a run whose
  only progress is a long running tool gets the stall notice; the runner cannot read the server
  value.
- Keep `runner.stop_grace` plus `runner.kill_grace` below `agent.cancel_grace` of the server,
  else the server ends a stopped run with `cancel_timeout` before the runner reports
  `cancelled`; the runner cannot read the server value.
- `runner.transcribe_cmd` is off by default; see "Voice messages" below. The transcriber runs as
  the runner user with the allow-list environment and gets the audio file of a person's message,
  so install only a program you trust there.

## Forums without Git or without a folder (RUNNER-NO-GIT)

Every forum the runner operates is a Claude Code chat (09 18.1, R-AG-35):

- **Git is optional.** A bound folder is ready when it exists under `runner.projects_root` and
  the runner user may read, write and search it; `.git` is not needed. The runner uses no Git
  feature itself (no worktree per post, no branch names, no diff or commit reports), so nothing
  is skipped; Claude Code finds out on its own whether the folder is a repository.
- **Forums without a binding.** With `runner.default_workspace` set, a run in a forum the runner
  operates but that has no project binding works in that folder: `"."` is the projects root
  itself, a name such as `"scratch"` a folder under it (checked like a `workspace_ref` when the
  configuration loads; it must exist and be writable when a run starts). Such a run uses
  `runner.default_model` and `runner.scoped_allowed_tools` and logs `default_workspace_used`. A
  bound forum never falls back to it, and neither does a channel the runner does not operate:
  a guest run there (09 3.6) ends with the cause `channel_not_served`, bound or not. A text
  channel the runner operates without a binding is treated like such a forum. The default is
  empty (off). Note that `"."` gives every such chat the whole projects root, with write access
  to every project folder; a dedicated folder such as `"scratch"` keeps them apart.
- **The cause in the thread.** A run the runner ends without a turn, or with `adapter_error`,
  carries the cause as the `summary` of `finishRun`, one or two sentences with the log code in
  backticks, for example "This forum has no project folder, so Claude Code has nowhere to work.
  Bind a folder to the forum in its settings, or set runner.default_workspace on the runner host
  for forums without a folder (`project_not_bound`)". The app shows it next to the reason. The
  texts carry only codes and key names, never a path of the host.

```toml
[runner]
projects_root = "/srv/projects"
default_workspace = "."
```

## Channel and binding changes while running

The runner loads its served channels at start and follows changes without a restart (09 18.1):
`agent.config_changed` reloads the named channel; `access.changed` (the only event of an
operator change, 04 8.2), `channel.updated`, `space.updated`, `channel.archived` and
`space.archived` reload the whole list; and every `runner.channels_resync_interval` (default
`5m`) the list is loaded again as a fallback. When a bound forum joined or left the list or a
readiness changed, a heartbeat goes out at once, so a forum that just got the runner as operator
is reported ready right after the change. Log codes: `channels_resynced` (cause and whether anything changed) and
`channels_resync_failed` (the cache stays as it was).

## Search assistant (09 18.17)

When the server names this runner's agent in `search.assist.agent_id`, every question of the
search assistant reaches it as `search.assist_requested`. The runner answers with one
`claude -p` call without tools (`--output-format json`, `--no-session-persistence`, the fixed
instruction `prompts/search_assist.md` as system prompt, the question and the candidates as a
data block on stdin), with the model `runner.search_assist_model` (default `haiku`) and the time
limit `runner.search_assist_timeout` (default `9s`, below the server's wait). It runs with the API
key when `runner.anthropic_api_key_ref` is set, else on the personal login when
`runner.allow_personal` is on, else it answers nothing. The reply becomes picks of the sent
references and further search terms; the usage goes back as costs, which the person sees as an
estimate. A failed call logs `search_assist_failed` with a code and sends nothing; neither the
question nor a candidate nor the reply is logged.

## Builder of the interview (26 6.2)

With `runner.agent_builder = true` (default `false`) the runner answers the interview of "New
agent" when the server names its agent in `agents.builder_agent_id`: only that agent receives
`agent.interview_requested`. Each request is one `claude -p` call without tools: the flags of the
class `chat` (`--tools ""`, `--disallowedTools "mcp__*"`), `--output-format json`,
`--no-session-persistence`, the model `runner.agent_builder_model` (default `sonnet`), the fixed
instruction `prompts/agent_builder.md` as system prompt and the interview so far, the template
catalogue and the definition limits as a data block marked as untrusted content on stdin. It runs
in a private folder below `runner.state_dir/agent-builder/`, removed afterwards, with the billing
rule of the search assistant, and within `runner.agent_builder_timeout` (default `80s`), never
past the request's `expires_at`.

Keep `runner.agent_builder_timeout` below the server's `agents.interview.timeout` (default
`90s`): the runner cannot read the server's key, and an answer after the server stopped waiting
is refused with `410`.

The runner parses the last JSON object of the reply and checks it before it sends anything: the
answer schema (one or two questions, or a draft with `name`, `handle`, `description` and an
optional `channel_suggestion`, no other member) and its own copy of the definition rules of 26
4.2 and 4.3 (`src/claude_code_runner/definition.py`, held equal to the server by the whole corpus
`tests/agent-definitions` in `tests/unit/test_definition.py`) for the class of the draft's
`settings.tool_class_wish`. An answer that fails sends nothing and logs `agent_builder_failed`
with a code (`invalid_answer` names the failed rule, never a value); the server then falls back
with `builder_timeout` and the apps offer the templates and the manual form. Neither a turn nor
the reply is logged. The builder holds no tool and writes no file; the cost of the call goes back
as `costs`, which the owner sees as an estimate.

## Voice messages (09 18.14)

Claude Code cannot hear audio. With `runner.transcribe_cmd` set, every audio attachment
(`audio/*`) of the triggering message that has no transcript of the server is given to that
program before the turn starts, and every audio attachment of a follow-up before its line is
written to the running turn (a later follow-up waits, so the order holds; stdin stays open
meanwhile, and a stop or the end of the run ends the transcriber): the runner appends the local
file path as the last argument, runs it without a shell, with stdin closed, stderr discarded,
the allow-list environment (no API key) and the time limit `runner.transcribe_timeout`, and
takes its stdout as the transcript. The text stands in the trigger part like a server
transcript (in a follow-up's line after its text) and is also stored on the server with
`submitTranscript` (only accepted for a voice message in mode `agent`; any refusal is only
logged). The heartbeat reports `capabilities.voice_input` true exactly when a transcriber is set.

Without a transcriber, or when it fails (`transcriber_unreachable`, `transcriber_failed`,
`transcriber_timeout`, `transcript_empty`), the runner posts a note in the thread that the voice
message was not transcribed and Claude Code received only the audio file (for a follow-up once
its line is written), and the turn (or the follow-up) gets the file entry marked
`not transcribed: <code>`.

Example with whisper.cpp: its `whisper-cli` reads WAV, so a small wrapper converts first.

```sh
#!/bin/sh
# /usr/local/bin/transcribe-audio <audio file>: prints the transcript on stdout.
set -eu
wav=$(mktemp "${TMPDIR:-/tmp}/voice.XXXXXX")
trap 'rm -f "$wav"' EXIT
ffmpeg -loglevel error -y -i "$1" -ar 16000 -ac 1 -f wav "$wav"
whisper-cli -m /opt/whisper/models/ggml-base.bin -l auto -nt -np -f "$wav"
```

```toml
[runner]
transcribe_cmd = ["/usr/local/bin/transcribe-audio"]
transcribe_timeout = "5m"
```

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
| `system/task_progress` `workflow_progress` | one `subagent` step `workflow_agent` per agent of the workflow, title its label, under the workflow step, `detail` `phase`, `last_tool` and `usage`; `done` ends it, the end of the workflow ends its open agents; the prompt and result previews are never sent. A foreground command of a workflow agent (`owned_by_subagent` with an unknown `tool_use_id`) gets no step |
| `system/background_tasks_changed` | open `background_job` steps outside the set end `completed`; `/metrics/background_jobs` |
| `system/status` | `compacting` sets `/status_line`, its absence removes it |
| `system/api_retry` | `/metrics/api_retries`, `/metrics/last_api_retry` |

The run graph fields (`tool_use_id`, `task_type`, `usage`, the workflow type `local_workflow`)
were read from the files of the pinned CLI 2.1.259 without a session (ruling D-M2-51-1) and read
again in 2.1.288 (`tests/fixtures/recording/stream_fields_2.1.288.json`, D-RUNNER-CLI-PIN-1); the
level-one switch test holds them against the installed CLI.

## Follow-ups, stop and error paths (M2-35)

- **Follow-ups** (`capabilities.followups = steer`): a follow-up is read with `getMessage` and
  written to the running turn's stdin as one `user` line, `Follow-up from <name> (@<handle>):`, a
  blank line and the text, with its own `uuid`; the runner patches `delivered`, and `absorbed`
  once Claude Code echoes the line (`--replay-user-messages`). Before its files are stored and
  again at the moment of writing the token is checked (expiry, judged at the time the open turn
  took the follow-up up, mode, tool class, run, `trigger_kind`, author and thread); a failed
  check stores, transcribes and writes nothing and never ends the run. Follow-ups of a run that
  has not started are held and written after the prompt line. After a `result` stdin stays open for
  `runner.followup_grace` while written follow-ups have no echo.
- **Background work** (09 18.5, FIX-runner-responsive-turns): a turn may end while work it
  started in the background (Bash or a subagent with `run_in_background`, a workflow) still
  runs. Claude Code stops that work when its stdin ends, so the runner keeps stdin open while the
  stream lists a background task, and for a process that had one also during a turn and for
  `runner.followup_grace` after the last line (log code `stdin_held_for_background`). A
  `result` while the input line is still queued in Claude Code (`command_lifecycle`, for
  example the empty turns for tasks a previous process lost) neither closes stdin nor starts
  the follow-up window, and an input that starts waits for its own `result`
  (FIX-runner-hold-background). Meanwhile
  every follow-up becomes a turn of its own with its own answer, the answer Claude Code gives
  when the work ends is posted in the thread, and the run stays `running` with progress patches
  against `run.stall_after`. The runner prompt asks Claude Code to start long work in the
  background, end the turn with a short reply and never wait for background work inside a
  turn; nothing enforces that. A stop ends the process and its background work. Since
  FIX-runner-reachable-2 the prompt also says that any other instruction to keep a turn open
  or wait inside it (user instructions files loaded into every session on the host, memories,
  rules for headless sessions, summaries of the conversation, hooks) does not apply here, that
  a turn ends within about a minute and that a status question is answered at once. A
  follow-up written during a turn reaches the model after the running tool call returns. A
  turn that fails (for example a notification turn while the host's login is briefly gone) no
  longer holds the run for the background work: a process still alive `runner.stop_grace` after
  an error `result` is stopped (log code `turn_stopped_after_error`), its open task steps end
  `cancelled`, and the run ends with a summary that names the lost tasks
  (FIX-runner-auth-failed-held). Once the process has ended nothing waits on the thread any
  more, also when the last answer promised to go on later: a run its turn completes carries
  the summary "Run ended; nothing is waiting ..." with the code `run_ended_idle`, which the
  apps show next to the run's state on its collapsed line and in the opened box
  (FIX-runner-silent-stop).
- **Attachments of follow-ups** (09 18.5, FIX-scan-state-stale): a follow-up whose files are
  still being checked waits, without a lock, until the server reports every file final, at most
  `runner.followup_file_wait`; its line then lists each file after the text under
  `Attachments:` as `local file: <path>` (downloaded into the thread's files folder), or
  `unavailable: file_blocked` or `unavailable: file_not_ready`. A follow-up of files only is
  written too.
- **Stop** (`run.cancel_requested`): SIGINT to the turn's process group and stdin closed, SIGTERM
  after `runner.stop_grace`, SIGKILL after `runner.kill_grace` (log code `stop_escalated`); the
  run finishes `cancelled` with the request's reason only when no process of the group is left,
  without an answer. A run that has not started is never started. A run the server ended
  (`409 conflict` on a patch, or `401`) has its turn stopped and is not finished.
- **Error paths** (09 18.9): a full context is compacted once (`/compact` as the only input line
  of a process with `--resume`) and the same prompt runs again, with a retry hint in the runner
  prompt when the turn had worked; a missing session is replaced by a new one and reported (with
  `bind_failed` in mode `bind`), fail open; a session that exists at the first turn is resumed; a
  missing login ends `runtime_not_authenticated` (at a turn with a summary that says the login
  was lost and that the next message continues the same session); a CLI too old for the model ends
  `runtime_error` with the required version in the last step and degrades the runner (see the
  operator rules); an error `result` ends `runtime_error` with its texts in the last step. Box notes are run steps of kind `status` with the codes
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

## Turn handover (09 18.19.3 to 18.19.7, AR-03)

Off by default (`runner.turn_handover = false`); without the key the runner behaves exactly as
before, held by the byte-equal fixtures of `tests/fixtures/baseline` (AR-T16). Switch it on only
against a server that accepts the heartbeat capability `turn_handover` and has
`run.launch.check` (16 10.1); the runner then reports `turn_handover: true`.

- **Keeper.** Every process of a run (an attempt) runs under `python -m
  claude_code_runner.keeper`, started with `start_new_session=True`, so keeper and CLI live in
  their own session and process group outside the service. The keeper opens the attempt's named
  pipe (`<launch>.stdin`) for reading, then for writing (it holds that write end for the turn's
  life), forks the CLI, names it in `<launch>.cli` before it releases it, writes the attempt's
  initial inputs once without blocking and then `<launch>.fed`, ignores SIGINT, SIGTERM, SIGHUP and
  SIGPIPE, closes its write end on SIGUSR1, writes the CLI's wait status to `<launch>.exit` and
  exits with the CLI's code (128 plus the signal number when a signal ended it). It holds no lock
  descriptor after the fork. The pinned CLI does not end its input when the last writer of a named
  pipe closes (macOS, 09 18.19.12), so the keeper relays the named pipe into an anonymous pipe that
  is the CLI's stdin and closes that pipe when the named pipe's last writer is gone.
- **Launch files.** `<launch>` is `<runner.state_dir>/turns/<run_id>.<attempt>.<try>`:
  `.inputs` (the initial input lines), `.spec` (the CLI's argument list, environment, folder and
  files, mode `0600`; the keeper removes it at once, because the environment can hold the API key),
  `.stdin`, `.keeper`, `.cli`, `.exit` and `.fed`; `turns/<run_id>.lines` keeps the run's input
  lines for a next runner. All of them go when the run ends.
- **Handshake.** The record is written before each step: phase `launching` with the attempt
  (number, kind `first`, `compaction`, `retry`, `resume_same_reference` or `new_session`, try,
  initial input ids, recovery flags, outage number, `process_start`), then the keeper's identity
  with `go_sent`, and only then the go byte; the CLI's identity in phase `process`. Follow-ups
  wait for `<launch>.fed`. A next runner settles a `launching` record by the four cases of 18.19.5
  (`launch_settled`).
- **Phases.** The record carries `preparing`, `waiting_lock`, `transcribing`, `launching`,
  `process` or `compaction_retry`. A next runner removes a `preparing` or `waiting_lock` record (the
  inbox restart starts the run again with one `run.start`), starts a `transcribing` run over,
  launches a recorded retry, and adopts or relaunches by the cases above.
- **Handover.** SIGTERM takes no new run, sends no further heartbeat, finishes an input line in
  flight within `runner.kill_grace` or records its byte count (`partial_input`), closes a launch
  pipe whose go byte is not recorded, logs `turn_handed_over` per turn and exits 75 when at least
  one turn was handed over, else 0. Hosted agents' turns are stopped as before.
- **Adoption with full rights.** The adopter replays the stream lines from `process_start` to the
  stored offset into `ProcessState` with the adoption time (nothing is posted or patched again,
  `turn_replayed`), then reads on, takes follow-ups and further inputs, applies the stdin rule and
  the stop after an error, and handles the end like its own (classification from the stream and
  `<launch>.exit`, then compaction or session recovery within the once-per-run limits the record
  proves). Before any launch of a run it did not accept it calls `run.launch.check`
  (`launch_check_allowed`, `launch_check_refused`, `launch_check_unreachable`). A turn whose stdin
  was closed and a compaction are adopted with reading rights only; a record without a named pipe
  is adopted as before.
- **`claude-code-runner stop --turns --config <path>`.** Stops every turn on purpose: a running
  runner (named in `<runner.state_dir>/runner.pid`) gets the marker `stop-turns` and SIGTERM and
  stops its own and adopted turns as in 18.6; the command then stops the turns of the records left
  itself, each run ending `cancelled` with `adapter_stopped`, and exits 0. Run it before unloading
  or uninstalling the service, and before switching `runner.turn_handover` off.
- **Switching the key off.** A runner without `runner.turn_handover` cannot continue a turn a
  runner with the key left behind, so `run` refuses to start while a turn record that carries a
  phase is left (exit 1, `start_failed` with `turn_handover_records_left`) and touches nothing;
  `stop --turns` stops such turns through their keeper also when the key is already off
  (`handover_records`, 09 18.19.1, issue 567).

## Costs

While a run works, `livecost.py` keeps its `/cost` current (FIX-cost-display): every `assistant`
message adds its `message.usage` priced with `prices.json`, a byte copy of the server's start
prices (`services/server/internal/billing/domain/startprices/start_prices.json`, a unit test holds
both equal); a message counts once with its newest usage, a model without a price adds nothing,
and every `result` settles the value on the exact amount the run would book at that point. The
patcher throttles the patches; `run.finish` books the reports below and the server replaces the
run's cost with the booked sum.

The runtime reports cumulative values per session (`result.modelUsage`), also across
`--resume`. The runner keeps the last values of each session in
`<runner.state_dir>/sessions/<ref>/usage.json` and reports per run only the difference
(docs/spec/09-agents.md 18.7). On `--resume` the CLI restores the session's totals from the
last `cost-state` line of its transcript (`<config dir>/projects/<folder>/<ref>.jsonl`, the config
dir being `CLAUDE_CONFIG_DIR` of the turn or `$HOME/.claude`) and counts on from there. Before
the first process of a run that resumes, the runner reads the same line and makes it the stored values
(log event `session_totals_taken`), so a run books only its own turns, in amounts and in tokens:
also the first run of a session taken over from an import or started outside the runner, and a
run after the session was continued outside the runner. Without such a line the stored values
stay as they are. A difference below zero is never booked (the counter-reset rule below). A
session bound with a cost baseline (an imported session or a
rebind, `AgentSession.cost_baseline_micros` with `currency`, docs/spec/22-import-migration.md 9
step 7) already holds amounts the old environment booked. When the runner has no stored values
for such a session and the transcript has no `cost-state` line (the transcript's totals take
precedence, they hold the same old amounts per model), it keeps a remaining baseline in `usage.json` and subtracts it from the
amounts of the next runs, shared across the models in proportion to their amounts and never below
zero per model, until it is used up; a reset of the runtime's counters (any value goes down)
clears it. The rule applies in every session mode, not only `bind`: a first run after a binding
that ended before its values were stored would otherwise book the old amount on the next
`resume` (docs/decisions.d/M8-06.md). A baseline in another currency than US dollars is ignored
with the warning `cost_baseline_currency_ignored` (no conversion). Tokens have no baseline in the
data model, so only in that case the token counts of the first run after a binding can include
the old session's tokens.

## Running

```sh
cd plugins/claude-code-runner
uv run --locked claude-code-runner check-config --config /etc/claude-code-runner.toml
uv run --locked claude-code-runner run --config /etc/claude-code-runner.toml
```

`check-config` contacts no server; it prints one code per check (`ok`, `unset` or the problem).
`run` logs JSON lines on stderr (event codes and IDs, never content, tokens or keys) and stops on
SIGINT or SIGTERM. Before it stops, every turn it started itself is ended as a stop by a person
ends it (SIGINT, SIGTERM and SIGKILL to its group, each escalation logged as `stop_escalated`), and
its run finishes `cancelled` with the reason `adapter_stopped` and a note in the thread, which
names the background tasks the stop ended (their results are lost; FIX-runner-silent-stop); the
turn record goes only once no process of the turn is left. With `runner.turn_handover` a restart
hands the turns over instead and no background task is lost. Turns adopted after a crash keep running and
keep their records for the next start (docs/spec/09-agents.md 18.6, 18.9).

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

## Slots and wait reasons (F-WARTESCHLANGE-GRUND)

Every heartbeat carries `run_slots`: the turns that hold a slot now and `runner.max_parallel_runs`
(docs/spec/09-agents.md 4.8, 18.2). A run that waits in prepare is listed in `waiting_runs` with
`thread_busy` while another turn holds its thread or session lock and `slots_full` while every
slot is in use; a heartbeat goes out at once when a run starts or stops waiting, so the app
shows why the run waits and which runs hold the slots (09 5.2.1). Raising
`runner.max_parallel_runs` is the operator's remedy for long `slots_full` waits.

## Hosting built-in agents (A1-04)

With `runner.hosted_agents = true` (default `false`) the runner also serves the built-in agents
whose hosting runner is its own agent (docs/spec/26-builtin-agents.md 3, 09 18.18). Without the
key nothing of this runs: no hosted list, no further connection, and the command line, heartbeat
and served channels are byte for byte those of the runner before stage A1 (the fixtures in
`tests/fixtures/baseline/`).

- **Keys.** `runner.hosted_agents`; `runner.hosted_max_tool_class` (`chat`, `scoped` or `host`,
  default `scoped`), the highest class a hosted agent runs with, reported in the runner's own
  heartbeat as the capability `hosts_agents`, so the server refuses a higher class for an agent of
  this runner; `runner.hosted_agents_root` (see "Turns of built-in agents"). `runner.hosted_agents`
  needs `runner.dpop_key_ref`, because every hosted token is bound to the runner's DPoP key.
- **The set.** Once the runner's own connection is ready and its heartbeat with `hosts_agents`
  is stored, the runner lists the agents it hosts (`listHostedAgents`), again on
  `agent.hosting_changed`, on `agent.updated` of a hosted agent and every
  `runner.channels_resync_interval`. A listed agent joins, an agent no longer listed is stopped
  (its connection closed); log lines `hosted_agent_joined` and `hosted_agent_left` name the
  agent ID only.
- **Tokens stay in memory.** For each hosted agent the runner fetches a token with
  `issueHostedAgentToken` with its own token and key. The token lives only in the runner's
  memory: never in a file, a log line, a turn record, an environment or anything the runner
  writes; a restart fetches new ones. The runner fetches the next token when the share
  `TOKEN_REFRESH_FRACTION` (`hosting.py`) of the token's lifetime (the server's
  `agents.hosted_token_ttl`) is left (`hosted_token_refreshed`);
  the server revokes the old one, and the agent reconnects with the new one. A failed fetch logs
  its problem code (`hosted_token_issue_failed`) and is tried again with the SDK's backoff. A
  connection closed with `4012` (the agent was disabled, moved to another runner, or a token of
  this runner was revoked) drops the agent until the next list names it again.
- **What lives where.** Each hosted agent has its own gateway connection with the consumer
  `default` of its own principal, its own cursor, channel cache and heartbeat, and its state in
  `<runner.state_dir>/hosted/<agent_id>/` (mode `0700`): inbox, outbox and thread locks. The
  runner's own state stays where it was. A hosted heartbeat carries the agent's
  `reported_tool_class` (the lower of its class and `runner.hosted_max_tool_class`), the runner's
  runtime state and billing modes and its capabilities without `hosts_agents`.
- **Shared slots.** `runner.max_parallel_runs` bounds the turns of the runner's own agent and of
  every hosted agent together. Each agent's heartbeat names its own waiting runs and the shared
  `run_slots`, so a person sees that turns of another agent hold the slots.
- **Without a definition** a hosted run takes its locks and a slot like a turn, then ends
  `failed` with `adapter_error` and the cause `definition_missing` in the summary, without
  `run.start` and without a turn (26 5.6 item 4).
- **Turning it off** takes the hosted agents offline (their runs end like those of any offline
  agent) and leaves the runner's own agent as before (26 11 item 4).

## Turns of built-in agents (A1-05)

A hosted agent's turns run from its definition (docs/spec/26-builtin-agents.md 4 and 5). The
server stores the definition as closed data; the runner writes every file the CLI reads from it,
never copying a file of it, and checks it itself before every turn.

- **Root.** `runner.hosted_agents_root` is required with `runner.hosted_agents` and created with
  mode `0700` when missing. The start is refused, naming the key, when it is equal to, inside or
  above `runner.state_dir` or `runner.projects_root`, when it or any folder above it holds a
  `CLAUDE.md` (it would reach every agent's prompt), or when group or others may write it.
- **Folders.** Per agent `<root>/<agent_id>/` (mode `0700`), the working folder of every turn,
  and next to it, outside the turn's reach below `host`, the control folder
  `<root>/<agent_id>.control/`.
- **Files.** `CLAUDE.md` (the instructions) in the agent folder, and under `host` also
  `.claude/settings.local.json` (the turn reads `--setting-sources project,local`); in the control
  folder `settings.json`, `plugin/.claude-plugin/plugin.json` (name `bothy-agent-<agent_id>`, the
  definition version, nothing else), `plugin/skills/<name>/SKILL.md` (frontmatter with exactly
  `name` and `description` as double-quoted scalars, then the body) with the skill's extra files
  flat beside it, and `mcp.json` with the servers of the class (none under `chat`, `http` ones
  under `scoped`, all under `host`) and every `secret_ref` unresolved.
- **Settings.** Exactly `disableSkillShellExecution`, `disableAllHooks`, `disableWorkflows` and
  edit and write deny rules (`Edit(<pattern>)`, `Write(<pattern>)`) for `CLAUDE.md`,
  `CLAUDE.local.md`, `.claude/**`, `.mcp.json`, `.lsp.json` and `.claude-plugin/**` (26 5.4).
- **Rebuild before every turn**, on start of the agent's connection and on
  `agent.definition_changed`: a symbolic link anywhere in both folders is removed
  (`hosted_link_removed`); every `.claude`, `.claude-plugin`, `.mcp.json`, `.lsp.json` and
  `CLAUDE.local.md` the runner did not write is removed at any depth of the agent folder
  (`hosted_surface_removed`); the agent's other files stay; the control folder keeps only what
  the runner wrote. Owned files are written atomically, so a concurrent turn of the same agent
  never reads half a file.
- **Own check.** `definition.py` repeats the rules of 26 4.2 and 4.3 (the server's size limits
  are server keys the runner cannot read and are left out) with `runner.mcp_stdio_runners` as the
  allowed `stdio` commands and the class of the invocation token. A finding, or a `stdio` server
  under a token class below `host`, ends the run `failed` with `tool_class_mismatch` before
  `run.start`; the summary names the field codes, never the content
  (`hosted_definition_refused`).
- **Order of a turn.** Locks and a shared slot; the definition (`getAgentDefinition` with the
  hosted token); the check; the rebuild; the billing pre-check; `run.start`; the per-turn MCP file
  `<control>/turns/<run_id>.mcp.json` (mode `0600`) with every `secret_ref` resolved; the turn in
  the agent folder; the file is deleted when the turn ends, also on a stop, and leftovers of a
  crashed runner are swept at start. A secret that cannot be read, or one out of scope, ends the
  run `adapter_error` with the cause `mcp_reference_unavailable` and writes no file. Secret values
  are never in the environment of a turn, a log line or another file.
- **Secret scope** (26 5.1 item 3). A definition's `secret_ref` resolves only as
  `file:<runner.hosted_secrets_dir>/<agent_id>/<name>`, also after following links, and never to
  a credential of the runner (`agent_token_ref`, `dpop_key_ref`, `anthropic_api_key_ref`); `env:`
  references and other agents' subfolders are out of scope. The host operator places each
  agent's secrets there (mode `0600` or `0400`). Without the key no reference of a definition
  resolves. The start is refused, naming the key, when it is not an absolute folder (a link is
  refused), when group or others may write it, when it overlaps `runner.hosted_agents_root`,
  `runner.state_dir` or `runner.projects_root`, or when a `file:` credential of the runner lies
  in it.
- **Command line** (26 5.3): `chat` gets `--tools ""` (or `Skill` with `--allowedTools Skill` when
  the definition has skills); `scoped` gets `--tools Read Grep Glob Bash [Skill] <tools>` and
  `--allowedTools <tools> [Skill]`, where `<tools>` is `runner.scoped_allowed_tools` narrowed by
  the definition's tool list in the order of the key; `host` keeps `bypassPermissions`. Directly
  after `--permission-prompts none`: `--restricted` (below `host`) or `--setting-sources
  project,local` (`host`), `--settings <control>/settings.json`, `--plugin-dir <control>/plugin`
  with skills, `--mcp-config <turn file>` with servers, always `--strict-mcp-config`. The cost
  cap `settings.max_cost_usd_per_run` becomes `--max-budget-usd` before
  `--append-system-prompt-file`; a run it stops ends `runtime_error` with the cause
  `cost_cap_reached` (the CLI's `result` subtype `error_max_budget_usd`, or a cost at or above
  the cap).
- **State.** A hosted agent's turn records, sessions, streams, attachments, scratch places and
  (billing `organization`) CLI configuration live under `<runner.state_dir>/hosted/<agent_id>/`.
- **Channel skills** (26 5.9). With `runner.channel_skills_dir` set, a channel's skill reference
  `claude-code:<name>` loads `<channel_skills_dir>/<name>/` (a folder with a `SKILL.md`) as one
  more `--plugin-dir`, only in a turn of class `host`, for the runner's own agent and for hosted
  agents. Below `host` every such reference is skipped, logged (`channel_skill_skipped`) and named
  once per run in the live box status line; a missing folder is named the same way. References
  of another adapter are skipped with a debug line. Without the key no reference is read.
  Before setting the key, know who can bind a skill: any member with `manage` on a channel can
  bind a channel skill there, and it then acts in every `host` turn an owner starts in that
  channel. There it acts with that turn's rights (the host user's), which can exceed the
  member's, and its content is not the owner's choice; binding grants the member no Bothy
  permission. Keep in `channel_skills_dir` only skills you would run in such a turn.

### Probe results of 26 5.7 (pinned CLI 2.1.288)

`tests/real_cli/test_hosted_probe.py` has one probe per row of 26 5.7. Level one (free) ran on
2026-11-06 against 2.1.288 and passed: `--help` names every flag of 26 5.3 (`--tools` takes
several values), and the CLI holds `disableSkillShellExecution`, `disableAllHooks`,
`disableWorkflows`, the result subtype `error_max_budget_usd` and the placeholder "[shell command
execution disabled by policy]". Level two (one real turn per row, costs money) has not run: it
needs the owner's `CLAUDE_CODE_RUNNER_REAL_TURN=1`. Until it runs, every row of 26 5.7 is not
proven, no fallback is taken, and the runner uses the expected behaviour of each row. The cost
cap is recognised by the result subtype read from the binary and, as the fallback of the last
row, by a cost at or above the cap.

## Project folder report (M4-44)

Every heartbeat carries `project_folders`: the names of the Git work trees directly under
`runner.projects_root` (docs/spec/09-agents.md 18.13). A folder without Git is still ready when
it is bound by name; the list only offers Git work trees, because the app checks every listed
folder for a new forum. An entry counts when its name is one
segment of letters, digits, `.`, `_` and `-` that starts with a letter or digit (so hidden folders
and the `.provision-<uuid>` folders never show), it resolves like a `workspace_ref` to a folder
under the root and it holds `.git`. The names go out in code point order, at most
`runner.project_folders_max`; more are cut and logged as `project_folders_truncated` with the
count, and a root that cannot be read sends the empty list and logs `project_folders_unreadable`.
Only names leave the runner. The app lists those without a forum under "Add forums for existing
folders" and binds the chosen ones with `provision` `none`.

## Moving a project folder to the trash (09 18.15)

When a person deletes a project forum with "Also move the project folder to the trash", the
server hands the folder to its runner once the deletion is final. The runner never removes
anything: it moves the folder with one rename into the trash of its host (`~/.Trash` on macOS,
the freedesktop trash `$XDG_DATA_HOME/Trash` or `~/.local/share/Trash` elsewhere, with a
`.trashinfo`), or into `.bothy-trash/<name>-<UTC time stamp>` under `runner.projects_root` when
that trash is on another file system or not writable. A person restores it from there.

- Every `runner.trash_check_interval` (default `60s`) the runner checks each bound folder and
  reports `trash_check` in the readiness entry; the app disables the box with the reason when the
  check is not ok. Refused are a reference outside the root (`outside_root`), a symbolic link as
  the bound path or a component of it (`symlink`), a missing folder (`workspace_missing`), a
  folder holding another bound folder or the state directory (`contains_other_project`), a linked
  work tree, submodule or folder of another repository (`foreign_worktree`), uncommitted or
  untracked changes or stashes (`uncommitted_changes`), commits on a local branch no remote branch
  contains (`unpushed_commits`) and any Git error or time out (`check_failed`). A folder without
  Git has no Git rule. `git` runs with the time limit `runner.cli_probe_timeout`.
- The request arrives in the answer of `sendHeartbeat`, which the runner sends every
  `runner.trash_check_interval`. The runner stops a turn still running in the forum, checks again,
  moves the folder and reports `trashed` or `refused` with the code in its next heartbeat. On
  Windows the folder always goes to `.bothy-trash` under the projects root, never to the recycle
  bin. A move is also refused with `move_failed` (the rename failed, or `.bothy-trash` is a link).
- `<runner.state_dir>/trash.json` remembers each handled request and its result, so a restart
  neither repeats a move nor loses a result. Log codes: `project_trash_moved`,
  `project_trash_refused`, `project_trash_deferred` (a turn did not end in time; the request
  waits for the next answer) and `project_trash_failed`; never a path or a folder name.

## Long answers (M4-47)

An answer is never split into several messages (docs/spec/01-product.md 6.13, 09 18.10). An answer
text longer than `LIMIT_READ_FLOOR_BYTES` bytes reads `message_max_content_bytes` with
`getInstance`; above it the message keeps the head (at a line break, never inside a character or a
fenced code block) and the rest goes as the last attachment `answer.txt` of the answer's set,
staged and uploaded like an `ATTACH:` file. When the set fails, a shorter head goes out with the
notice naming `answer.txt`, so both fit the limit together.

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
  `getInstance` (`message_max_attachments`, `file_max_upload_bytes`; only files that pass count,
  the first ones go). Accepted files are copied with `O_NOFOLLOW` and an inode check into
  `<runner.state_dir>/outgoing/<run_id>/` (mode `0600`) and uploaded from the copies, one after
  the other, through `createUpload`, `uploadPart` and `completeUpload` inside the run binding
  (07 SR9); `/status_line` is `uploading_attachments` meanwhile.
- Best effort (FIX-attach-partial-and-roots): every file that passes its checks and uploads goes
  with the answer; a refused file never stops the others. The answer then ends with the notice
  `Attachments not sent:` and one line per refused file in line order (a German heading and
  readable German reasons when the instance's `locale_default` is German), its base name and a
  code of the closed set `not_found`, `outside_allowed_roots` (with the hint to copy the file
  into `TMPDIR` and attach the copy), `denied_pattern`, `not_a_regular_file`, `hard_link`,
  `empty_file`, `changed_during_check`, `too_large`, `too_many`, `limits_unavailable`,
  `upload_refused`, `upload_failed`, `not_attached` (and `set_not_sent` in records of an older
  runner). When no file is left, or the server refuses the attachments (`403 feature_disabled`,
  `403 forbidden`, `409 conflict` with `file_not_attachable`), the answer goes out without
  attachments and with the notice. A refused file never fails the run.
- Files from elsewhere: the roots stay narrow; the runner prompt tells Claude to write files for
  the person into the project directory or `$TMPDIR` and to copy a non-secret file from anywhere
  else into `$TMPDIR` before it attaches the copy.
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

The end-to-end suite `tests/realserver` (pytest marker `e2e`, left out of the command above) runs
the runner with Fake-Claude against the server of this tree (`services/server/test/sdkserver`)
as Lotse with tool class `chat` and the default workspace `website`: a forum post, a mention in
the main thread answered in the auto thread, a reply that resumes the session, a follow-up
into the running turn, an answer attachment, a voice message with a local transcriber, cancel
and retry. It needs the Go tool chain and a container runtime (`make adapters-e2e` from the
repository root).

`tests/unit/test_byte_equal_baseline.py` compares the runner without the keys of stage A1 with
the fixtures in `tests/fixtures/baseline/`; their README says how they were recorded and that they
change only with a ruling. `tests/unit/test_hosting.py` and `tests/scenarios/test_hosted_agents.py`
run a hosting runner against `tests/hosting_world.py`; `tests/realserver/test_hosting.py` hosts
one built-in agent against the server of this tree (token issued, connection open, a message to
the agent ends with `definition_missing`). `tests/unit/test_definition.py` runs the whole corpus
`tests/agent-definitions` through the runner's own check; `tests/unit/test_materialize.py`,
`tests/unit/test_hosted_commandline.py`, `tests/unit/test_channel_skills.py` and
`tests/scenarios/test_hosted_turns.py` hold the folders, the command line, channel skills and
hosted turns of A1-05.

Every test uses Fake-Claude through `runner.claude_bin` and a fake `HOME`; no installed CLI and
no login of the developer is touched. The real CLI runs only behind switches:

- Level one, free and without credentials: `CLAUDE_CODE_RUNNER_REAL_CLI=1 uv run --locked pytest
  tests/real_cli` checks the installed version against the pin and the flags of `--help`, and
  that every recognised error text of `ERROR_TEXTS` and every stream type and member the runner
  reads occur in the files of the installed CLI (`test_pinned_cli.py`); a missing CLI fails. The
  pinned CLI can be installed into a temporary folder from the npm package of the platform
  (`@anthropic-ai/claude-code-<platform>` at the pinned version) without a login.
  `test_named_pipe.py` runs the pinned CLI with a named pipe as stdin against a local stand-in for
  the API on loopback (no login, no model call), directly and under the keeper (09 18.19.12).
- Level two, additionally `CLAUDE_CODE_RUNNER_REAL_TURN=1` and `CLAUDE_CODE_RUNNER_REAL_KEY_FILE`:
  `auth status` per mode, two short real turns (one `--resume`) and one turn with a background
  command that checks what 09 18.5 "Background work" relies on. It uses the operator's key or
  login and costs money; running it is the owner's decision.
- `tests/real_cli/captured/api_errors_<pinned version>.jsonl` holds the `assistant` and `result`
  lines the pinned CLI wrote against a local stand-in for the API (`ANTHROPIC_BASE_URL` on
  `127.0.0.1`, a fake key, an empty `HOME`; no model call): a prompt above the context window
  (400), a refused key (401) and a CLI below the model's minimum (400). `test_failures.py`
  classifies them. A change of the pin captures them again with the new version.

## To check before the build

- Does `system/init` come before the first model request with `--input-format stream-json`?
  Open; until it is checked with level two, the pre-check is binding and the counter-check at
  `system/init` interrupts the turn with SIGINT.
- Is `/compact` available in print mode of the pinned version? Yes: the command `compact` of
  Claude Code 2.1.259 has `supportsNonInteractive: true` in the files of the pinned CLI
  (retrieved 2026-10-24, again in 2.1.288 on 2026-10-29), and the Agent SDK page on slash commands sends `/compact` as a prompt
  and reports it with a `system` message `compact_boundary`
  (https://code.claude.com/docs/en/agent-sdk/slash-commands, retrieved 2026-10-24). That the CLI
  takes it as the content of a stream-json `user` line is not run with a real session yet (level
  two would need a paid run).
