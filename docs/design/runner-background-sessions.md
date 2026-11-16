# Claude Code runner on background sessions

Status: design of 2026-11-16, built by the ticket FEAT-RUNNER-BG-SESSIONS. This note records
the goal, the evidence and the decisions with their reasons, and ends with the build brief. The
normative text is in [09-agents.md](../spec/09-agents.md) sections 18 and 21 (the paragraphs
"Background mode") and the rulings are in `docs/decisions.d/FEAT-RUNNER-BG-SESSIONS.md`. Where
this note and a chapter disagree, the chapter wins. Section 9 lists where the build changed a
decision of the draft; the decisions below say what was built.

Why a design note: the change adds a second process contract to the runner
(`plugins/claude-code-runner`) and rests on a part of Claude Code that is in research preview,
so the decisions and what they rest on are written down before any code.

Sources. The runner at commit `45e09c3c7` (`plugins/claude-code-runner/src` and its README) and
09 sections 18 and 21. The public Claude Code documentation, retrieved 2026-11-16: the pages
agent-view, cross-session-messaging, sessions, cli-reference, permission-modes, permissions,
hooks and settings under `https://code.claude.com/docs/en/`. Claude Code 2.1.325, the version
this draft is written against (the runner's pin moves to it in FIX-RUNNER-CLI-PIN-325). Every
fact below carries its evidence:

- **[docs]**: a sentence of the public documentation.
- **[read]**: read in the help text or the file of Claude Code 2.1.325, not run.
- **[run A]**: run on 2026-11-16 with 2.1.325 on a subscription login, with a small model and
  short prompts.
- **[run B]**: run on 2026-11-16 with 2.1.325 against the local stand-in for the API
  (`tests/real_cli/stand_in.py`) with a configuration folder of its own: no login, no model
  call (section 2.4 says how).
- **[unproven]**: neither documented nor run.

## 1. Goal

Owner request of 2026-11-16: the runner starts every turn as `claude -p` (print mode, 09 18.2).
Claude Code documents a second unattended path, background sessions (`claude --bg`), and says
of it: "background sessions consume your subscription usage the same as interactive sessions"
[docs, agent-view, Limitations]. The runner shall be able to run its turns on that path, and
the path shall be tested.

What this design can and cannot decide: no public page says that print mode counts differently
from an interactive session, and none promises that a background session is cheaper than a
print turn. The design makes the path selectable, proves which path a turn took (transcript
lines of a background turn carry `"entrypoint":"cli"` and `"sessionKind":"bg"`, those of a
print turn `"entrypoint":"sdk-cli"` [run A, run B]) and keeps the runner's behaviour. The rate
at which either path draws a subscription is a measurement of the operator on their own
account (section 5 item 1).

Constraints: additive and off by default. An installation that does not set the new key
behaves exactly as before, held by the byte-equal fixtures of
`plugins/claude-code-runner/tests/fixtures/baseline`.

### 1.1 Done means

Each point is decided by a test of section 7.3.

1. Without `runner.launch_mode` the command line, the heartbeat and the served channels equal
   the baseline fixtures, and the runner never calls `claude --bg`, `claude agents`,
   `claude stop`, `claude rm` or `claude daemon`.
2. With `runner.launch_mode = "background"` a message in a forum of the runner's own agent in
   billing mode `personal` starts exactly one background session in the project folder. The run
   completes with the session's answer, the live box shows the model, the text excerpt and one
   step per tool call, the costs of the run are the difference of the session's totals, and the
   session of the thread is the one the CLI created. The next message in the thread continues
   the same session under the same id.
3. A follow-up written while a tool call runs is `delivered`, then `absorbed` in the same turn,
   and the turn's answer was produced after the model received it.
4. A stop ends the run `cancelled` without an answer and leaves no process of the session.
5. A turn that starts a background command ends, the run stays `running`, a follow-up gets a
   turn of its own, the answer to the command's notification is posted, and the run ends when
   nothing runs any more.
6. An account error in a process that holds or held background work keeps the process and its
   work, shows why the run waits, tries again by the schedule of `runner.account_retry_*` and
   posts the answer of the retry turn; the give-up path and a stop during the wait behave as in
   print mode (the scenarios of FIX-RUNNER-LIMIT-KEEPS-RUN, run in both modes).
7. A runner that is killed while a background turn runs is replaced by a runner that adopts
   the turn: the answer is posted exactly once, and a follow-up after the adoption is absorbed.
8. For every cause of section 4.13 the run completes in print mode, the live box names the
   cause, and the prompt reaches the model exactly once.
9. No value of a transcript record that holds account identifiers (section 2.2 item 12)
   appears in a file the runner writes, in a log line or in a fixture.

## 2. What Claude Code offers

### 2.1 The documented path

- A background session is started with `claude --bg`, runs under a supervisor process, is listed
  by `claude agents --json`, stopped with `claude stop <id>`, removed with `claude rm <id>` and
  continued with `claude --resume <session-id> --bg`. "Claude Code rejects `--bg` combined with
  `-p` or `--print`" [docs, agent-view].
- "`claude agents --json` is the supported way to read session state from outside Claude Code,
  for example from a status bar, a scheduler, or another Claude session that supervises
  background work." [docs, agent-view]. "The files under `~/.claude/jobs/<id>/` are not a
  stable interface." [docs, agent-view].
- Transcripts: "The entry format is internal to Claude Code and changes between versions, so
  scripts that parse these files directly can break on any release." [docs, sessions].
- "Agent view is in research preview. The interface and keyboard shortcuts may change as the
  feature evolves." [docs, agent-view].
- No documented command sends a message to a running background session from a script, and no
  documented interface streams its reply. Cross-session messaging delivers text from one
  session to another over a per-session socket: "The receiving Claude reads the message between
  tool calls during an active turn." and "Claude Code tells B's Claude that the message came
  from another session, not from you, and limits what the message can do" [docs,
  cross-session-messaging]. The page describes the socket for "a script or hook" that posts
  into a session and does not give the format of a message line.

### 2.2 What the runs showed

| # | Fact | Evidence |
|---|---|---|
| 1 | `claude --bg <flags>` without a terminal returns at once with exit 0 and prints `backgrounded · <short id> · <name>` on stdout (and `Starting background service…` on stderr when no supervisor ran). The short id is the first eight characters of the session id; the full id is the member `sessionId` of the session's entry in `claude agents --json --all` | run A, run B |
| 2 | With no positional prompt the text on stdin of that command becomes the first user message. A prompt of several hundred kilobytes with line breaks, non-ASCII characters and a line that starts with `/` arrived unchanged, also together with `--resume` | run B |
| 3 | `--session-id` is ignored with a warning: the CLI makes the session id. `--resume` with an id no session has prints the same line and exits 0, but the entry is `failed` at once and has no process | run A, run B |
| 4 | Honoured with `--bg`: `--name`, `--model`, `--permission-mode`, `--tools`, `--disallowedTools`, `--append-system-prompt-file`, `--mcp-config` with `--strict-mcp-config`, `--add-dir`, `--effort`, `--settings` as a file or a JSON text (its `env` block, `crossSessionInbound` and `worktree.bgIsolation` took effect) and `--messaging-socket-path` (in the CLI's file with a help text, not in `--help`). `--allowedTools` is accepted and stored; `--max-budget-usd` was accepted and did not end a turn above the cap. `--permission-prompts` belongs to print mode | run A, run B, read |
| 5 | In a folder without workspace trust the command prints "Workspace not trusted. Run `claude` in <folder> once and accept the trust prompt, then retry.", exits 1 and starts nothing. A plain folder below a trusted one starts; a Git repository below a trusted folder needs an entry of its own. The documented manual entry (`projects["<path>"].hasTrustDialogAccepted` in the CLI's `.claude.json`) satisfies the check | run A, run B, docs |
| 6 | `--permission-mode bypassPermissions` without a recorded acceptance prints "--bg with bypassPermissions requires accepting the disclaimer first. Run `claude --dangerously-skip-permissions` once interactively." and exits 1 | run B, docs |
| 7 | Of the launching command's environment only `PATH` reaches the session; the `env` block of `--settings` does. The supervisor keeps the environment of the command that started it, and session processes inherit that | run A |
| 8 | A background session authenticates with the stored login of its configuration folder. An `ANTHROPIC_API_KEY` in the environment of the launching command and of the supervisor is not used (the turn ends "Not logged in"); an `apiKeyHelper` of the user settings is | run B |
| 9 | The supervisor is started by the first `claude --bg`, detached from the command that started it, one per configuration folder, and it exits a few seconds after its last session process is gone. `claude daemon --help` says: "Service install is disabled in this version". `claude daemon status` prints its process id and version and exits 1 when none runs | run A, run B, read |
| 10 | `claude agents --json --all` needs neither a terminal nor a running supervisor and lists every session of the operating-system user, a print-mode process as kind `interactive`. For a background session: `status` is `busy` during a turn and while a background command runs after a turn, `idle` otherwise, and `waiting` with `waitingFor` at a question or a permission prompt; `pid` is present while the process lives. `state` is written by a model request at each turn end and was wrong or stuck in several turns | run A, run B |
| 11 | The command line of a session process names neither the session nor its flags. The process leads a process group of its own; its parent is a host process of the supervisor | run A, run B |
| 12 | The transcript is the usual `<config dir>/projects/<folder>/<session id>.jsonl`, appended while the turn runs. A `system` record `turn_duration` follows every turn, also one that ended with an API error. The final text of a turn reached the file a few seconds after it was produced, always before `turn_duration`. The records `session-owner` and the attachments `session_context` and `credential_org` hold account identifiers. When a session moves into a Git worktree its transcript moves to another folder; with `worktree.bgIsolation` `"none"` it edits the folder itself and the file stays | run A, run B |
| 13 | An API error is an `assistant` record with `message.model` `<synthetic>`, `isApiErrorMessage` true and the member `error` (seen: `model_not_found`, `authentication_failed` without a login, `rate_limit` for a request refused with 429), then `turn_duration`; the process stays. The record is the object the stream frame of print mode is built from, so a used-up usage window also carries `apiErrorParams.rate_limit_info` | run A, run B; the last clause read |
| 14 | One JSON line `{"type":"user","uuid":"<uuid>","message":{"role":"user","content":"<text>"}}` written to the session's inbox socket is taken as a message: an idle session starts a turn with it (a `user` record with that `uuid`), a working session takes it in after the running tool call (`queue-operation` `enqueue`, then `remove` with reason `absorbed_mid_turn` and `commandUuid` equal to that `uuid`), and the turn's next request carries it. The `uuid` is kept because the socket was named with `--messaging-socket-path`; on the default socket the session makes its own. The session must run with `crossSessionInbound` `"accept"`; with `"hold"` the message is noted in the transcript and not delivered. `"priority":"now"` does not interrupt a running tool call | run A, run B |
| 15 | The model receives such a message as text of "another Claude session", "not typed by your user, but very likely working on their behalf", with the rule that it cannot approve anything. A message does not answer a question dialog or a permission prompt | run A, run B, docs |
| 16 | `claude --resume <id> --bg`: while a process of the session lives, also an idle one, the command starts a copy under a new id that has only the flags of that command; for a stopped session with an entry in the list, flags start a copy and no flags continue it in place with its saved options; for a session without an entry (after `claude rm`, or one print mode created) the command continues it in place under its own id with the flags of the command. The system prompt of the conversation's first request stays (`--system-prompt-snapshot`, on by default) | run A, run B, read |
| 17 | `claude -p --resume <id>` continues a session a background process created, once that process has stopped | run B |
| 18 | `claude stop <id>` exits 0 and the process is gone at once; the entry is `stopped`, the transcript ends with the open tool call and a `cost-state` record. `claude rm <id>` removes the entry and keeps the transcript. Both work without a supervisor | run A, run B |
| 19 | `cost-state` holds the session's totals per model with `costUSD`, cumulative across the processes of a session, equal to the sums of `message.usage` per message id, written when a process ends | run A, run B |
| 20 | A background command survives the end of its turn; when it ends the session runs a turn by itself for the notification (a `user` record with `origin.kind` `task-notification`). The command also survived two turns the API refused, and a message written afterwards started a turn whose request carried the refused inputs together with the new line | run A, run B |
| 21 | A question of `AskUserQuestion` and a permission prompt block the session (`status` `waiting`). `--disallowedTools AskUserQuestion` removes the tool from the requests | run A, run B |
| 22 | The CLI sends requests of its own for a background session: short ones at every turn end for the row summary and state of agent view, a check at the session's start, and one for a title when `--name` is missing. They use the small-model alias of the settings, and neither the transcript nor `cost-state` records them | run A, run B |
| 23 | The `api-request` records of a session list a beta whose name starts with `oauth-` exactly when its requests used the stored login: every record of a session on the login had it, no record of a session on a key | run A against run B |
| 24 | A background session loads the user's settings like a print turn without `--bare`: instructions file, hooks, plugins, default permission mode, fallback model | run A |
| 25 | Right after the launch command returned, the entry of the session names no `pid` yet. `claude stop` and `claude rm` at that moment both succeed and remove the entry, and no session process takes the prompt afterwards: no request reached the stand-in (found by the build) | run B |

### 2.3 Where the documentation and the runs disagree

The runs on the pinned CLI win.

- **`state`.** The documentation reads `done`, `blocked`, `working` as the session's state. In
  the runs the value came from a model request and did not follow the turn reliably (item 10).
  The design never reads `state`.
- **`--resume --bg`.** The documentation says the command "either continues that session under
  the same ID, or starts a copy" and gives no rule. The runs gave the rule of item 16; the
  sentence "the session keeps its flags" [docs, sessions] holds only while the session has an
  entry in the list.
- **The supervisor.** The documentation calls it a background service nobody has to manage. In
  2.1.325 it is a process that lives only while a session process lives (item 9).

Where the documentation is silent and a run decided:

- **An API key of the launching shell.** The documentation says the supervisor and its
  sessions use the stored credentials and keep the provider selection of the dispatching
  shell. It does not say what happens to an API key of that shell: it was not used (item 8).
- **Messages from a script.** The documentation leaves the line format open. The CLI's own
  debug log prints an example line, and item 14 is that form.
- **The prompt on stdin** (item 2) and **the session id** (item 3) are not documented.

### 2.4 The free check against the real CLI

The runs B need no login and no model call, which makes them the method of the real-CLI tests
(section 4.12): the CLI gets a `HOME` and a `CLAUDE_CONFIG_DIR` of its own, `ANTHROPIC_BASE_URL`
on loopback with the stand-in behind it, an `apiKeyHelper` in that folder's `settings.json`
that prints a dummy key (item 8: the environment key is not enough for a background session),
and a trust entry for the working folder in that folder's `.claude.json` (item 5). The
supervisor of such a folder is a separate instance with its own session list, so the check
never sees or touches sessions of the developer.

## 3. The process contract in both modes

| Topic | Print mode today (09 18) | Background mode (proposed) |
|---|---|---|
| Start | one `claude -p` process with stream-json on stdin and stdout, in a process group of the runner; the prompt is the first stdin line | one launch command `claude --bg <flags>` that returns at once; the prompt is its stdin; the session process belongs to the supervisor (4.3) |
| Session id | chosen by the runner (`--session-id`) | made by the CLI, read from the session list and reported as the thread's session (4.3) |
| Continuation | `--resume <id>` with the full flag list of the run | any process of the session is stopped and its entry removed first, then `--resume <id> --bg` with the full flag list (4.3) |
| State | the process and its stream | `pid`, `status` and `waitingFor` of `claude agents --json --all`, and the transcript; never `state` (4.5) |
| Live box source | stream messages (18.4) | transcript records (4.6); no task, workflow, progress or retry frames |
| Follow-up during a turn | one `user` line on stdin; absorbed after the running tool call; echo by `--replay-user-messages` | one line on the session's inbox socket; absorbed after the running tool call; echo by the transcript; the model is told the text comes from another session (4.4) |
| Stop | SIGINT to the group, then SIGTERM, then SIGKILL | `claude stop`, then the same escalation to the session process (4.7) |
| End of a turn and answer | `result` with the answer text | `system` `turn_duration`; the answer is the text of the turn's last assistant message (4.5) |
| End of the run | stdin closes and the process exits | the runner stops the session and removes its entry (4.7) |
| Usage and cost | `result.modelUsage`, cumulative per session; difference per run (18.7) | the `cost-state` record after the process ended, the same totals; the CLI's own requests are not in it (4.8) |
| Account errors and waiting | `error` of the API error message; a process with background work is kept, retry line on stdin (18.9) | the same member in the transcript record, the same wait, schedule and texts; the retry line goes to the inbox socket (4.9) |
| Background work after a turn | stdin stays open while the stream lists a task | the process stays while `status` is `busy`; every follow-up is a turn of its own (4.5) |
| Adoption after a runner restart | by the turn record and the process identity; without `runner.turn_handover` reading only | by the turn record and the session list; with every right, because no channel belongs to the runner process (4.11) |
| Hosted agents | from their definition (26 5) | stay in print mode (4.2) |
| Tool classes | flags of 18.2 and `--permission-prompts none` | the same class flags without `--permission-prompts`; `AskUserQuestion` disallowed; a session that waits for a person is stopped (4.5) |
| Voice | transcribed by the runner before the prompt or the follow-up line (18.14) | unchanged: the transcript text is part of the prompt or of the line |
| Attachments | local files named in the prompt or the line, `--add-dir` for the thread's folder; `ATTACH:` lines of the answer | unchanged; `TMPDIR` of the run reaches the session through the `env` block of `--settings` (4.3) |

## 4. Decisions

### 4.1 D-BG-1: one key, off by default, with an optional list of channels

`runner.launch_mode` is `"print"` (the default) or `"background"`. `runner.background_channels`
is a list of channel ids, empty by default; when it is not empty, only runs in these channels
use background mode and every other run uses print mode. Both keys are read at start.

- Alternatives: (a) no list, one switch for all forums: an operator could not try the mode on
  one forum of a running installation first. (b) a member `launch_mode` in the binding's
  `runner_config`: it is a closed schema the server checks (09 18.1), so it needs a change of
  the server, the API and the apps for a property of the runner host; left for later. (c) a
  setting per hosted agent: hosted agents stay in print mode (D-BG-2).
- Reason: nothing changes until an operator opts in, the opt-in can be one forum, and the way
  back is one key.

### 4.2 D-BG-2: what runs in background mode

Only the turns of the runner's own agent in billing mode `personal`. In print mode stay:

- **`organization` turns.** A background session takes no API key from the environment (2.2
  item 8), and a pay-per-token key has no subscription window to save.
- **Hosted agents.** Below `host` their turns run `--restricted`, for which the CLI binds no
  inbox socket [read], so no follow-up or retry line could reach them; `--max-budget-usd` did
  not act in a background session (item 4), so the cost cap of 26 5.8 would rest on the
  runner alone; and a hosted turn is never adopted (26 3.2) while a background session
  outlives the runner, with its per-turn MCP file of resolved secrets.
- **The compaction attempt of 18.9.** `/compact` as an input line is a print-mode path, and a
  slash command in an inbox message does not run [docs]. Both modes share the session store
  (items 16 and 17), so the compaction runs in print mode and the retry in background mode.
- **The one-shot calls** (search assistant, builder of the interview): they keep no session.

- Alternative: every turn. Refused for the reasons above; each is a ticket of its own (7.5).

### 4.3 D-BG-3: launch, session id and continuation

1. **Before a launch** the runner reads the session list once. It continues only when no
   process of the session lives. An entry of the session that carries the runner's name prefix
   is stopped (`claude stop`) when it has a process and then removed (`claude rm`). Removing
   the entry is what makes the next step continue in place with this run's flags (item 16)
   and keeps the host user's agent view free of finished runner sessions. An entry under
   another name belongs to another program (a session a person bound to the thread and also
   runs themselves) and is never stopped or removed: with a live process the run ends `failed`
   with `adapter_error` and the cause `session_busy`, without one the run uses print mode
   (4.13), because flags would start a copy (item 16).
2. **The command** is `<runner.claude_bin> [--resume <id>] --bg --name bothy-<run id> --model
   <model> <flags of the tool class> [--add-dir <folder>] --append-system-prompt-file <file>
   --messaging-socket-path <socket> --settings <file>`, in the project folder, with the
   allow-list environment of 18.2 plus `DISABLE_AUTOUPDATER`, the prompt on stdin, and stdout
   and stderr in files of the run. The files of a launch (the settings file, the system prompt
   file, the launch command's stdout and stderr) and the inbox socket lie in
   `<runner.state_dir>/bg/` and are named after the run, never after the session: the CLI makes
   the id of a new session only after the launch (step 5), the session's own folder moves to
   that id, and a live session process keeps the paths it was started with. The socket's name is
   the first 16 hexadecimal digits of the SHA-256 of the run id, which keeps the path below the
   system's bound. They are removed when the run left the runner. The flags of the class are
   those of 18.2 with
   `AskUserQuestion` added to `--disallowedTools` under `scoped` and `host` (`chat` has no
   tool). `--permission-prompts none` is left out (print mode only). A test compares the whole
   list per class. With `runner.launch_mode` `"background"` the start is refused, naming
   `runner.state_dir`, when the socket path is longer than a socket address of the system
   allows.
3. **The settings file** of the run (mode `0600`, in that folder) holds exactly:
   `crossSessionInbound` `"accept"`, `worktree.bgIsolation` `"none"`, and `env` with
   `CLAUDE_PRODUCT_RUN`, `DISABLE_AUTOUPDATER`, the run's `TMPDIR` and, only with
   `runner.forward_ssh_agent`, `SSH_AUTH_SOCK`. Reasons: the launching command's environment
   does not reach the session (item 7); without `bgIsolation` `"none"` the session would move
   into a worktree of its own, against 18.1, and its transcript would move with it (item 12).
4. **The prompt** is the two parts of the first stdin line of 18.2 (the context with trust
   marks, then the trigger part) joined by a blank line, as one text. It never starts with `/`
   or `!`, because the context part comes first. Alternative: the positional argument. Refused:
   it is bounded by the system's argument limit and is readable in the process list.
5. **A new session.** `run.start` carries the runner's own UUID as today (the order of 4.9 in
   09 stays). After the launch the runner reads the short id from stdout and the full id from
   the list, moves the session lock and the session's state folder to that id and reports it
   with `agent_session.report`, as the "session missing" path of 18.9 reports a new reference,
   without the box note. Alternative: let the runner choose the id. Not possible (item 3).
6. **A resumed session.** The runner first checks that the transcript exists; a missing one
   takes the "session missing" path of 18.9 with a new session. After the launch the short id
   must be the first eight characters of the session id and stderr must carry no `note:` line;
   otherwise the CLI did not continue the session in place (another program started the
   session in between): the runner stops what the launch started, removes a copy, and ends the
   run `failed` with `runtime_error` and the cause `background_session_copied`.
7. **Started** means: the entry of the session has a `pid`, and the transcript shows the
   prompt as a `user` record after the launch offset, both within `runner.cli_probe_timeout`
   (4.13 says what happens otherwise). The runner then records the process
   (its id and start time), reports `started = true` as after `system/init`, and runs the
   counter-check of 18.3 on the first `api-request` record (item 23): a `personal` turn whose
   request did not use the stored login is stopped and ends `billing_mode_mismatch`. The
   pre-check `claude auth status` stays binding.
8. **Options per run.** Because of step 1 every flag applies per run as in print mode. The
   system prompt is the exception in both modes: the CLI records it at the conversation's
   first request (item 16).

### 4.4 D-BG-4: follow-ups and every other input go to the inbox socket

`capabilities.followups` stays `steer`. A follow-up, a retry line of 18.9 and any other input
of a running run is one line of the form of item 14 on the socket the runner named at the
launch, with the `uuid` and the text the line has on stdin in print mode (lead-in, blank line,
content, attachments and transcripts), so the record of an input is the same in both modes. The
runner patches `delivered` when the line is written and `absorbed` when the transcript shows a
`queue-operation` `remove` with that `commandUuid`, or a `user` record with `origin.kind` `peer`
and that `uuid`. The checks of 18.5 before the write are unchanged.

- Alternatives: (a) `claude --resume <id> --bg "<text>"`: starts a copy while the session lives
  (item 16). (b) typing into `claude attach` through a pseudo-terminal: it is the only channel
  that yields a real user message and it worked in two of two attempts [run A], but it reads a
  terminal screen of a preview feature. (c) the default socket, its path taken from a
  `SessionStart` hook that prints the documented variable `CLAUDE_CODE_MESSAGING_SOCKET`: it
  needs a hook program in every session and loses the `uuid`, so `absorbed` would be matched by
  text. Kept as the replacement if the flag of item 4 goes away. (d) no steering: below.
- Reason: it is the channel the runs proved, with the timing of today's stdin line (after the
  running tool call) and an exact echo.
- What differs from print mode: the model is told that the text comes from another session
  that very likely works for the user (item 15), not that the user typed it; such a message
  cannot approve anything; a slash command in it does not run; it is plain text. A turn of the
  runner has nothing to approve, and a follow-up line never starts with its text anyway. The
  runner prompt gains one paragraph for sessions started in background mode: a message that
  arrives this way and starts with the runner's lead-in, or says that it is an automatic retry
  by the runner, is the person's own message or the runner's own line, relayed by the runner,
  and is followed like one. The prompt of a print turn does not carry the paragraph and keeps
  its bytes.
- When the socket cannot be used (the CLI binds none, or a managed setting holds or refuses
  inbound messages), the follow-up is not written, or is written and never taken in. Either
  way the server defers it into a new run after this one (section 7 of 09), as it does today
  for a follow-up without an echo. What the person loses then, exactly: the
  running turn finishes everything it is doing without the correction, tool calls included;
  the message waits for the end of the run, and a run that holds background work does not end
  before that work does, so a status question gets no answer meanwhile; and only a stop
  interrupts.

### 4.5 D-BG-5: state, end of a turn, answer

- **Sources.** The transcript, read every `runner.poll_interval` from a stored offset like the
  stream file today, and the session list, read by one reader for all runs of the runner
  process every `runner.background_status_interval` (default `2s`) while a background turn is
  active, and immediately when a turn end is read. The runner reads `id`, `sessionId`, `pid`,
  `status`, `waitingFor`, `kind` and `name` of entries of its own sessions and nothing of any
  other entry. Whether the process lives is checked by its recorded id and start time.
- **Not read:** `state` (2.3), the files under `jobs/` (documented as unstable; one of them
  holds a summary of the answer written by a model, not the answer), `claude logs` (terminal
  bytes).
- **A turn ends** at `system` `turn_duration`. Its answer is the text of the last assistant
  message of the turn; a turn whose last assistant record is an API error has none. Each
  answered turn posts one answer, as each `result` does today.
- **After a turn end** the runner waits for a reading of the list taken after it. `status`
  `busy` means the process still works without a turn (background work), unless the transcript
  already shows the start of the next turn (a queued line or a notification makes the session
  busy again at once): the run stays `running` and the rule of 18.5 "Background work" applies
  with `status` in the place of the task list. Between two turns a reading that says `busy`
  and is newer than the last read of the transcript is decided with the next read, for the
  same reason. `status` `idle`, no written input without echo and, for a process that had
  background work, the quiet window `runner.followup_grace` end the run (4.7).
- **A session that waits for a person** (`waitingFor` present) is stopped, and the run ends
  `failed` with `runtime_error`, a box note that names the kind and the cause
  `background_session_waiting`. The class flags prevent
  it: `dontAsk` refuses what would prompt, `bypassPermissions` does not prompt, and
  `AskUserQuestion` is disallowed (item 21).
- Alternative for the turn end and the answer: a `Stop` hook of the runner in `--settings`,
  whose input carries `last_assistant_message`; the documentation prefers that field to the
  transcript ("the transcript file isn't guaranteed to include the final message at Stop time
  on all versions") [docs, hooks]. Refused for the first build: it adds a program the CLI
  runs in every session, does not fire for a turn that ends with an API error, and gives no
  tool steps, so the transcript is read anyway. The risk is in section 5 item 4.

### 4.6 D-BG-6: the live box from transcript records

The table replaces the stream table of 18.4 for a background turn. Only records of the main
conversation are read (`isSidechain` false). Every other record type is skipped and counted
per type in a debug line; no transcript line is copied anywhere (2.2 item 12).

| Record | Fields | Live box and run |
|---|---|---|
| `user` with a text content and `origin.kind` `human`, the first after the launch offset | `sessionId` | the run's prompt, the echo of the input the session was launched with; `agent_session.report` with `started = true` once (in the place of `system/init`); every record before it is skipped, because a resume first writes what closes the interrupted turn of the process before |
| `api-request`, the first of the process | `params.betas` | counter-check of 18.3 (4.3 step 7) |
| `assistant` | `message.id`, `message.model`, `message.content[]` (`text`; `tool_use` with `id`, `name`, `input`), `message.usage` | `/model` from the first one that is not synthetic; `/last_text_excerpt`; one `tool_call` step per `tool_use`; context percent while the window of the model is known; `/cost` as in 18.7, once per `message.id` with its newest usage |
| `assistant` with `isApiErrorMessage` | `error`, `apiErrorParams` | no work; the class of an account error and its reset time (4.9); the recognised error texts of 18.9 |
| `user` with `tool_result` items | `tool_use_id`, `is_error` | the step ends `completed` or `failed` |
| `user` with `origin.kind` `peer` | `uuid` | the echo of the written line with that `uuid`: `absorbed` of that follow-up, its turn starts |
| `user` with `origin.kind` `task-notification` | none | a turn the session starts by itself |
| `queue-operation` | `operation`, `reason`, `commandUuid` | `enqueue` and `dequeue` stand in for `command_lifecycle` `queued` and `started` (18.5), matched in their order because they carry no id, and `dequeue` starts a turn; `remove` with `absorbed_mid_turn` sets `absorbed` |
| `system` `turn_duration` | none | end of the turn, answer, account wait (4.5, 4.9) |
| `cost-state`, the last after the process ended | `modelUsage` | costs of the run (4.8) |

Not available in background mode:

| Print mode | In background mode |
|---|---|
| `tool_progress` | none; while `status` is `busy` during a turn each reading of the session list counts as one progress event (the live box patches `/metrics/progress_events` at most once per `runner.progress_patch_interval`), so a long tool call is not reported stalled |
| `system/task_*` and `workflow_progress`: steps of kind `subagent`, `workflow`, `background_job`, their usage, the agents of a workflow, `/metrics/subagents` | none; a subagent or a workflow shows as the one `tool_call` step of its tool, and steps inside it are not shown |
| `system/background_tasks_changed`: `/metrics/background_jobs` | 1 while `status` is `busy` between turns, else 0; no task list, so a summary names lost work without a count per task |
| `system/status` `compacting`, `system/api_retry` | none: no `compacting` status line, no retry metrics |
| `rate_limit_event` | none; the usage mark of 18.19.10 cannot be set |
| `result`: `contextWindow`, cost per turn, `num_turns` | the window of a model is known only from an earlier print turn of the session's store; `/cost` is settled once, after the process ended |

How the build reads the table: each record is mapped to the stream message of 18.4 that means
the same (`bgtranscript.py`), so the live box, the turn record, `ProcessState` and the rules of
18.5 and 18.9 run one code path in both modes; a class test holds that every recorded
transcript gives the decisions of the print stream of the same scenario. `apiErrorParams` is
read in the CLI's file and is in no recording, because the stand-in sends no usage window. The
record `session-owner` is written only on a login; the recorder and the runner drop it by type
either way.

### 4.7 D-BG-7: stop and the end of a run

- **Stop** (`run.cancel_requested`, a run the server ended, the stop after an error of 18.5):
  `claude stop <short id>`; when the process still lives after `runner.stop_grace`, SIGTERM to
  its process group, after `runner.kill_grace` SIGKILL, each logged `stop_escalated`. The run
  finishes `cancelled` only when the process is gone; then the entry is removed. A stop ends
  the session's background work, as today.
- **A cancellation while the launch is under way** (a stop of the run or of the runner between
  the launch command and "started") stops and removes the session of that launch and finishes
  the run `cancelled` with `adapter_stopped`. The run is not left for a start from the inbox:
  the session may have taken the prompt, and a second start would run it again. The stop works
  also while the entry names no process yet (item 25).
- **End of a run** ("release", in the place of closing stdin): the same `claude stop`, then
  the costs, then `claude rm`, then `run.finish`. The summary `run_ended_idle` is unchanged.
  Alternative: keep the process between runs and send the next run's prompt to the inbox
  socket. Refused: the prompt of a run would then be a relayed message and not a user message
  (item 15), totals are written only when a process ends (item 19), an idle process is
  stopped by the supervisor anyway and comes back without the named socket, and the launch
  after a stop was cheap: its first request read the prompt cache of the stopped process
  [run A].
- **An orderly stop of the runner** stops its background turns as 18.19.2 says, also with
  `runner.turn_handover` on: the keeper and the named pipe are print-mode parts. Leaving
  background turns for the next runner at an orderly stop is left for later (7.5).
- A process of the session other than the one the runner launched (the supervisor restarted
  it) is not driven: it has no named socket. The runner stops it and ends the run
  `runtime_error` with the cause `background_process_replaced`.

### 4.8 D-BG-8: costs

The run books the difference between the session's totals after its release and the stored
values, by the rules of 18.7 unchanged; the totals come from the last `cost-state` record,
which the runner already reads before a resume. While the run works, `/cost` is priced from
`message.usage` as today. Without any totals of the run's processes after the release (a
process killed hard writes no `cost-state` record) the priced sum of the run's `assistant`
messages is booked and logged as `costs_priced_without_totals`; the stored session values stay
as they are, because the CLI restores the totals from the last record it finds.

- Not booked: the CLI's own requests of item 22. They are short, they draw the same
  subscription, and an operator whose settings point the small-model alias at a large model
  pays that model at every turn end. Every launch passes `--name`, which saves the title
  request.

### 4.9 D-BG-9: account errors

The classes, the rule "a process that holds or held background work is kept", the status line,
the schedule, the give-up and the summaries of FIX-RUNNER-LIMIT-KEEPS-RUN stay as they are:
`ProcessState` decides from the same events. Only two things change. The event source is the
API error record and the `turn_duration` after it (item 13). The retry line is written to the
inbox socket; the turn it starts carries the refused input, a person's message or a task's
notification, in the same request, and a background command lives through refused turns (item
20, the three properties `tests/real_cli/test_refused_request.py` holds for print mode).
Without a usable socket the retry line cannot be written; it is tried again after each pause
of the schedule, and the wait ends with the give-up of `runner.account_retry_max_wait`, as for
a line that cannot be written in print mode.

### 4.10 D-BG-10: the supervisor

- **Start.** The runner installs and starts no service. The launch command starts the
  supervisor when none runs, with the launch environment, so sessions inherit the allow list
  of 18.2 and nothing else.
- **Check before every launch:** `claude daemon status`. No supervisor: launch, and record
  the supervisor that launch started (its process id and start time, in
  `<runner.state_dir>/bg/supervisor.json`) when the launch printed that it started the
  service. A launch that printed no such line although the check found no supervisor ran
  under a supervisor another program started in between: its session is stopped and removed
  and the run goes on by the rule of 4.13 with `background_supervisor_foreign`. A recorded
  supervisor with the pinned version: launch. Any other supervisor, of another
  version or started by another program of the same user: no launch, the run uses print mode
  with the cause
  `background_supervisor_foreign` or `background_supervisor_version` (4.13). The check, the
  launch and the record are one step that the runs of a runner process take one after the
  other (a lock of the process, held until the launch command ended and the record is
  written): of two runs that start together while no supervisor runs, the second reads the
  supervisor only after the first one's launch started it and the runner recorded it. Without
  that order the second run reads "no supervisor", its launch prints no line about the
  service, and the runner takes its own supervisor for another program's. Reason: sessions
  inherit the supervisor's environment (item 7), and a supervisor a person started from their
  shell would hand that shell's variables to a `scoped` turn, which can print them with a
  read-only command. Alternative: accept a shared supervisor. Refused: it weakens the allow
  list of 18.2 without a sign.
- **The runner never stops a supervisor.** `claude daemon stop --any` would end sessions of
  the host user.
- **Heartbeat.** No new member and no new capability: both lists are closed (09 4.8), and a
  server would refuse them. `run_slots` counts a background turn until its release.
  `degraded_reasons` gains the code `background_unavailable` while the last launch attempt
  fell back for a cause of the host (4.13), cleared by the next background launch that
  starts. It blocks no run.
- **After a machine restart** no supervisor and no session runs, and nothing starts them
  ("Shutting down or restarting your machine stops running background sessions" [docs]). The
  runner's start finds the turn records: a session without a process is a gone turn (4.11).
  The next message in the thread launches the session again, which starts a supervisor.
  Background work of the old process is lost, as in print mode.
- **Updates.** `DISABLE_AUTOUPDATER` is in the launch environment and in the session's `env`;
  the operator rule "keep automatic updates of the CLI off" of the README holds for this mode
  too, because the documentation says a supervisor restarts itself onto a new version.

### 4.11 D-BG-11: adoption after a runner restart

The turn record gains `launch_mode`, the session's name, the short id once known, the socket
path, the `uuid` of the input the session was launched with, the process id with its start
time, the launch offset and the stored offset of the transcript, and the run's model, tools
and billing mode. It is written before the launch command runs, so a next runner finds a
session by its name. A session process cannot inherit the runner's lock descriptors, so the
locks of a background turn live with the runner process; the adopting runner takes them again
before it reads on (one attempt; a busy lock leaves the record for the next start). At start,
before the first inbox event:

- the session's entry has the recorded process and the run runs: adopt. The runner maps the
  transcript from the launch offset to the stored offset again, replays `ProcessState` as
  18.19.7 describes, and from then on drives the turn like its own, with follow-ups, retry
  lines and stop, because none of these channels belonged to the old runner process. The end
  of an adopted turn is classified without a recovery: no process is ever started for an
  adopted run (R-AG-17).
- the process lives and the run has ended: stop and remove it.
- a process of the session that is not the recorded one (the supervisor started the session
  again): it is stopped first and counts as gone.
- no process: finish from the transcript when the turn's end is in it or the record holds a
  decided end, by the ledger's rules (an answer is posted once), else leave the run to the
  server (`interrupted`).
- a record of a background turn is settled the same way by a runner whose
  `runner.launch_mode` is `print` again, so switching the key off needs no step.
- a record that still carries the runner's own UUID of 4.3 step 5 (the runner died between
  the launch and the report of the CLI's id): the session is found by its name
  `bothy-<run id>` and handled by the same three cases.

### 4.12 D-BG-12: the version pin and the tests that hold the interface

Background mode runs only on the pinned version; on any other version a run uses print mode
with the cause `background_version_untested` (print mode itself only warns then). The pin
holds five surfaces: the launch output and the refusal texts of items 1, 5 and 6 with the
`note:` lines of item 16; the fields of the session list; the transcript records of 4.6; the
line of the inbox socket; the outputs of `stop`, `rm` and `daemon status`.

- **Recorded fixtures** per pinned version, written by `tests/real_cli/record.py` with the
  method of 2.4 into `tests/fixtures/recording/background_<version>/`: what the launch command
  prints per case, the session list in each state, the outputs of `stop`, `rm` and
  `daemon status`, the transcripts of five scenarios (a new post, a resumed session, a
  follow-up during a tool call, a background command with its notification, a stopped turn),
  the print streams of the first four as their twins, and a table of the members the CLI
  writes per record type (the counterpart of `stream_fields_<version>.json`). The recorder
  keeps only the record types the runner reads, which drops those of item 12. The scenarios
  that need a refused request, a copy or a waiting session are run live by level one and
  played by Fake-Claude, not recorded.
- **Fake-Claude** gains a background side: `--bg` prints the launch line and starts a detached
  player under a fake supervisor process; the player writes the stream fixture of the scenario
  as transcript records (a test holds every member it writes against the recorded table),
  listens on the named socket and waits on the gates of a test; `agents --json --all`, `stop`,
  `rm` and `daemon status` answer from a state file in the fake configuration folder. Every
  unit and scenario test uses it.
- **Real CLI, level one** (free, behind `CLAUDE_CODE_RUNNER_REAL_CLI=1`):
  `tests/real_cli/test_background.py` runs the mechanics of items 1 to 6, 13, 14, 16 to 18, 20
  and 25 against the stand-in and compares the field table with a fresh run; `test_pinned_cli.py`
  holds the names the mode relies on in the CLI's file.
- **Real CLI, level two** (a real turn, the owner's decision as today): one short background
  turn on the login that checks the `entrypoint` of item 12 and the beta of item 23
  (`test_real_background_turn_runs_on_the_stored_login`). It refuses to start while a
  supervisor runs and stops and removes only its own session. The function that reads both
  facts from a transcript is held by level one; the test itself was not run when the mode was
  built.
- Changing the pin records the fixtures again and reads section 2.2 against the new version;
  the README section "Changing the pinned version" gains these steps.

### 4.13 D-BG-13: when the mode is unavailable, the run uses print mode and says why

A run that should use background mode and cannot uses print mode. The live box shows once per
run, as a status line, that background sessions are not available, with the code; the log has
`background_fallback` with the code; and a cause of the host sets `background_unavailable` in
the heartbeat (4.10).

| Code | When | Host cause |
|---|---|---|
| `background_version_untested` | the installed CLI is not the pinned version | yes |
| `background_supervisor_foreign`, `background_supervisor_version` | 4.10 | yes |
| `background_bypass_not_accepted` | the refusal of item 6 (class `host`) | yes |
| `background_workspace_untrusted` | the refusal of item 5 for this folder | no |
| `background_session_foreign` | the session has an entry in the list that the runner did not create (4.3 step 1) | no |
| `background_launch_failed` | any other launch that exits with an error, `claude daemon status` or the session list that cannot be read, or no process within `runner.cli_probe_timeout` (agent view switched off, for example) | yes |

- The rule that keeps a prompt from running twice: the fallback happens only while it is
  certain that no session took the prompt (the launch exited with an error, or, watched for
  `runner.cli_probe_timeout`, the session list shows no session of the run or its entry got no
  process and its transcript no record of the prompt; a session whose process starts late
  would take the prompt after the fallback). In every other doubt the runner stops and removes
  the session and ends the run `runtime_error` with the code as its cause.
- The status line of the live box is "Background sessions are not available (`<code>`); this
  run uses print mode.". The fallback holds for the rest of the run: a later process of the
  same run (a recovery) uses print mode too.
- The runner never writes the CLI's trust entry or its acceptance of bypass mode. Both are
  one-time steps of the operator in the host user's own CLI configuration; the README names
  them.
- Alternative: end such a run `failed`. Refused: the mode is an optimisation, and both modes
  continue the same session (items 16 and 17).

## 5. Risks and what the design does not cover

1. **The usage rate is not proven.** The design proves the path (the `entrypoint` of a
   turn), not what the path costs. An operator measures both modes on their own account before
   relying on it.
2. **Research preview.** Background sessions and agent view may change in any release. The
   answer is the pin (4.12), the fallback (4.13) and the default `print`.
3. **Undocumented surfaces.** The transcript format, the socket line, the flag
   `--messaging-socket-path`, the `betas` member and the output texts are internal. Only the
   session list is called supported. A server-side switch of Claude Code can turn the inbox
   socket off for a session [read]; follow-ups then wait for the end of the run (4.4).
4. **The transcript at the end of a turn.** The documentation does not promise that the final
   text is in the file when the turn ends. In every run it stood before `turn_duration`, which
   is the only point the runner reads it at; a version that breaks this order fails the
   level-one test.
5. **The idle stop.** The supervisor stops a session process that has been idle "for about an
   hour" and restarts it at the next message [docs]. The runner ends a run long before, and a
   process that works is not idle; a restarted process is handled by 4.7. The restart itself
   was not run.
6. **Background work other than a shell command.** That `status` stays `busy` for a
   background subagent, a workflow or a monitor is documented, not run. A scheduled wake-up of
   the session with `status` `idle` in between ends the run and is lost, as a timer of a
   print-mode process is lost when that process ends.
7. **A changed login.** The supervisor reads the stored login when it starts and looks again
   while it has none [run B]. What a running supervisor and its sessions do when the stored
   login is replaced or renewed under them is not documented and not run; the account wait of
   4.9 covers a login that comes back, and a supervisor ends with its last session.
8. **Service managers.** The supervisor leaves the process group of the command that starts
   it, so it survives the runner. A service manager that ends every process of the service's
   control group still ends it with the runner; then 4.11 finds no process.
9. **Shared session list.** Every runner session is visible to the host user in agent view
   while its run lasts, and a person can attach to it or stop it. The runner treats a session
   that vanished or was replaced as an error of the run.
10. **Control calls.** Before the first request of a run the runner makes a handful of short
    CLI calls (supervisor check, session list, launch, and stop and remove for a resumed
    session), and the session list is read at the status interval while a background turn is
    active. Each call starts the CLI binary, so a run starts later than in print mode. Runs
    that start together make the calls up to the launch one after the other (4.10), so each
    of them also waits for the launches before it.
11. **Not covered:** hosted agents, `organization` turns, a member of the binding for the
    mode, the handover of background turns at an orderly stop, steps inside subagents and
    workflows, the usage window of 18.19.10, Windows (the socket is a named pipe there, not
    run), and a helper that records workspace trust.
12. **Messages of other sessions of the host user.** `crossSessionInbound` `"accept"` in the
    settings file of a run is what makes the session take the runner's lines (item 14), and it
    holds for every message on the inbox socket, not only the runner's. Without a value a
    session that bypasses permission prompts (tool class `host`) holds a message of a session
    that does not bypass them for an approval, and a value of `--settings` goes before the one
    in the user settings [docs, cross-session-messaging]; with `"accept"` in the run's file
    neither protects a runner session. The socket is restricted to the operating-system user,
    such a message cannot approve anything and a command in it does not run [docs]. But the
    paragraph of 4.4 tells the model that a message with the runner's lead-in is the person's
    own, so another Claude Code session of the host user that writes the lead-in can steer a
    runner session with the rights of its tool class [unproven]. The README tells an operator
    to switch the mode on only where the host user runs no other sessions, or only ones
    trusted like the runner. A marker per run in the paragraph and in every line, which
    another session cannot know, would close this; it is not built.

## 6. Specification changes of the build

09 18.1 (the session id of a background turn), 18.2 (the second argument list and the
settings file), 18.3 (the counter-check), 18.4 (the table of 4.6), 18.5 (inputs on the inbox
socket), 18.6 (stop and release), 18.7 (totals from `cost-state`), 18.9 (the causes of 4.13,
adoption) and 21 (`runner.launch_mode`, `runner.background_channels`,
`runner.background_status_interval`); the runner README, `config.example.toml` and the
generated configuration reference; the rulings in `docs/decisions.d/FEAT-RUNNER-BG-SESSIONS.md`
and a changelog fragment of the runner.

## 7. Build brief: FEAT-RUNNER-BG-SESSIONS

### 7.1 Scope

An opt-in background mode for the turns of the runner's own agent in billing mode `personal`,
as decided in section 4, in `plugins/claude-code-runner` and the chapters of section 6. Depends
on FIX-RUNNER-CLI-PIN-325. No change of the server, the SDK, the API or the apps. Print mode
keeps every line of its behaviour.

### 7.2 Core goals

1. Without the key nothing changes (1.1 item 1).
2. A `personal` run of the own agent in a listed channel runs as a background session from
   launch to release and books its costs (1.1 item 2, D-BG-3, D-BG-7, D-BG-8).
3. Follow-ups steer a running turn through the inbox socket with `delivered` and `absorbed`
   (1.1 item 3, D-BG-4).
4. Stop, background work and the account wait behave as in print mode (1.1 items 4 to 6,
   D-BG-5, D-BG-9).
5. A killed runner's background turn is adopted with every right (1.1 item 7, D-BG-11).
6. Every unavailable case falls back to print mode with a visible cause and one prompt (1.1
   item 8, D-BG-10, D-BG-13).
7. The interface is held by recorded fixtures and a free real-CLI test, and no account
   identifier leaves a transcript (1.1 item 9, D-BG-12).

### 7.3 Test plan

- **Unit** (Fake-Claude, no CLI): the argument list per tool class and the settings file
  against fixed lists; the parser of the launch output and of each refusal text; the reader of
  the session list (own entries only, `state` never read); the mapping of 4.6 record by
  record, with the class test of `test_processstate.py` extended so that every recorded
  background transcript gives the same `ProcessState` decisions as its print-mode stream; the
  cost difference from `cost-state` including a missing record; the configuration keys; the
  supervisor check; the fallback decision for every row of 4.13, including "never after the
  prompt was taken".
- **Recorded fixtures:** the scenarios below recorded from 2.1.325 by the method of 2.4, the
  field table, and the test that the recorder and the runner write none of the record types of
  item 12.
- **Scenarios** (Fake-Claude background side and the SDK FakeServer), each also in print mode
  where it exists today: a new post and a second message; a follow-up during a tool call; a
  follow-up with an attachment and one with a voice message; an answer attachment; a stop
  during a tool call and during the account wait; a background command with a follow-up and
  the notification's answer; the account error with retry and with give-up; a session that
  waits for a person; a copy at resume; a killed runner and the adoption, with a second kill
  of the adopter; each fallback cause; a channel outside `runner.background_channels`; two
  runs that start together while no supervisor runs.
- **Real CLI:** level one as in D-BG-12, required for the ticket. Level two only on the
  owner's word.
- **End to end:** `tests/realserver` gains one forum in background mode with Fake-Claude: a
  post, a follow-up, a stop. Built on a server of its own (`tests/realserver/test_background.py`):
  the world of the shared server has Lotse on `organization`, and I1 allows `personal` only
  while the credential holder alone can trigger, so the file revokes the grants of persons on
  the space and switches Lotse to `personal` before its flows.

### 7.4 Order of work

1. The background side of Fake-Claude and the recorder, with the first recorded fixtures; the
   level-one test of the mechanics. Everything after it is built against these.
2. The session module: launch, session list, stop, remove, supervisor check, socket write.
3. The transcript mapping and its tests against the fixtures.
4. The run: launch to release in `runturn.py` beside the print path, costs, turn record.
5. Follow-ups and the account wait on the socket.
6. Adoption.
7. Fallback, heartbeat reason, configuration keys.
8. Specification, README, configuration reference, rulings, changelog fragment.

### 7.5 Deliberately left for later

- Background mode for hosted agents and a setting per agent.
- A member of the binding for the mode per forum (server, API, apps).
- Leaving background turns for the next runner at an orderly stop with
  `runner.turn_handover`.
- Steps inside subagents and workflows from their own transcripts, and the usage window from a
  status line command.
- Following a process the supervisor restarted, and taking over a copy as the thread's
  session.
- A `Stop` hook as a second source of the answer, and the hook route to the default socket.
- A command of the runner that lists the bound folders without workspace trust.
- A key that accepts a supervisor the host user started (owner decision 6).

## 8. Owner decisions

1. **Measure before relying on it.** Whether background mode draws a subscription like the
   interactive CLI is the reason for the ticket and is not proven here. Recommendation: one
   measurement of the three paths (interactive, print, background) on the operator's own
   account before the build starts, because a result "background draws like print" ends the
   ticket.
2. **Steering as a relayed message.** In background mode a follow-up reaches the model marked
   as coming from another session (4.4). Recommendation: accept it, with the paragraph in the
   runner prompt; the alternative is `followups = queue` for this mode.
3. **Hosted agents stay in print mode** in the first build. Recommendation: yes, and decide
   after the first weeks of use whether the follow-up ticket is worth it.
4. **The CLI's own requests** at every turn end are not booked and follow the host's
   small-model alias (4.8). Recommendation: leave the alias to the operator and say so in the
   README.
5. **Level two of the real-CLI test** costs usage, as today. Recommendation: one run before
   the first use on a real forum.
6. **A supervisor the host user started** makes the runner fall back to print mode while it
   lives (4.10), which can be often on a host where the person uses agent view themselves.
   Recommendation: keep the refusal in the first build, because the cause is visible and the
   alternative hands the person's shell environment to runner turns; add a key that accepts a
   shared supervisor only when the fallback proves to be a burden.

## 9. What the build changed

The build kept every decision of section 4 in substance. Where it had to be more exact than
the draft, or found that the draft was wrong, the decision above was changed in the same
change; this list names each such place.

| Decision | Draft | Built | Reason |
|---|---|---|---|
| D-BG-3 step 2 and 3 | the launch files are "files of the run", the socket lies "in a folder of the state directory" | everything lies in `<runner.state_dir>/bg/`, named after the run; the socket's name is a digest of the run id | the session's folder is renamed when the CLI's id is known, and a live process keeps its paths; a socket path is bounded |
| D-BG-3 step 7 | no bound for "started" | within `runner.cli_probe_timeout` | a launch that never takes the prompt must end |
| D-BG-4, the `uuid` of a line | a fresh `uuid` per line | the `uuid` of the runner's input line | one record of an input in both modes; adoption maps the echo by it |
| D-BG-4, the prompt paragraph | the prompt paragraph names the lead-in | it also names the runner's retry line | the retry after an account error arrives the same way |
| D-BG-5, after a turn end | `busy` after a turn end is background work | unless the transcript already shows the next turn | a queued line or a notification makes the session busy again at once |
| D-BG-5, a waiting session | no cause for a waiting session | cause `background_session_waiting` | the summary in the thread names why the run failed |
| D-BG-6, the first prompt record | the first prompt record is ignored | it is the echo of the launch input | the turn record counts the input as taken |
| D-BG-6, progress | progress by the runner's own clock per `runner.progress_patch_interval` | one progress event per reading of the session list, throttled by the live box | the reading is the evidence that the session works |
| D-BG-7 | no rule for a cancellation during the launch | the session is stopped and removed, the run ends `cancelled` | the prompt must not run twice |
| D-BG-9 | a retry that cannot be written counts as an error after `runner.stop_grace` | it is tried again after each pause until the give-up | the same path as print mode |
| D-BG-10, the record of the supervisor | the supervisor is recorded after a launch without one | only when the launch printed that it started the service; else the launch is aborted | another program can start a supervisor between the check and the launch |
| D-BG-10, runs that start together | every run checks, launches and records by itself | the runs of a runner process do the three one after the other | the second of two runs that start together took the supervisor the first one's launch started for another program's and ended `runtime_error` |
| D-BG-11 | adoption reads from the stored offset | it maps the records from the launch offset again first; a foreign process of the session is stopped; the end has no recovery; one lock attempt | the mapper holds state of earlier records; R-AG-17 |
| D-BG-12 | the transcripts of every scenario of 7.3 are recorded | five scenarios and four print twins; the others are run by level one and played by Fake-Claude | a refusal, a copy and a waiting session are single commands, not conversations |
| D-BG-13 | "no process" is judged at once | the entry is watched for `runner.cli_probe_timeout` first | a process that starts late would take the prompt after the fallback |
