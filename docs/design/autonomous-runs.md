# Autonomous runs: wake-ups, restart handover and no silent time caps

Status: design of 2026-11-06, revised six times the same day after red team reviews: their
blockers and majors are resolved in the text, the keeper of D-AR-3 is proven by a probe under a
scratch launchd job (section 2.4), the replay of a turn's state at a handover by a probe against
the runner modules (section 2.5), the open owner decisions are recorded as rulings (section 11),
and the minors are in section 12. Of it, only D-AR-1 is built (AR-02, with the run reason
`adapter_stopped`, [AR-02 rulings](../decisions.d/AR-02.md)). This
note records the goal, the evidence and the decisions with their reasons. Ticket AR-01 wrote the
normative text into the chapters it names (09 5.8 and 18.19, 07 2.14, 03 and the others of section
7), [ADR 0077](../adr/0077-autonomous-runs-wake-ups-and-turn-handover.md), the rulings
[decisions.d/AR-01.md](../decisions.d/AR-01.md) and the briefs AR-02 to AR-10, and recorded the CLI
facts marked unverified below in 09 18.19.12. Where this note and a chapter disagree, the chapter
wins.

Why a design note and not an ADR: the decisions span the runner, the server (data model, actions,
API, a periodic job), the clients and the operation of an installation, and some of them are
owner rulings on cost. The repository keeps such cross-cutting drafts in `docs/design/` (for
example [builtin-agents.md](builtin-agents.md)); the architecture part becomes one ADR in AR-01,
and the rulings go to `docs/decisions.d/` with the tickets that build them.

Sources used below. Runner and SDK code at `fd01c2b16` (`plugins/claude-code-runner/src` and
`sdk/python/src`). Spec [09-agents.md](../spec/09-agents.md). The strings of the pinned CLI.
Scratch probes: a real runner process with a fake CLI (section 2.2), the keeper under a scratch
launchd job of its own (section 2.4) and the runner modules (section 2.5). Where the note speaks
of a runner run as a launchd job, it assumes the form stated in section 2.4, not any particular
installation. Facts that are not proven carry "unverified".

## 1. Goal

Owner request (translated and shortened): every session, chat or development, must
be able to keep working on its own as long as it has work, without stopping after some time, so
that a steering session can in principle work on indefinitely when enough tasks exist.

Three failures, and what this design can honestly promise for each:

- **(a)** The agent ends a turn promising to continue after an outside event (for example "CI is
  green") and nothing wakes it. This cannot be made impossible: an agent can always decide not to
  ask for a wake-up. The design makes it never silent: every run end carries a machine-readable
  state of the agent and a flag whether a wake-up is pending, and a run that ends `waiting` with
  nothing pending gets one nudge and then a visible note (D-AR-8).
- **(b)** A runner restart kills the session's background waiters. With `runner.turn_handover` on
  this becomes impossible for restarts and runner crashes (D-AR-2, D-AR-3, D-AR-14). It stays
  possible for a host reboot or a killed turn group; then a recovery run or a note follows
  (D-AR-7).
- **(c)** A time limit silently ends a session that still has work. Every limit of section 3 is
  removed, made irrelevant, or ends work visibly. The subscription usage window, which a session
  that runs on long enough reaches by construction, no longer kills background work; the run ends
  `usage_limited` and the thread wakes when the window resets (D-AR-12).

Constraints: every change is additive and off until switched on, like D-A1-21 to D-A1-23 of
[builtin-agents.md](builtin-agents.md); an installation that sets none of the new keys behaves as
today, proven by today's runner fixtures staying byte-equal. Cost safety stays: autonomous work
is limited by who may start it, by a visible cap and by an idle cap that cannot be switched off.

### 1.1 Done means

1. An agent in a runner turn can request a wake-up ("after 20 minutes", "at 17:55", "when the
   checks of pull request 123 finish", "when this command succeeds"). The wake-up survives the end
   of the process, a runner restart and a runner crash, and later starts a run in the same thread
   that resumes the same Claude session with `--resume` and tells the agent why it woke, or is
   delivered as a wake input into the thread's active run (D-AR-13).
2. A runner restart (SIGTERM from `launchctl kickstart -k`, followed by launchd's kill of the job's
   process group, or a runner crash) hands every live turn over to the next runner with stdin
   intact: the keeper and the CLI survive, background tasks keep running, the next runner knows
   them and every other per-turn state (D-AR-2), follow-ups and wake inputs reach the turn without
   closing its stdin while a task runs, the server never ends the run `interrupted`. Runs between processes continue in the next runner (D-AR-14):
   preparation and lock wait, which lie before `run.start`, through the inbox restart of 09 4.9,
   and transcription, launch and compaction retry from the turn record, a
   runner crash during a launch leaves one recorded CLI or none (D-AR-3 launch handshake), and an
   adopted process that ends gets the same recovery attempts as an own one (adoption rule of
   D-AR-14).
3. When work is lost anyway (host reboot, a killed process group, an error end with running
   tasks, a turn stopped by adoption after `run.orphan_timeout`), the thread gets a recovery run
   that tells the agent which tasks were lost, within the autonomy limits of item 5, or a visible
   note.
4. No silent end: every finished run carries the agent's end state (`done`, `waiting` or
   `blocked`) and whether a wake-up is pending; a run that ends `waiting` with nothing pending and
   no running task gets one nudge, then a visible note in the thread.
5. Every time limit of section 3 is removed, made irrelevant by this design, or set by a named
   configuration key with a visible notice when it ends work. Autonomous work is capped by a count
   of autonomous inputs per thread and person and by an idle count; both are visible when reached.
6. With none of the new keys set, the runner's command line, environment, heartbeat and stream
   handling stay byte-equal to today's fixtures, and the server behaves as today.

## 2. Evidence: the three failure classes

### 2.1 (a) Nothing wakes a session after its run

- A run ends when its process exits. The runner keeps stdin open only while the CLI reports
  background tasks (`background.py` `holds()`, lines 104 to 122) and for `runner.followup_grace`
  after the last stream line of a process that had background work; a process that never had
  background work gets stdin closed at its `result` (`followups.py` `stdin_close_due`, 09 18.5).
- After `run_finished`, a new run starts only from a new human message or a deferred follow-up
  (09 section 7). Messages with `origin = 'agent'` never trigger (09 3.2), and the server has no
  job, webhook or timer that starts a run (`runs.trigger_kind` is one of `message`, `command`,
  `interaction`, `migrations/00051_runs_runs.sql` line 70).
- The CLI's own timers do not help. Session crons and `ScheduleWakeup` are available to runner
  turns (the `system/init` frame of a runner stream lists them), but the cron fire handler returns
  at once when input is closed, and crons die with the process (strings of the pinned CLI).
- Failure class, read from the code and the strings: an agent that calls `CronCreate` for a
  one-shot hours ahead and tells the person that the timer is set ends its turn; with no
  background task running, stdin closes at the `result`, the run finishes, and the timer dies
  with the process without firing.
- The runner prompt (`prompts/runner_prompt.md`, section "Background work") tells the agent to end
  every turn within about a minute and to rely on background notifications, but nothing tells it
  that a promise without a running task ends its work.

### 2.2 (b) A runner restart kills live turns and their background work

- Root cause, reproduced: on SIGTERM the runner cancels every run task (`__main__.py` 74 to 76,
  SDK `adapter.py` 656 to 660 and 718 to 725). The cancellation arrives inside `_run_process`,
  whose `finally` (`runturn.py` 679 to 681) has already set `self.process = None`; the
  `except CancelledError` branch (`runturn.py` 449 to 455) then skips `terminate`, and the outer
  `finally` (456 to 459) deletes the turn record and the run files anyway. The CLI runs in its own
  session (`turn.py` line 140, `start_new_session=True`), survives, and sees end of input when the
  old runner's pipe closes.
- Scratch probe at the code of this note (a real runner process with a live turn, then SIGTERM):
  `runner_rc 0`, `turn_alive_after_sigterm True`, `record_exists False`, `run_state running`,
  `stop_escalated_in_log False`. No test covers this path; `tests/scenarios/test_crash.py` covers
  SIGKILL and the stop of an already adopted turn only.
- This contradicts 09 18.2, row "Detachment": "a restart never silently kills a turn".
- Failure class: a restart while turns run kills their background work. Each orphaned CLI sees
  end of input, and after the CLI's background ceiling (`CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS`,
  section 3) it terminates every background task it still runs, agents, workflows and shells
  alike. The runner log has no `turn_adopted` and no `stop_escalated` for such a turn. The CLI
  lives until the ceiling only because it runs in its own session; a process left in the launchd
  job's process group is killed with the job (section 2.4).
- Side effect, read from the code: an orphan keeps the thread and session locks (the turn
  inherits the `flock` descriptors), so the next run in its thread waits `thread_busy` until the
  orphan dies at the ceiling, and past `runner.session_lock_wait` it is refused.
- Even the crash path (SIGKILL, record left) loses the work: an adopted turn gets no stdin and no
  follow-ups (D-M2-35-3), and its pipe ended with the old runner, so the CLI's background ceiling
  applies.
- The server then ends such runs `failed/interrupted` after `agent.presence_timeout` (09 5.5).

### 2.3 (c) Time limits that can end work

Section 3 lists every limit found, with its decision. The ones that can end work: the CLI's
background ceiling after stdin closes, the CLI's short sweep of background shells after stdin
closes when no agent, workflow or armed Monitor runs (a built-in delay, read from the strings),
the drop of session crons while input is closed, the runner's kill of a process that outlives an
error `result` by `runner.stop_grace`, and the single compaction per run (`recovery.py` 188 to
191).

One limit is not a timer but reaches every long session: the subscription usage windows. Runner
streams carry `rate_limit_event` frames with the status of each window. When overage is not
available (`overageStatus: rejected`), no request can fall back to overage once a window is used
up. A frame has this shape (synthetic values, field names as in a runner stream):

```json
{"type": "rate_limit_event", "rate_limit_info": {"status": "allowed", "resetsAt": 1700000000,
 "rateLimitType": "five_hour", "overageStatus": "rejected", "unifiedWindows": {"five_hour":
 {"utilization": 0.5, "resetsAt": 1700000000}, "seven_day": {"utilization": 0.5,
 "resetsAt": 1700500000}}}}
```

The runner handles none of this (no rate or usage limit text in `failures.py`, `causes.py` or
`recovery.py`). Read from the code, a rejected request would end so: the error `result` closes
stdin at once (18.5), after `runner.stop_grace` the runner stops the process with its background
tasks, the run fails `runtime_error`, and nothing wakes the thread after `resetsAt`. (Since
2026-11-13 a process with background work is kept at such an error and the turn is tried again
after short pauses, 09 18.9 "Account errors", `docs/decisions.d/FIX-RUNNER-LIMIT-KEEPS-RUN.md`;
the wake-up at the reset stays the design below.)

The schema of the frame in the pinned CLI strings names the values: `status` and `overageStatus`
are each one of `allowed`, `allowed_warning`, `rejected`; `rateLimitType` is one of `five_hour`,
`seven_day`, `seven_day_opus`, `seven_day_sonnet`, `seven_day_overage_included`, `overage`; the
frame is "emitted when rate limit info changes". Unverified: the text of the error `result` of a
rejected request, and whether the `rejected` frame always comes before that `result`. A later frame with `status` `allowed*` ends
such an outage early (D-AR-12); it does not restart a run that already ended.

### 2.4 The process group of the launchd job

- Assumed form of the job: a runner run as a launchd job in a user's `gui` domain whose program
  is `uv run ... claude-code-runner run` (so `uv` is the job's main process), with an
  `ExitTimeOut`, `KeepAlive` `{SuccessfulExit: false}` and no `AbandonProcessGroup`; the service
  guide of AR-09 describes this form. `launchd.plist(5)`, AbandonProcessGroup: "When
  a job dies, launchd kills any remaining processes with the same process group ID as the job".
  Any helper the runner starts without its own session is signalled with a restart.
- POSIX `setpgid` refuses to move a process into a group of another session. A probe
  (`Popen(..., process_group=<pid of a start_new_session child>)`) failed with
  `PermissionError [Errno 1]`. A helper can therefore not join the turn's group from outside.
- `ExitTimeOut` in `launchd.plist(5)` states SIGTERM first, then SIGKILL after the timeout.

Probe of the keeper of D-AR-3 under a scratch launchd job (own label, own temporary plist of the
assumed form above: `uv run` as main process, `KeepAlive {SuccessfulExit: false}`, no
`AbandonProcessGroup`, a short `ExitTimeOut`). A probe runner started a keeper with
`start_new_session=True`; the keeper opened a named pipe for reading without blocking, then for
writing, forked a fake CLI with the read end as stdin, and the fake CLI started one background
`sleep`. The runner also started a plain child as a control. Results:

| Step | Result |
|---|---|
| Start | keeper PID = its process group = its session; fake CLI and its background task in the keeper's group; runner, `uv` and control in the job's group |
| `launchctl kickstart -k`, runner exits 0 at SIGTERM (the handover) | `uv` forwarded SIGTERM to the runner; the control child got SIGTERM and died; keeper, fake CLI and background task lived on and never saw end of input; launchd restarted the job although it exited 0; the new runner opened the pipe with `O_WRONLY` and `O_NONBLOCK` and its line reached the fake CLI |
| `kickstart -k` while the runner ignores SIGTERM | after `ExitTimeOut` launchd sent SIGKILL to the main process (`last terminating signal = Killed: 9`), that is `uv` only; keeper, fake CLI and background task lived on |
| leftovers of the job's group | a control child that traps SIGTERM got exactly one SIGTERM and lived on; the runner child that ignored SIGTERM also outlived the killed `uv` (section 12) |
| `launchctl bootout` | the runner exited, launchd did not restart it, keeper and fake CLI lived on (D-AR-2) |
| SIGUSR1 to the keeper with no runner left | the keeper closed its write end, the fake CLI saw end of input and exited 3, the keeper wrote wait status 768 (exit code 3) to the exit file by rename and exited |

So a process in its own session survives every restart path of the job, and a helper in the job's
group gets at least SIGTERM. Which of launchd and `uv` sent that SIGTERM to the job's group was
not separated; it does not matter for the design, which keeps nothing of a turn in that group.
The probe files are scratch files outside the repository; AR-09 turns the probe into a test.

### 2.5 What an adopting runner does not know today

- An adoption reads the stream from the record's `offset` (`runturn.py` 1587 to 1592), the last
  checkpoint, and starts with a fresh `BackgroundWork`. The turn record (`_persist`, `runturn.py`
  1484 to 1521) holds the ledger, follow-ups, recovery flags and steps, but not the running
  background tasks, `seen` or `turn_open`. `system/background_tasks_changed` comes only when the
  set changes, so a task started before the checkpoint is never reported to the adopter again.
- The stdin rule (`runturn.py` 819 to 840) closes stdin when `holds()` is false and
  `stdin_close_due` is true. An adopter with full rights (D-AR-2) would therefore close stdin at
  the first `result` after the handover (a follow-up's or a wake input's) and the CLI would kill
  the task: failure (b) again. The same holds for every other state that lines before the
  checkpoint carry: the task list with descriptions (D-AR-7), the nudge mark (D-AR-8) and the
  usage-limit mark (D-AR-12).
- The stream reader skips every message type outside `KNOWN_TYPES` (`streamreader.py` 28 to 31
  and 139 to 140); `rate_limit_event` is not among them, so today no runner reads the frame at all.
- The stdin and stop rules also read state that `_read` keeps as a local variable or as an
  attribute reset per process, which no record holds. Inventory of every value the stdin rule
  (`holds()`, `runner.followup_grace`, `command_lifecycle`, pending inputs, `stdin_close_due`)
  and the stop rule (`_failed_at`, `runner.stop_grace`) read, at the code of this note:

| Value | Read by | Written at | Survives a handover today |
|---|---|---|---|
| `last_result_at` | `stdin_close_due` (`followups.py` 290 to 305, `None` means never), held log (`runturn.py` 826) | local of `_read`, `None` at each call (`runturn.py` 738); `result` without a queued input sets it (777 to 780); `command_lifecycle` `started` clears it (773 to 775) | no: starts empty in the adopter |
| `_failed_at` | stop after an error (`runturn.py` 808 to 817), `_lost_work` (691 to 697) | first error `result` of an own turn (788 to 793); reset per process (684 to 689) | no: reset, and the adopted read skips it (`own` gate, 788) |
| `_lost_tasks` | `_lost_work`, log `turn_stopped_after_error` (815 to 816) | with `_failed_at` (793), again at the stop (815) | no |
| `BackgroundWork.running`, `seen`, `turn_open`, `last_line_at`, `queued` | `holds()` (`background.py` 104 to 122), `input_waiting` (`runturn.py` 777, 830) | `apply` per line (`runturn.py` 754, `background.py` 68 to 102); new per process (685) | no: fresh object (first fix round: replayed) |
| `_held_logged` | the one log `stdin_held_for_background` (`runturn.py` 826 to 828) | reset per process (686) | no; logging only |
| `followups.pending()` | `stdin_close_due` (`runturn.py` 835, `followups.py` 220 to 222) | write and echo of follow-ups | yes: record `followups` (`runturn.py` 1562) |
| `phase` | both rules (`runturn.py` 819, 836) | write, close, stop (665, 844, 850) | set to `closed` on adoption (1558); D-AR-2 sets `open` for a turn with a named pipe |
| `stop_reason`, `server_ended`, `stopping` | stop of a stopped or ended run (`runturn.py` 801 to 807) | request, server answers, stop task | `stop_reason` from the record (1563 to 1564); `server_ended` from the server's next answer; `stopping` is a task of the runner itself |

  The values derived from stream lines (the first five rows) are lost on a handover; the others are
  in the record, come from the server or belong to the runner. Of the lost ones, `last_result_at`
  is today re-derived only by accident: `TurnLedger.checkpoint` never moves the stored offset past
  the start of the last `result` (`ledger.py` 307 to 323), so the adopter reads that line again.
  The red team's case of a `result` before the stored offset therefore does not occur with the
  current ledger (probe case 2 shows what would happen), but the stdin rule would then depend on a
  rule the ledger has for posting answers, and `_failed_at` is lost for real when an error `result`
  is followed by a notification turn's `result` before the checkpoint (probe case 3).

Probe against the runner modules for this state (`BackgroundWork`, `stdin_close_due`,
`TurnLedger` with its real checkpoint, `StreamReader`; a scratch script outside the repository,
adapted from the red team's probe). A prototype
`ProcessState` of D-AR-2 holds `BackgroundWork`, `last_result_at`, the failure mark and the lost
task count, and its `apply` has the branches of `_read`; the rule mirrors `_idle` (`runturn.py`
803 to 840) and reads only that object. The old runner reads the lines at 100 s plus their index,
the adopter replays from `process_start` to `offset` and reads on, both at the adoption time
200 s; "no replay" is today's adopted read with the stdin rule switched on; "2nd adopt" is a third
runner at 300 s after the first adopter wrote the record again through the ledger. Defaults
`runner.followup_grace` 30 s and `runner.stop_grace` 10 s. Cases: (1) background work ended, its
notification turn answered, restart inside the quiet window, no further input; (2) the same with
the offset past the last `result`, the red team's assumption; (3) an error `result` while a task
runs, then the task's notification turn, restart before the stop; (4) a task still runs. Output:

```text
[quiet window] process_start 79 ledger offset 356 used offset 356 file size 397
  old runner: [(110.0, 'hold'), (140.0, 'CLOSE')]
  no replay : [(200.0, 'CLOSE'), (229.0, 'CLOSE'), (230.0, 'CLOSE'), (86400.0, 'CLOSE')]
  replayed  : replay 5 lines, read 1; [(200.0, 'hold'), (229.0, 'hold'), (230.0, 'CLOSE'), (86400.0, 'CLOSE')] (last_result_at 200.0, failed_at None, lost 0)
  2nd adopt : process_start kept True [(329.0, 'hold'), (330.0, 'CLOSE')]
[quiet window, offset past the result] process_start 79 ledger offset 356 used offset 397 file size 397
  old runner: [(110.0, 'hold'), (140.0, 'CLOSE')]
  no replay : [(200.0, 'keep-open'), (229.0, 'keep-open'), (230.0, 'keep-open'), (86400.0, 'keep-open')]
  replayed  : replay 6 lines, read 0; [(200.0, 'hold'), (229.0, 'hold'), (230.0, 'CLOSE'), (86400.0, 'CLOSE')] (last_result_at 200.0, failed_at None, lost 0)
  2nd adopt : process_start kept True [(329.0, 'hold'), (330.0, 'CLOSE')]
[error result before the checkpoint] process_start 79 ledger offset 318 used offset 318 file size 359
  old runner: [(103.0, 'hold'), (112.0, 'STOP')]
  no replay : [(205.0, 'CLOSE'), (210.0, 'CLOSE'), (86400.0, 'CLOSE')]
  replayed  : replay 4 lines, read 1; [(205.0, 'hold'), (210.0, 'STOP'), (86400.0, 'STOP')] (last_result_at 200.0, failed_at 200.0, lost 1)
  2nd adopt : process_start kept True [(305.0, 'hold'), (310.0, 'STOP')]
[task running] process_start 79 ledger offset 206 used offset 206 file size 247
  old runner: [(110.0, 'hold'), (3600.0, 'hold')]
  no replay : [(230.0, 'CLOSE'), (3600.0, 'CLOSE')]
  replayed  : replay 2 lines, read 1; [(230.0, 'hold'), (3600.0, 'hold')] (last_result_at 200.0, failed_at None, lost 0)
  2nd adopt : process_start kept True [(330.0, 'hold'), (3600.0, 'hold')]
```

So with the replay into one object, stdin closes `runner.followup_grace` after the adoption when
nothing more comes (cases 1 and 2, whether the last `result` lies before or after the offset), a
replayed error `result` stops the group `runner.stop_grace` after the adoption with the lost task
counted (case 3), a running task keeps stdin open (case 4), and a second handover gives the same
decisions because `process_start` is part of the ledger's record. Without the replay the adopter
closes too early (cases 1, 3, 4), never closes (case 2) or never stops after the error (case 3).

Probe against the runner modules (`BackgroundWork`, `TurnLedger`, `StreamReader`,
`stdin_close_due`, run with `uv run` in `plugins/claude-code-runner`; a scratch script outside the
repository, adapted from the red team's probe). A stream file holds an
earlier process of the run (task `old01`), then the adopted process: a background task with a
description, a `rejected` usage frame, the echo of a nudge input, a `result`, checkpointed by the
real ledger; after the handover a follow-up turn runs while the task still runs. Output:

```text
record: process_start 204 offset 577 file size 618
today:  running [] holds False stdin_close_due True
replayed lines 3 (none posted), then normal read from offset; rate_limit_event known: False
replay: running ['bgjob01'] seen True holds True usage mark None task descriptions {'bgjob01': 'Wait for CI'} nudge mark True
--- with rate_limit_event among the known types (AR-06):
record: process_start 204 offset 577 file size 618
today:  running [] holds False stdin_close_due True
replayed lines 4 (none posted), then normal read from offset; rate_limit_event known: True
replay: running ['bgjob01'] seen True holds True usage mark rejected task descriptions {'bgjob01': 'Wait for CI'} nudge mark True
```

So the replay of D-AR-2 restores the running task, `seen`, the task descriptions and the nudge
mark, ignores the earlier process's task, and keeps stdin open where today's adoption closes it;
the usage mark needs `rate_limit_event` among the reader's known types, which AR-06 adds.

## 3. Time limits: inventory and decision

| Limit | Value | What it ends | Decision |
|---|---|---|---|
| CLI background ceiling after stdin EOF (`CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS`) | built-in default of the pinned CLI, idle time after EOF; overridden by `runner.cli_bg_wait_ceiling` | all background tasks, agents and workflows included | made irrelevant: stdin is closed only when no task runs (today's rule) and no longer by a restart (D-AR-3); set explicitly through `runner.cli_bg_wait_ceiling` (D-AR-10) |
| CLI shell sweep after EOF | built-in delay of the pinned CLI, a few seconds | background shells | same as above; whether a shell can be held to the background ceiling instead is unverified (section 12) |
| CLI session crons and `ScheduleWakeup` | dropped while input is closed; die with the process; one-shots missed on resume are skipped; recurring ones expire after a built-in period of the CLI | the agent's own timers | replaced by runner wake-ups; these tools are disallowed in turns when wake-ups are on (D-AR-6; that the flag works under `bypassPermissions` is unverified, section 12) |
| CLI Monitor timeout | built-in default and maximum of the pinned CLI (inferred from the strings) | one Monitor inside a process | kept; it notifies the agent, which decides; waits longer than minutes use a wake-up (prompt, D-AR-8) |
| CLI Stop hook block cap | built-in cap of the pinned CLI | a hook loop | not relevant to the runner, which configures no Stop hook; whether hooks of the host user's own CLI configuration reach runner turns is unverified and recorded by AR-01 |
| CLI Bash, max turns, API timeouts | defaults unverified | a foreground call or a turn | the runner sets none of them; foreground calls stay short by the prompt; AR-01 records the defaults from the pinned CLI |
| Subscription usage windows (`five_hour`, `seven_day` and the per-model weekly windows; overage where the account allows it) | set by the account; `resetsAt` in each `rate_limit_event` | every request of every session of the account | a `rejected` frame without overage marks the run usage-limited; running background tasks are not stopped; a wake-up at `resetsAt` with reason `usage_limit` continues the thread, as a wake input while the process is held, else as a wake run; one wake-up per outage, a pending one cancelled when the mark clears; the run end is `usage_limited` (D-AR-12) |
| `runner.followup_grace` | 30 s | closes stdin after the quiet window when no task runs | kept: it ends a process with no work; the end-state check (D-AR-8) runs before the close |
| `runner.stop_grace` after an error `result` | 10 s (plus `runner.kill_grace` 5 s) | a process that outlives an error, with its background tasks | kept (it fixed a held process that refused every follow-up), except after a usage-window end (D-AR-12); lost tasks now lead to a recovery run (D-AR-7) |
| Compaction and session replacement | once per run | the run, `runtime_error` | kept per run; the run end leads to a recovery run, which has its own budget (D-AR-7); an adopted turn gets the same recovery attempts as an own one, within the same once-per-run limits proven by the record, and a pending retry survives a restart (adoption rule of D-AR-14) |
| `runner.session_lock_wait` | 10 min | a run refused `session_busy` | made irrelevant for orphans by D-AR-2 and D-AR-3; a wake-up never starts a second run in a thread with an active run (D-AR-13) |
| `runner.max_parallel_runs` slot wait | no limit, bounded by `run.queued_max_age` | none | kept |
| launchd `ExitTimeOut` | set in the job's definition by the operator | the old runner on restart (SIGKILL to `uv`, the main process) | kept: the handover exits at once (D-AR-2), and the keeper is outside the job's group (D-AR-3, probe of section 2.4) |
| `agent.presence_timeout` | 90 s | a run missing from `active_runs`: `interrupted` | kept: the handed-over turn is in the next runner's first heartbeat (adoption runs before it); the old runner sends no heartbeat after the handover began (D-AR-2) |
| `run.orphan_timeout` | 15 min without heartbeat | `stale` | kept: a runner down for `run.orphan_timeout` with live turns is an outage; adoption stops such a turn as today (`adoption.py` line 143, 09 18.9), and a recovery run follows (D-AR-7) |
| `run.queued_max_age` | 48 h | a queued run, `agent_unavailable` | kept; a wake run waits for an offline runner like any run |
| `agent.invocation_token_ttl` | 60 s | a follow-up or wake input token | kept; tokens are renewed as today |
| `run.stall_after` | 10 min | nothing, a notice | kept |
| New: `run.wake_max_pending` | 5 per thread | a further wake-up of the thread is refused | new cap, visible: the refusal names it (D-AR-9) |
| New: `runner.wake_check_failures` | set in AR-05 | a `pr_checks` or `command` wake-up whose check keeps failing ends `failed` | new cap, visible: note and notification (section 8, test 8) |
| New: `run.wake_max_wait` | 7 days | a pending wake-up | new cap, visible: the expiry posts a note in the thread (D-AR-5) |
| New: `run.usage_wake_max` | 20 per run | a further usage-limit wake-up of the run is refused | new cap, visible: the refusal posts a note, a reply after the reset continues (D-AR-12) |
| New: `run.wake_min_delay` | 60 s | nothing; delays the earliest fire of every wake-up | new floor against tight loops (D-AR-9) |
| New: `run.autonomous_chain_max` | 25 | the next autonomous input of a thread and person | new cap, visible: a note in the thread, a reply by that person continues (D-AR-9) |
| New: `run.autonomous_idle_max` | 3 | the next autonomous input after idle ones | new cap that cannot be switched off, visible (D-AR-9) |

The runner itself has no limit on run duration, turns or budget (`commandline.py` 216 to 235), and
the server has none for a running run (`RenewVerdict` applies only to `queued` and
`waiting_input`). This stays so.

## 4. Decisions: restart

**D-AR-1 Fix the orderly stop first, as its own small change.** The cancel path of `RunTurn.run`
must never lose the process handle: the handle the `except CancelledError` branch uses is kept
until the turn is known to be gone, and the turn record and run files are removed only after the
process exited. Until D-AR-3 is built, an orderly stop terminates live turns as 09 18.6 already
says (SIGINT, SIGTERM, SIGKILL to the group), logs `stop_escalated`, and finishes their runs
`cancelled` with a reason the thread shows. Reason: the bug is reproduced, contradicts 09 18.2 and
leaves orphans that hold locks; the fix is small and does not wait for the rest. Rejected: leaving
the current behaviour until the handover exists (it silently orphans turns at every update).

**D-AR-2 A restart hands turns over instead of ending them.** On SIGTERM the runner stops taking
new runs, sends no further heartbeat, leaves every live turn running, keeps its record, writes
`turn_handed_over` per turn and exits with code 75 (`EX_TEMPFAIL`) when at least one turn was
handed over, else 0 as today. The non-zero code makes `KeepAlive {SuccessfulExit: false}` start
the next runner after every SIGTERM that does not unload the job (a `kill` of the job, not only
`kickstart -k`, which restarts even after exit 0, section 2.4). The next runner adopts every
recorded live turn (09 18.9 crash path) with full rights: stream reading, live box, follow-ups,
wake inputs, the stdin rule of 18.5, and at the end of the process the classification and the
recovery of `_recover` like an own turn's (the adoption rule of D-AR-14), on the replayed
per-turn state described below. Only `claude-code-runner stop --turns` (a new explicit command) or an
owner stop ends turns on purpose. A `launchctl bootout` or an uninstall unloads the job and leaves
handed-over turns without a runner (probe of section 2.4); the runner service guide that AR-09
writes into 16 (the repository has none yet) therefore runs `claude-code-runner stop --turns`
before `bootout` or an uninstall, and if that was skipped, the next install
adopts the turns from their records, or, after `run.orphan_timeout`, stops them and starts a
recovery run (D-AR-7). Reason: an update is routine and must cost nothing; adoption already exists and covers
the crash case. Rejected: "refuse to restart while turns run" as the lasting rule (a held turn
may run for days, so such a gate blocks every update for that long and operators switch it off,
and a gate that people turn off is no protection; ruling 4 keeps "restart only while no turn
runs" only for the restarts that install the handover), and "kill and resume" (loses background work every time).

*Per-turn state survives by replay.* The adopter must know everything the old runner knew about
the process, not only what lies after the last checkpoint (section 2.5). The turn record
therefore stores `process_start`, the stream offset where the turn's current process began (the
`stream_start` the runner already passes to the ledger and the reader when it starts the
process, kept unchanged by later checkpoints). It is a field of the ledger's part of the record
(`TurnLedger.to_record` and `from_record`), not one of the keys `_persist` copies for an adopted
turn (`runturn.py` 1518 to 1520), so every adopter writes it again and a second or third handover
replays from the same start. The adopter first replays the lines from `process_start` up to the
record's `offset` through the state parts only, then reads on from `offset` as today.

All state the stdin and stop rules read and that stream lines produce lives in one object per
process, `ProcessState` (inventory in section 2.5): it owns `BackgroundWork` and takes over
`last_result_at` (today a local of `_read`), the failure mark `_failed_at` with the lost task
count, and the marks below. Its `apply(message, now)` is the only writer, and the same call serves
the live read and the replay; `_idle` and the stop rule read only this object, the record's
follow-ups, the run's stop state and the configuration. A rule that later reads a new value of a
stream line gets it in `ProcessState` and is fed by the replay by construction. The object holds:

- `BackgroundWork` (running tasks, `seen`, `turn_open`, queued inputs), so the stdin rule of
  18.5 holds stdin open while a task runs;
- `last_result_at` with today's branches: a `result` without a queued input sets it, a
  `command_lifecycle` `started` clears it; a replayed `result` sets it to the adoption time, so
  with nothing pending stdin closes `runner.followup_grace` after the adoption, never before;
- the failure mark: an error `result` read while the usage mark of D-AR-12 is not set sets it
  once with the count of running tasks, live or replayed; a replayed one sets it to the adoption
  time, so the stop after an error applies `runner.stop_grace` after the adoption (the old runner
  already closed stdin at that line; the adopter closes it again, which is idempotent); an error
  `result` read while the usage mark is set never sets the failure mark but only the usage error
  mark below, so clearing the usage mark can never start a stop of running tasks; with the handover the adopter's read applies this rule and the stdin rule like an own turn's
  (the `own` gate of `runturn.py` 788 and 795 becomes "the turn has stdin");
- the task list with each task's id and description from `system/background_tasks_changed`, for
  the recovery run of D-AR-7;
- the usage-limit mark of D-AR-12 with its `resetsAt` and `rateLimitType`, the outage number
  (one more at each `rejected` frame that sets the mark while it is clear), and whether the
  outage's wake-up was already requested (the request is idempotent per run, reason and outage
  number, D-AR-12); the outage number counts per run, not per process: the turn record keeps the
  run's highest outage number, and the `ProcessState` of a later attempt starts from it, so a
  compaction or retry attempt never reuses a number of an earlier attempt;
- the usage error mark of D-AR-12: whether the last error `result` came while the usage mark was
  set; it only feeds the classification `usage_limited` and never leads to a stop;
- the nudge mark of D-AR-8: the nudge is an input line like any other, so its `uuid` is in the
  record's input list before it is written (ledger rule 3), flagged as the nudge; its echo in the
  replayed lines restores "nudged in this waiting phase";
- the idle flag and the autonomous-input count of the current input (D-AR-9).

The replay posts nothing, patches nothing, plans no answer and moves no offset; the ledger keeps
its own rules for the lines from `offset` on (answers, inputs, end evidence). Replayed lines get
the adoption time as their read time, so the quiet window of `holds()`, the window of
`stdin_close_due` and the stop grace after an error all start again at the adoption (stdin is held
longer, never shorter, and a failed turn is stopped at most `runner.stop_grace` later than by the
old runner). A record without `process_start` (written by a runner without
`runner.turn_handover`) or without a named pipe is adopted as today, without stdin. Every new
per-turn state a later change adds goes into `ProcessState` and is derived in the same replay, so
it is correct after a handover by construction; AR-03 adds a test that fails when `_read` or
`_idle` keep a stream-derived value outside the object (the replayed object of a recorded stream
must equal the old runner's, field by field, apart from the read times). Proof: the probes of
section 2.5. Rejected: persisting
each state explicitly in the record (every new state needs its own field and write order, and a
forgotten field is this failure again; the stream already holds all of it and is the evidence);
replaying the whole stream file (it also holds earlier processes of the run, whose tasks ended
with them, task `old01` in the probe). The replay reads the process's lines once; for a process
held for days that is a linear read of its stream at start, no network call.

**D-AR-3 Stdin of a turn survives its runner: the keeper is the turn's session leader.** With
`runner.turn_handover` the runner no longer starts the CLI itself. It creates a named pipe per
launch, `<launch>.stdin` (mode `0600`; `<launch>` is defined with the launch handshake below), and
starts a small keeper program
(`python -m claude_code_runner.keeper`, part of the runner package, so nothing new to install)
with `start_new_session=True`. The keeper therefore leads its own session and process group,
outside the launchd job's group, and:

1. opens the named pipe for reading without blocking, then for writing (it holds this write end
   for the turn's whole life), then switches the read end to blocking;
2. writes its own identity file and waits for the go of the launch handshake below; only after
   the go it forks and execs the CLI with the read end as stdin, the stream and stderr files as stdout and
   stderr, and the lock descriptors passed on as today; the CLI stays in the keeper's group, so
   one group holds both, and the keeper closes its own copies of the lock descriptors and of the
   pipe's read end after the fork (it keeps only its write end); after releasing the CLI it
   writes the attempt's initial inputs (launch handshake below) into its own write end, then
   writes `<launch>.fed` atomically by rename, and for a compaction attempt then closes its write
   end (the `/compact` line is that attempt's only input). It writes these inputs with the write
   end in non-blocking mode, in one loop that also watches the CLI's exit (`waitpid` with
   `WNOHANG` on a `SIGCHLD` wake-up), so a CLI that ends or stops reading in the middle of an
   input never leaves the keeper blocked in a write; `<launch>.fed` is written only after the
   last byte of the last initial input was written. When the CLI ends first (a write fails with
   `EPIPE`, or the loop sees the exit), the keeper writes no `<launch>.fed` and goes on with
   step 3;
3. ignores SIGINT, SIGTERM, SIGHUP and SIGPIPE (the CLI gets the default handlers back before
   `exec`), waits for the CLI, writes the CLI's wait status to `<launch>.exit` (atomically, by
   rename) and exits with the CLI's exit code, or 128 plus the signal number when a signal ended
   the CLI; it never outlives the CLI;
4. on SIGUSR1 closes its write end; this is the new meaning of "close stdin": the runner closes
   its own write end and sends SIGUSR1 to the keeper after `process_matches`.

The runner opens its own write end with `O_WRONLY | O_NONBLOCK`; `ENXIO` (no reader) means the
turn takes no more input and is never a hang. After a restart the next runner opens the recorded named pipe
of the turn's current launch again and writes follow-ups as before; the last clause of D-M2-35-3 ("an adopted turn takes no follow-up")
is replaced, and the rest of D-M2-35-3 (follow-ups held before the turn starts, rewritten with
their original `uuid`) stays.

*Launch handshake: every identity is persisted before the CLI runs.* Today the record is written
only after the CLI started (`runturn.py` 626 to 641: `Turn.start`, then `read_identity`, then
`_persist`), so a runner that dies in between leaves a CLI that no record names. A run has one or
more processes (attempts): the first one, and after a failure the process that `_recover` picks
(`runturn.py` 515 onwards): the compaction (`kind="compaction"`), the retry after it (with or
without `hint`), `ResumeSameReference` or `NewSession`. Every attempt, the first included, starts
through the same handshake. Its files are named per launch: `<launch>` is
`<runner.state_dir>/turns/<run_id>.<attempt>.<try>`, where `<attempt>` is the attempt's number in
the run (the first process is 1, each recovery process adds one) and `<try>` counts the launches
of that attempt (a launch repeated after a crash adds one). Every file of a launch carries this
name, the named pipe `<launch>.stdin` included, so a keeper of an earlier try, slow to exit, can
only open and write files of its own try and never touches the files of the next one. With the
keeper the order is fixed, and each step is persisted before the next one starts:

1. The runner computes `process_start` (the stream offset where the new process will begin) and
   writes the record in phase `launching` (D-AR-14) with: the attempt's number and kind
   (`first`, `compaction`, `retry`, `resume_same_reference`, `new_session`) and what the kind
   needs to build the command line again (resume flag, `hint`, the session reference, a new
   reference and `bind_failed`); `<try>`; `process_start`; the named pipe `<launch>.stdin`; the paths of the
   keeper's identity file `<launch>.keeper`, the CLI's identity file `<launch>.cli`, the exit file
   `<launch>.exit` and the feed mark `<launch>.fed`; the attempt's initial inputs with their
   `uuid` (the prompt line of the first attempt, the `/compact` line of a compaction, the
   unanswered inputs of a recovery attempt in their order with their original `uuid`; these are
   the inputs of ledger rule 3, in the record before the CLI can have one); `inputs_written =
   false`; and `go_sent = false`. It writes the initial inputs to `<launch>.inputs` atomically by
   rename. Then it creates the named pipe `<launch>.stdin` and removes the named pipe of an
   earlier try of the run, if one is left. A keeper that cannot open its named pipe writes the
   exit file with the label `not_started` and exits.
2. It starts the keeper with one extra descriptor, the read end of an anonymous launch pipe whose
   write end only the runner holds (`pass_fds`, every other descriptor closed). The keeper opens
   the named pipe (step 1 of the list above), writes `<launch>.keeper` (its PID, which is its
   group and session, and its start time) atomically by rename, and blocks reading the launch
   pipe.
3. The runner reads `<launch>.keeper`, checks it with `process_matches`, and writes the record
   with the keeper's identity and `go_sent = true`; only then it writes the go byte into the
   launch pipe and closes its end.
4. On the go byte the keeper forks; the child waits on a second pipe from the keeper before
   `exec`. The keeper writes `<launch>.cli` (the child's PID and start time) atomically, then
   releases the child, which execs the CLI. So no CLI runs before a file names it.
5. The runner reads `<launch>.cli` and writes the record with the CLI's identity in phase
   `process`.
6. The keeper, not the runner, writes the initial inputs from `<launch>.inputs` into the named
   pipe (keeper list, step 2), once, and then writes `<launch>.fed`. The runner writes no
   follow-up or wake input and does not close stdin before `<launch>.fed` exists, which keeps
   them behind the initial inputs; until then they stay queued in the record, as follow-ups held
   before the turn starts (D-M2-35-3). It records `inputs_written = true` only after
   `<launch>.fed` exists. If the keeper ends without `<launch>.fed` (its identity no longer
   matches), the initial inputs may be cut off in the middle of a line, so the runner writes
   nothing more into that pipe: it closes its own write end and `inputs_written` stays `false`; a
   CLI that still lives then sees end of input (the case "keeper killed" of D-AR-7), and the end
   of the process is handled as every process end (for an adopted turn by the adoption rule of
   D-AR-14); no input is written again. A keeper that lives without `<launch>.fed` waits on a CLI
   that does not read its input; that is a stalled turn, shown by `run.stall_after` and stopped
   as in 18.6. The runner never writes an initial input itself, so a runner crash at any point
   can neither lose nor repeat one: the keeper that got the go is the only writer of these
   lines, and only for its own try.

If the launch pipe ends without a go byte (the runner died or abandoned the launch before step
3 finished), the keeper starts no CLI, writes the exit file with the label `not_started`, removes
nothing else and exits; a child that sees its release pipe end without a release exits without
`exec`. The next runner treats a record in phase `launching` by what the files of the recorded
try show, and in every case continues exactly the recorded attempt (its number, kind, command
line and initial inputs from the record); it never starts the run over from `preparing`, never
takes the recovery decision that led to this attempt again, and never runs an earlier attempt
again. Before any launch of the attempt it passes the launch check of D-AR-14:

- `go_sent = false`: no CLI of this try can run. The record names no keeper yet (that comes with
  step 3). A keeper named in `<launch>.keeper` is awaited until it exits (at most
  `runner.kill_grace`, then SIGKILL to the keeper's group, which holds no CLI); a keeper that has
  not written the file yet sees the launch pipe end, exits `not_started` by itself and touches
  only files of its own try. The runner removes the named pipe of this try and launches the same
  attempt again with `<try>` plus one from step 1; this is safe because no CLI of this attempt
  ran and no initial input of it was written.
- `go_sent = true` and `<launch>.cli` names a live CLI (`process_matches`): the CLI runs, so the
  turn is adopted per D-AR-2 from `process_start`; nothing is launched again. With
  `inputs_written = false` the adopter follows step 6: it writes no follow-up before
  `<launch>.fed` exists, records `inputs_written = true` only then, and handles a keeper that
  ends without `<launch>.fed` as step 6 says; it writes no initial input, because the keeper
  delivers them exactly once with their original `uuid`.
- `go_sent = true`, `<launch>.cli` present and the CLI gone: the process ran and ended; the
  runner handles the end by the adoption rule of D-AR-14 (classification from the stream and the
  exit file, then `_recover` with the next attempt number within the once-per-run limits the
  record proves, else the finish, D-AR-7 for lost work), never with a second launch of this
  attempt.
- `go_sent = true` and no `<launch>.cli`: while the keeper lives the runner waits for the file or
  the keeper's exit (at most `runner.kill_grace`); a keeper that hangs is stopped with SIGKILL to
  its group. The keeper may still write `<launch>.cli` and release the child between the
  runner's last look and the keeper's exit or the SIGKILL, so only after the keeper's group is
  gone does the runner read `<launch>.cli` once more, and that read decides. A file present now
  counts as a CLI that may have run and read input: the case "`<launch>.cli` present and the CLI
  gone" above applies (adoption rule of D-AR-14), and the attempt is never launched again with
  the same inputs. Only a file still absent (or the exit label `not_started`) proves that the
  keeper started no CLI and wrote no input (the go byte was persisted but never written, or no
  child was released); then the same attempt is launched again as for `go_sent = false`.

So in every window between the record of `launching`, the keeper start, the record of its
identity, the go byte, the CLI's exec and the feed of the initial inputs, of every attempt of a
run, a crashed runner leaves either one recorded CLI that the next runner adopts, with its initial
inputs delivered exactly once and its end handled by the adoption rule of D-AR-14, or no CLI
and a launch of the same attempt that the next runner repeats once; never a CLI that no record names, never two CLIs for one attempt, never an input
answered twice, and never a preparation or an earlier attempt repeated. An orderly handover
(D-AR-2) in phase `launching` before step 3 closes the launch pipe without a go byte; after step 3
it hands the turn over like one in phase `process`.

Consequences, each part of AR-03:

- Identity. The keeper's PID is its process group and its session; the CLI's PID differs, its
  group is the keeper's. The turn record stores both identities separately (PID and start time
  each, plus the one group), written in the order of the launch handshake, and
  `process_matches` checks both: the turn is alive while the CLI
  matches; a matching keeper without a matching CLI is a keeper about to exit.
- Exit code. The runner that started the keeper is its parent and reaps it, so it gets the CLI's
  exit code from the keeper's own exit code. An adopting runner is nobody's parent (the keeper is
  re-parented to launchd, which reaps it); it learns the end by polling the identities, as for
  adopted turns today, and reads the wait status from `<launch>.exit`. `exit_label` and the
  failure classification use that file in both cases; when it is missing (keeper killed),
  `exit_label` is `unknown` and the classification uses the stream alone, as today for adopted
  turns (`runturn.py` sets `exit_label="unknown"` there).
- Stop. 09 18.6 signals the one group of keeper and CLI: SIGINT and SIGTERM reach the CLI and its
  background tasks in that group (the keeper ignores both), SIGKILL ends all of them, keeper
  included. "No process of the group is left" stays the end condition. A SIGKILL to the keeper
  alone closes its write end; the CLI then sees end of input once the runner's end is closed too,
  which is the case "keeper killed" of D-AR-7.
- When the CLI dies on its own, the keeper exits right after it, so no keeper is left over.
- Proof: the probe of section 2.4 showed each of these behaviours under a scratch launchd job
  (survival of `kickstart -k` with and without the SIGKILL of `uv`, write after adoption, close by
  SIGUSR1, exit file); AR-03 and AR-09 turn it into tests 2 and 3.

Reason: the CLI kills background work only after end of input; a writer that lives with the turn
in the turn's own session, not with the runner and not in the launchd job's group, removes that
cause for restarts and for runner crashes. Rejected: starting the keeper as a plain child of the
runner (it would be in the launchd job's process group, which gets SIGTERM at the restart and per
`launchd.plist(5)` is killed, section 2.4, bringing failure (b) back; that a keeper ignoring
SIGTERM outlived it in the probe is no guarantee the design may rely on); a double fork with
`setsid` instead of `start_new_session` (same result, but the runner would lose the keeper as its
child and with it the exit code of a turn that never restarted); moving the keeper into the CLI's group after the start
(`setpgid` across sessions is refused, section 2.4); setting `AbandonProcessGroup` in the plist
(it would also leave every other helper of the runner alive, and an installation without the key
would silently lose turns again); setting the CLI ceiling to 0 and leaving stdin closed
(background shells are swept after a few seconds anyway, crons are dropped, follow-ups impossible); opening
the named pipe read and write in the CLI itself (it would never see end of input, so it could
only end by a signal); a separately installed supervisor service (a second program to install and
update, D-A1-3 rejected the same).

**D-AR-4 The handover is on only with a key.** `runner.turn_handover` (default off). Without it
the runner starts the CLI with an anonymous pipe and stops turns per D-AR-1, as today plus the fix.
Reason: the keeper changes how every turn starts; the byte-equal fixtures of section 1.1 item 6
must hold without the key.

**D-AR-14 Runs between processes survive a handover.** The turn record is written when the runner
accepts a run, not only when a process starts, and carries the run's phase: `preparing`,
`waiting_lock` (`thread_busy` or `session_busy` wait), `transcribing`, `launching` (the launch
handshake of D-AR-3, with the attempt's number and kind, `<try>`, its initial inputs,
`inputs_written`, `go_sent` and the files of the launch), `process`, or `compaction_retry`
with the retry's input and the new session reference. Every record from `launching` on also
carries the run's recovery flags (09 18.9), so it shows which recovery kinds the run already
used. The phases `preparing` and `waiting_lock` lie before `run.start` (09 4.9): the run is
still `queued` on the server and, while it waits on a lock or a slot, listed in `waiting_runs`
(09 5.2.1), and its adapter inbox entry is still `ACCEPTED`. The phases from `transcribing` on
lie after `run.start`, so the run is `running`. On a handover the runner abandons the
in-memory work of the first three phases (nothing irreversible has happened: no CLI process ran
for this run) and leaves the record; in `launching` it closes the launch pipe without a go byte
when the go is not yet recorded, else it hands the turn over like `process`. The adopter treats
a record by phase. A run in `preparing` or `waiting_lock` continues through the existing inbox
restart of 09 4.9: the SDK starts its `ACCEPTED` inbox entries again after the restart
(`adapter.py`), which prepares the run anew, takes the locks and calls `run.start` once; the
record of such a phase is only removed, never used to start the run, and no launch check runs
for it (the run start does its own checks). The record-based start over and the launch check
apply only from `run.start` on, to `transcribing`, `launching`, `process` and
`compaction_retry`. For a run still `running` on the server, `transcribing` starts over from the
record in the next runner (no CLI ran for the run yet); `launching` follows the cases of the launch handshake for the recorded attempt,
whatever its kind (D-AR-3: launch the same attempt again with the next `<try>` only when no CLI
of it was released, else adopt the recorded CLI or handle its end by the adoption rule below;
never back to `preparing`); `process` is adopted per D-AR-2, and its end follows the adoption
rule below; with `inputs_written = false` in `process` (the runner crashed after the CLI
identity was recorded and before it recorded the feed) the adopter follows step 6 of D-AR-3
exactly as for `launching`: it writes no initial input and no follow-up or wake input before
`<launch>.fed` exists, records `inputs_written = true` only then, and handles a keeper that ends
without `<launch>.fed` as step 6 says; `compaction_retry` starts the recorded retry, the decision of `_recover` already
taken, through the launch handshake. Every process, the first, the compaction, the retry,
`ResumeSameReference` and `NewSession` alike, starts through the launch handshake, and the
record in `launching` carries the attempt's kind, number, command line inputs and initial inputs
until its CLI is recorded. A run from `transcribing` on stays in the first heartbeat's
`active_runs`, so the server never sees it lost; a run in `preparing` or `waiting_lock` stays
`queued` (in `waiting_runs` while it waits) until its single `run.start`; a wake run caught in
any phase keeps its chain.

*One adoption rule: an adopted process ends like an own one.* With `runner.turn_handover`, the
end of a process that the runner did not start itself is handled exactly like the end of its
own process. This holds whatever the record's phase (`launching` with a recorded CLI, or
`process`), whether the CLI ended while no runner ran or later under the adopter, and whether
the turn came by an orderly handover (D-AR-2) or after a crash. The runner classifies the end
from the stream (replayed and read lines) and `<launch>.exit` (`exit_label` `unknown` when the
file is missing, D-AR-3); then `_recover` may pick the next attempt with the next attempt number,
as for an own turn, and that attempt starts through the launch handshake after the launch check
below. The once-per-run limits of `_recover` stay (each recovery kind at most once per run, the
flags `compacted`, `session_replaced` and `session_resumed` of `recovery.py`), and the record is
their proof: the recovery flags are part
of the next attempt's `launching` record, written before its keeper starts, so an adopter sees
every recovery the run already used, also one whose attempt never started, and never uses a kind
twice. When `_recover` picks nothing (no recovery fits, or its limit is used), the run finishes
by the classification, and lost work leads to D-AR-7. A runner that dies after an attempt ended
but before the next attempt's `launching` record leaves the record in `process` with the CLI
gone; the next runner applies this rule to it and takes the recovery decision that the dead
runner had not recorded yet, which is safe because no keeper of the next attempt started before
that record. Unchanged exceptions: a live turn whose run the server already ended is stopped as
in 18.6 without a finish and without a new process (09 18.9, D-AR-7); a recorded process that
ended (in `launching` with a recorded CLI, or in `process`) of a run the server already ended
(`stale` after `run.orphan_timeout`, `interrupted`, or ended otherwise) gets no finish, no
`_recover` and no new process, only D-AR-7 through `run.wake.recover`; a record without
`process_start` (written without `runner.turn_handover`) is adopted as today and finished once
without recovery. This amends 09 18.9 ("finish once by the classification above without
recovery" and "never a new process for such a run") for records written with
`runner.turn_handover`; the sentence "a process keeps its kind, so an adopted compaction is never
posted and never completes the run" stays true, because the compaction's own output is still not
posted and the retry after it is a new attempt that posts and completes the run. It also amends
R-AG-17 by a clarification: such an attempt is no retry of an `interrupted` or `stale` run (the
run is still `running`); it is the attempt an own runner would start, and it repeats no input
an earlier attempt answered (only unanswered inputs, with their original `uuid`, ledger rule 3).

*Launch check before every launch by a runner that did not accept the run.* After an outage or
a handover the agent's tool class, the person's rights or SR2 may have changed while no runner
ran. Before a runner launches any process of a run it did not accept itself (a repeated try of
D-AR-3, the next attempt by the adoption rule, the retry of `compaction_retry`, or a start over
from `transcribing`; a run in `preparing` or `waiting_lock` restarts through the inbox and is
not checked here), it calls `run.launch.check` (adapter), and
the server checks against the current state as `run.wake.check` does: the run is
still `running`; its person still has `trigger` on the agent in the channel and the agent is
still allowed there; billing mode rules of 10; for tool class `host`, the agent's `tool_class`
is still `host` and SR2 (07) holds for that person. The runner also compares its own actual tool
class and billing mode with the answer (`compare_actual`, as for a token). Only on allow does it
launch. The check extends today's pre-check before a recovery process (`precheck` of
`recovery.py`, `runtime_not_authenticated` or `billing_mode_mismatch`), which still runs. A
refusal launches nothing, finishes the run `failed` with the reason of the failed check
(log code `launch_check_refused`), and lost work leads to D-AR-7; a refusal because the server
already ended the run changes nothing on the server (no finish), and lost work leads to D-AR-7
through `run.wake.recover`; when the server cannot be
reached, nothing launches, the check is tried again with backoff, and the run stays in
`active_runs`.

Reason: long steering sessions compact often, and a restart during preparation, launch or
compaction, or a handover of a session that later fills its context, must neither drop the
input nor end the run; one rule for every adopted end keeps the result independent of the
moment of the restart. A restart before `run.start` needs nothing new, because the inbox restart
of 09 4.9 already continues such a run. Rejected: handing a run after `run.start` back to the
server as `queued` (a new state
transition from `running`, and it would not cover the compaction retry, whose first process
already consumed input); recovery only for a recorded `compaction_retry` and none for other
adopted ends (the same CLI end would then complete or fail depending on the phase the record
happened to be in).

## 5. Decisions: wake-ups

**D-AR-5 A wake-up is a server record; the runner evaluates host conditions and fires.** A new
table `run_wakes` holds per wake-up: agent, thread, the run that asked for it (`source_run_id`),
the person on whose behalf it runs (`triggered_by` of the source run), kind, condition, a short
label written by the agent, the reason (`agent`, `recovery`, `usage_limit`), `not_before` (at least
creation plus `run.wake_min_delay`), `deadline` (at most `run.wake_max_wait`), state (`pending`,
`fired`, `cancelled`, `expired`, `refused`, `failed`), the reason of the last state change, and the
run or wake input it produced. For `pr_checks` it also stores the head commit of the pull request
at creation (section 12). Kinds:

| Kind | Condition | Evaluated by | Allowed when |
|---|---|---|---|
| `at` | a time, or "after N minutes" turned into a time | server job `runs.wake_due` | always (within D-AR-9) |
| `pr_checks` | repository and pull request number; fires when no check is pending, with the result | runner, with the host user's `gh`, read only | the source run's person is an owner of the instance |
| `command` | a command line run in the turn's working folder; fires on exit code 0 | runner, each execution only after `run.wake.check` allowed it | tool class `host` (SR2, 07), at create and before every execution |

`pr_checks` is restricted to owners because it reads GitHub with the owner's `gh` login; a run of
another person (for example a `scoped` run in a shared channel) could otherwise read any
repository the owner can read and get the result text into its thread, which is wider than 18.2
allows `scoped` and conflicts with the target of 21 (Z3, Z4: no owner secrets in a person's
environment). A refused create says `pr_checks_owner_only`.

Operations:

- `run.wake.create` (adapter, agent token, only for an active run of that agent in that thread),
  with reason `agent` (from `bothy-wake`) or `usage_limit` (D-AR-12); a `usage_limit` request
  carries the outage number as idempotency key, so a repeated request of one outage updates its
  pending wake-up instead of adding one, and a new outage gets its own (bounded by
  `run.usage_wake_max`).
- `run.wake.recover` (adapter, agent token): creates a wake-up of kind `at` with reason `recovery`
  or `usage_limit` for an ended source run of that agent and thread in state `interrupted`,
  `stale` or `failed`, at most one per source run and reason (idempotency key). It exists because
  recovery wake-ups are created after the run ended, often by the next runner after a crash, and
  because a usage-window end may only be seen after a restart (D-AR-7, D-AR-12).
- `run.wake.list_pending` (adapter): the agent's pending wake-ups of every kind, `at` included,
  called at start and after every reconnect, so `pr_checks` and `command` checks survive a runner
  restart and crash, and the end-state check of D-AR-8 knows a `--after` or `--at` wake-up set
  before the restart.
- `run.wake.check` (adapter, agent token): asked by the runner before every execution of a
  `command` check and every read of a `pr_checks` check; answers allow or ends the wake-up
  (below).
- `run.wake.fire` (adapter, for `pr_checks` and `command`, with a short result text).
- `run.wake.cancel` (the agent, the person of the wake-up, or a person with `manage` on the
  channel).
- A read for clients.
- The event `run.wake_changed`, delivered to clients of the thread and to the adapter inbox of
  the wake-up's agent, so a runner learns of new, cancelled and expired wake-ups between reads.

Firing re-checks everything a new run needs at that moment, as for a human message: the person
still has `trigger` (3.4), the agent is still allowed in the channel, billing mode rules of 10 (I1
to I5), and the autonomy rules of D-AR-9. A fired wake-up creates a run with the new
`trigger_kind = 'wake'` and `triggered_by` of the source run, so the run is delivered, tokened,
billed, shown and stopped like every run, and the runner resumes the thread's session with
`--resume` (09 section 9). If the thread has an active run at fire time, D-AR-13 applies, so at
most one run per thread stays true (R-DM-14).

*Checks before every host execution.* A `command` check runs agent-supplied code on the host long
before the wake-up fires, so the fire-time checks come too late for it. Before every execution of
a pending `command` (and every `gh` read of a pending `pr_checks`) the runner calls
`run.wake.check`, and the server checks against the current state: the wake-up is still
`pending`; the wake-up's person still has `trigger` on the agent in the channel (3.4) and the
agent is still allowed there; for `command`, the agent's `tool_class` is still `host` and SR2 (07)
holds for that person as at a delivery (an active owner, or SR3 with a ready dev environment); for
`pr_checks`, the person is still an owner, and no tool class is required, as at create (the table
above; D-AR-6 offers `bothy-wake` under `scoped`, so an owner's `scoped` run may create one). The
runner also checks its own actual tool class against the
answer (`compare_actual`, as for a token). Only on allow does it run the command, once. Any
failed check ends the wake-up `cancelled` with the reason of the failed check
(for example `capability_missing`, `agent_not_reachable`, `tool_class_owner_only`,
`dev_environment_not_ready`, `tool_class_mismatch`, `pr_checks_owner_only`), an
audit entry naming the check and the wake-up, the log code `wake_check_refused` and the note of
the next paragraph, and the command is not run. When the server cannot be reached or does not
answer, nothing runs (fail closed), and the check is tried again at the next interval; this
counts towards `runner.wake_check_failures`.

Every end other than `fired` posts a `system` message in the thread (never triggers, 09 3.2) that
names the label and the reason, for example "Wake-up 'CI of #123' expired after 7 days" or
"Wake-up refused: you no longer have the right to trigger this agent here".

Reason for the server as the home: a wake-up must survive the runner's state folder, be visible
to the person in the thread and be cancellable from any client; only the server can start a run,
issue its token and apply the trigger and billing rules at fire time. Conditions that need the host
(GitHub access, a command) can only be checked on the runner host, so the runner evaluates them.
Rejected: wake-ups only in the runner's state folder (invisible, not cancellable, lost with the
folder, and the runner would still need a server operation to start a run; the task text proposed
this, and this note departs from it for these reasons); a server webhook from GitHub (needs a
GitHub app and public reachability per installation, an external account); the CLI's own crons
(section 2.1).

**D-AR-13 A wake-up that fires into an active run is a wake input with its own token.** When the
thread has an active run at fire time and that run's `triggered_by` is the wake-up's person, the
server delivers a wake input to the runner over the follow-up channel. Its token has
`trigger_kind = "wake"` and carries `run_id`, `thread_id`, `wake_id` and `sub` (the wake-up's
person); there is no message and no author. The runner writes the input line (label, reason,
result text) into stdin only when: the token is valid and not expired; `trigger_kind` is `wake`;
`run_id` and `thread_id` match the active run; `wake_id` is the delivered wake-up; `sub` equals
the run's `triggered_by`; and billing mode and tool class match (`compare_actual`, as for
follow-ups). Otherwise it skips the input with the log code `wake_input_check_failed`, and the
server puts the wake-up back to `pending` so it fires after the run as a wake run. When the
active run belongs to another person, the wake-up stays `pending` until the run ends. The D-AR-9
checks run at fire time on both paths, and a wake input counts as one autonomous input exactly
like a wake run. After a handover the adopting runner takes wake inputs like follow-ups, on the
replayed state of D-AR-2: while a task of the held process runs, the `result` of the wake input
does not close stdin. Reason: 18.5 checks every follow-up against a message and its author
(`followups.py` lines 110 to 128), which a wake-up does not have; without its own checks and
count, a turn held open by a long background task could take one wake input per minute forever
while the run count never rises. Rejected: always waiting for the run to end (a held steering
run may last days, and the wake-up exists to reach it); writing wake inputs without a token (the
runner would write into a run on a claim it cannot check).

**D-AR-6 The agent asks through a runner command, `bothy-wake`.** With `runner.wakes` on, the
runner puts a folder with the command `bothy-wake` first on the turn's cleaned `PATH` (owned by the
runner user, not writable by group or others, 18.2) and adds `Bash(bothy-wake:*)` to the allowed
tools under `scoped`. The command takes the kind and condition as arguments (`--after 20m`,
`--at 17:55`, `--pr owner/repo#123`, `--until-success '<command>'`, `--label`, `--deadline`), hands
the request to the runner through a request file in the run's folder, and prints the runner's
answer: the wake-up's id and when it fires, or the refusal with its reason (exit code non-zero).
The command of `--until-success` never runs without the checks of D-AR-5 before each execution
(`run.wake.check`), so a person who lost `trigger` or an agent that lost `host` stops it before
its next run, not only at fire time.
`bothy-wake --list` and `bothy-wake --cancel <id>` work on the thread's wake-ups. With the key on,
the turn also gets `--disallowedTools CronCreate CronDelete CronList ScheduleWakeup RemoteTrigger`
(the names are in the pinned CLI strings), so the agent cannot set a timer that dies with the
process. Reason: a command works in every class that has Bash and is visible in the stream as a
normal tool call; the request file keeps the token out of the turn. Rejected: an MCP server of the
runner (`chat` disallows `mcp__*`, `--mcp-config` changes the command line in every turn, and
D-A1-8 already uses `--strict-mcp-config` for hosted agents); a marker line in the reply like
`TITLE:` for the request itself (it would be posted text, and the agent would get no answer
whether it worked); translating the CLI's cron calls (two timers for one intent, and the CLI may
also fire it while stdin is open). Tool class `chat` has no tools and therefore cannot request
wake-ups; a `chat` session keeps working on its own only through recovery and usage-window
wake-ups and through replies, so the owner request "every session, chat or development" is met
fully for development sessions and only partly for chat. The end-state check of D-AR-8 applies
to both.

**D-AR-7 Lost work leads to a recovery run that says what was lost.** When a turn ends with
background tasks that did not finish, the runner calls `run.wake.recover` with reason `recovery`,
if the autonomy rules of D-AR-9 allow it. The cases:

- the next runner finds a recorded turn whose process is gone and starts no further attempt for
  it (the adoption rule of D-AR-14 finds no recovery that fits or its limit used, or the record
  was written without `runner.turn_handover`: adoption "leave", `app.py`); for a run the server
  still shows `running` and that has no `result` or decided end to finish by, the runner first
  ends it `failed` itself, the way it ends a run from the log today, with the cause AR-01 names,
  so `run.wake.recover` finds an ended run and the thread does not wait for
  `agent.presence_timeout`;
- adoption finds a live turn whose run the server already ended (`stale` after
  `run.orphan_timeout`, `interrupted`, or ended otherwise) and stops it (`adoption.py` line 143,
  "alive and run ended: stop", 09 18.9);
- an error `result` was followed by the stop of 18.5;
- the CLI wrote the ceiling line;
- a second `context_full` ended the run;
- the keeper was killed and the CLI saw end of input.

The recovery run starts with a text that lists each lost task (id, kind, description from
`system/background_tasks_changed`, for an adopted turn from the replay of D-AR-2, how it ended) and says that nothing was re-run. Without
permission the thread gets the visible note instead. This amends R-AG-17: the server still never
retries the input of an `interrupted` or `stale` run; the recovery run is a new run that only
informs the agent, which decides what to do again. Reason: the owner asked for work to go on after
a loss; repeating side effects blindly is what R-AG-17 forbids, and this does not.

**D-AR-12 The usage window marks a run usage-limited, keeps its background work and wakes it at
the reset.** The runner reads every `rate_limit_event` frame (AR-06 adds the type to the stream
reader's `KNOWN_TYPES`, which skips it today, section 2.5). A frame whose
`rate_limit_info.status` is `rejected` while `overageStatus` is not `allowed` or
`allowed_warning` (no overage takes over the request) marks the turn usage-limited until the
frame's `resetsAt`, with its `rateLimitType`; a later frame with `status` `allowed*` clears the
mark. Only `rejected` sets the mark: a `status`
outside the three values of the pinned schema (section 2.3) is logged (`usage_status_unknown`)
and treated as allowed, so a new harmless value never turns error ends into `usage_limited`; a
real rejection without a `rejected` frame takes the path of the last paragraph. The mark
survives a handover through the replay of D-AR-2. While the mark is set:

- error `result`s of the turn are usage errors: they set the usage error mark of D-AR-2, never
  the failure mark, are classified `usage_limited`, never `runtime_error`, and never lead to the
  stop of 18.5 after an error `result`, also not after the usage mark clears (the failure mark
  that the stop reads was never set by them); stdin stays open per the background rule, the
  process stays held and the run stays `running`, and the process ends on its own when the tasks
  finish (their notification turns may be rejected too; they cost nothing and change nothing). An
  error `result` after the mark cleared is an ordinary error again and sets the failure mark;
- each outage gets its own wake-up. An outage starts at a `rejected` frame that sets the mark
  while it is clear and ends when a later frame clears it. At the outage's first such frame the
  runner calls `run.wake.create` with kind `at`, reason `usage_limit`, `not_before = resetsAt`
  plus `run.wake_min_delay` and the run's outage number (counted per run across attempts, D-AR-2) as idempotency key, if the autonomy rules of
  D-AR-9 allow it; the wake-up counts in the chain like every autonomous input. A later `rejected`
  frame of the same outage with a later `resetsAt` (another window) moves `not_before` of that
  wake-up while it is pending (the same call with the same key updates it). When the mark clears
  while the outage's wake-up is still pending, the runner cancels it (`run.wake.cancel`, reason
  `usage_cleared`), so the next outage, after the first wake-up fired or was cancelled, gets a
  new wake-up of its own. The server bounds this by `run.usage_wake_max` (server key, default
  20): the usage-limit wake-ups of one run, every state counted; at the cap a further one ends
  `refused` with the reason `usage_wake_max` and a note in the thread, and a reply of the person after the
  reset continues. If the process is still held when a wake-up fires, it arrives as a wake input
  (D-AR-13) that tells the agent the window has reset; otherwise it starts a wake run that resumes
  the session;
- the live box and the thread show "Usage limit of the account reached; continues at <time>", or,
  without autonomy permission, that a reply after the reset continues;
- when the process ends while the mark is set, or with a usage error as its last error `result`
  and no later answer, the run ends `failed` with the new failure cause
  `usage_limited`, carrying `resetsAt` and whether a wake-up is pending; if the runner learns of
  the mark only after the run ended (a restart in between, the frame read from the stream file),
  it calls `run.wake.recover` with reason `usage_limit` instead.

`allowed_warning` changes nothing but the log (`usage_warning`) and a hint in the live box. The
classification keys on the frame, not on the `result` text, because the CLI's wording is
unverified. If a rejected request ever ends a turn without a preceding `rejected` frame (unverified,
section 2.3), the end is `runtime_error` as today and the lost background work leads to a recovery
run (D-AR-7); AR-01 records the CLI's behaviour from the pinned strings so this case is known.
Reason: a session that runs on with enough tasks reaches the window by construction; ending it
silently or killing its background work is failure (c). Rejected: treating it as `runtime_error`
(today's path, which kills background work and wakes nothing); ending the run at once while the
process lives on (at most one run per thread, R-DM-14, and the process is the run); polling the
window (the frame already carries `resetsAt`).

**D-AR-8 An end state on every run, checked before stdin closes.** The runner prompt requires a
last line `STATE: done`, `STATE: waiting` or `STATE: blocked` in every answer, handled like the
existing `TITLE:` line: the runner removes it from the posted text and stores the value in the
run's finish summary. Meanings: `done` (nothing more to do), `waiting` (more work after something
happens), `blocked` (needs a person). A missing or malformed marker counts as `waiting`.

When the runner is about to close stdin at a `result` without error and no task runs, it reads
the state. For `waiting` with no pending wake-up of the thread, it writes one input line into the
still open stdin: "You said you are waiting, but nothing will wake you: start a background task,
request a wake-up with bothy-wake, or end with STATE: done or STATE: blocked." If the next
`result` still leaves `waiting` with nothing pending and no task running, stdin closes and the
thread gets the note "Ended waiting, but nothing will wake this thread" (log code
`continuation_without_wake`). There is at most one nudge per waiting phase: a phase starts with
an input of a person (the run's prompt or a follow-up) or a wake input, and only such an input
starts a new one; notification turns of background tasks and the answer to the nudge stay in
the phase. So "wait, nudge, start a background `sleep`, its notification turn ends `waiting`"
gets no second nudge: if nothing runs and nothing is pending, stdin closes with the note, and if
the agent keeps starting such tasks, the idle count of D-AR-9 ends it. The nudge mark survives a
handover through the replay of D-AR-2. The answer to the nudge is posted like any follow-up
answer. For
`chat` (no tools, no wake-ups) the nudge is skipped and the note follows at once. Every finished
run's summary carries the end state and whether a wake-up is pending and when; clients show
both. Reason: 09 3.2 wants freedom from loops as a data field, not a text heuristic; a required
marker is a data field the agent sets, a missing one fails closed (09 1, item 4) towards the note,
and the pending flag shows the rest. Rejected: a phrase list over the answer (the first version of
this note: any promise worded differently ended silently); a model call to judge every end (cost
per run); no check (failure (a) stays silent).

## 6. Decisions: cost safety and limits

**D-AR-9 Who may run on alone, and how far.** An autonomous input (a wake run, a wake input, a
recovery or usage-window wake-up) is allowed when the source run's person is an owner of the
instance, or when the channel has the new switch `allow_autonomous_runs` (column of `channels`,
default `false`, changed with `manage`). The server checks this at create and again at fire time.

- **Chain count.** Kept per thread and person (the wake-up's `triggered_by`): consecutive
  autonomous inputs since that person last triggered a run or wrote a follow-up in the thread.
  Only a run or follow-up of that same person resets that count, so a message of another person
  never extends an owner's chain. When the count reaches `run.autonomous_chain_max` (server key,
  default 25, `0` means no cap), the next wake-up ends `refused` with the reason
  `autonomous_chain_max` and a note that a reply of that person continues.
- **Idle count.** Kept the same way: consecutive autonomous inputs that ended without any tool call
  other than `bothy-wake` and without starting a background task. The runner reports the idle
  flag per input, not per run: at the `result` that ends a wake input's turns, and at the end of
  a wake run's first turn, from the stream, on the run's patch, so a held run that takes wake
  inputs for days is counted after each one. When the count reaches `run.autonomous_idle_max`
  (server key, default 3, minimum 1, cannot be switched off), the chain ends with the reason
  `autonomous_idle_max` and a note, also when the chain cap is `0`; the server refuses further
  wake inputs into the held run, as at the chain cap.
- **Notification turns of a held process.** A turn the CLI starts by itself after a background
  task ended is no autonomous input and does not count in the chain (a person or wake-up started
  the work). It counts in the idle count when it made no tool call other than `bothy-wake` and
  starting background tasks, so a self-poll ("start `sleep 60` in the background, wake to its
  notification, start it again") is idle and stops at `run.autonomous_idle_max` also when the
  chain cap is `0`; a notification turn that does other work resets the idle count. The runner
  reports the flag on the run's patch like a wake input's; when the server's answer says the cap
  is reached, the runner closes stdin at that turn's `result` and the thread gets the note; the polling task
  ends with the process, which is the visible end the cap intends.
- **Minimum delay.** Every wake-up fires no earlier than `run.wake_min_delay` (default 60 s)
  after it was created, also `--at` in the past, `--after 0` and a `command` that succeeds at
  once; the first check of `pr_checks` and `command` happens no earlier either.
- **Pending cap.** Pending wake-ups per thread are capped by `run.wake_max_pending` (default 5).

Two threads that wake each other through shared files or a pull request each count their own
chain; the idle count stops them when they do nothing else, the chain cap otherwise.

Reason: autonomous runs spend money (organization) or quota (personal) without a person present;
owners already carry SR2 and billing decisions, and a channel switch makes the opt-in explicit for
others. Counting inputs, not runs, closes the path through wake inputs (D-AR-13); counting per
person closes the reset by another person; the idle count and the delay stop loops that burn
quota within minutes. A count is understandable and works in both billing modes, where a dollar
budget does not: costs of `personal` runs are information only (10, 12.2). Rejected: a daily
dollar budget as the only cap (meaningless for `personal`; may come later for `organization`
together with the monthly budget left open by D-A1-20); no cap (the owner asked for cost
safety); a cap on run duration (it would recreate failure (c)); a per-thread count reset by any
person (a non-owner could buy the owner more runs).

**D-AR-10 Explicit settings for the CLI limits that remain.** A new runner key
`runner.cli_bg_wait_ceiling` (default unset, meaning the CLI's built-in default) is passed as
`CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS` when set; the environment allow list of 18.2 names it. With
D-AR-3 it applies only when the runner itself closes stdin while tasks run, which happens only
after an error `result` and is followed by the stop of 18.5 anyway. Reason: the limit becomes a
named, documented value instead of a hidden default, and the byte-equal fixtures hold without the
key.

**D-AR-11 Off until asked for.** `runner.turn_handover`, `runner.wakes` and the end-state check
(`runner.end_state`) default off; the server keys and the channel switch default to today's
behaviour (no wake-up is possible without a runner that creates one). Every new column is nullable
or has a default meaning "as before"; widening the `trigger_kind` check is an expand step
(R-OP-12). Going back one server version leaves `run_wakes` unread, but a run with `trigger_kind =
'wake'` would be unknown to the older code: AR-04 states how the older version shows it, with a
test, or the rollback note says to let pending wake-ups fire or cancel them first. The runner
reports the capabilities `wakes` and `turn_handover` in its heartbeat, each only while its key is
on, so the heartbeat stays byte-equal without the keys (test 16); clients show wake-up controls
only when the thread's agent reports `wakes`. `details.capabilities` of the heartbeat is closed
(09 4.8): a server without AR-04 refuses both keys as unknown members, and it also lacks
`run.launch.check` and `run.wake.check`, which a runner with `runner.turn_handover` or
`runner.wakes` calls. The server support of AR-04 (both capability keys in the heartbeat schema,
`run.launch.check`, `run.wake.check`) therefore runs before any runner switches either key on
(section 9), and a server rollback below AR-04 first turns both runner keys off.

## 7. Specification changes

| Chapter | Change |
|---|---|
| 01 product | a thread shows pending wake-ups with label, condition, deadline and a cancel button; the run end state and pending flag; the notes of D-AR-5, D-AR-8, D-AR-9 and D-AR-12 |
| 03 data model | table `run_wakes` with the idempotency key of `run.wake.recover` and of `usage_limit` wake-ups (source run, reason, outage number); `runs.trigger_kind` gains `wake`, with `triggering_wake_id`; failure cause `usage_limited`; `channels.allow_autonomous_runs`; chain and idle counts per thread and person |
| 04 sync protocol | event `run.wake_changed` to clients and to the adapter inbox; delivery of a wake run like a message run; the wake input with its token (D-AR-13) |
| 05 actions | `run.wake.create`, `run.wake.recover`, `run.wake.list_pending`, `run.wake.check`, `run.wake.fire`, `run.wake.cancel`, `run.launch.check` (D-AR-14), the channel switch |
| 06 API | the operations and the client read |
| 07 permissions | who may cancel; `command` only under `host` (SR2); `pr_checks` only for owners; `trigger`, tool class and SR2 checked again before every execution of a `command`, `trigger` and the owner check (no tool class) before every read of a `pr_checks` (`run.wake.check`, audited cancel on failure); `trigger`, tool class, billing mode and SR2 checked again before every launch by a runner that did not accept the run (`run.launch.check`, D-AR-14); the autonomy rule of D-AR-9 |
| 09 agents | 4.8 (heartbeat capabilities `turn_handover` and `wakes`, each reported only with its key; a server before AR-04 refuses them as unknown members); 4.9 (with `runner.turn_handover`, a crash between `run.start` and the runtime start, in the phases `transcribing` and `launching` of D-AR-14, no longer ends the run `interrupted`: the next runner continues it from the turn record after the launch check; a crash before `run.start`, in `preparing` or `waiting_lock`, keeps the inbox restart unchanged); 3.2 (a wake run is triggered by an agent action that carries a person's `triggered_by`; it is no agent message, so "two agents can never trigger each other" still holds, and the D-AR-9 caps are the loop protection for it); section 5 (trigger kind `wake`), 5.5 (handed-over turns, no heartbeat after the handover), 7 (wake input into an active run), 14 (relation to reports without a trigger), 18.2 (named pipe, keeper as session leader, launch handshake with identity files, `PATH` folder, disallowed CLI timers, `runner.cli_bg_wait_ceiling`), 18.5 (stdin rule with keeper, end state check, wake input checks, no stop after a usage-window end with running tasks), 18.6 (one group of keeper and CLI, exit code from the keeper), 18.9 (handover, phases in the record including `launching` and its cases, `process_start` in the ledger record and the replay of per-turn state into one object, adoption with follow-ups, wake inputs, the stdin rule and the stop after an error, the adoption rule of D-AR-14 replacing "finish once ... without recovery" and "never a new process for such a run" for records written with the key (an adopted process ends like an own one: classification, then `_recover` with the next attempt number within the once-per-run limits the record proves), the launch check before every launch by a runner that did not accept the run, recovery runs, stop of an ended run followed by recovery), a new 18.18 "Wake-ups and handover", 19 decisions, 21 configuration; D-M2-35-3 amended (its last clause only); R-AG-17 amended twice: the recovery run of D-AR-7 is a new run that only informs, and the next attempt of an adopted run by the adoption rule of D-AR-14 is no retry of an `interrupted` or `stale` run and repeats no answered input |
| 10 backend policy | billing of a wake run and a wake input: the rules of the source run's person at fire time |
| 13 push notifications | the person of a wake-up gets a notification for `refused`, `expired` and `failed`, and for a `usage_limited` end |
| 16 operations | a new runner service guide: the assumed launchd job form of section 2.4, restart and update with handover instead of waiting, exit code 75 after a handover, `claude-code-runner stop --turns` before `bootout` or uninstall; the operator rule of ruling 4: the restarts that install AR-02 and AR-03 and the one that switches `runner.turn_handover` on happen only while no turn runs; the server of AR-04 runs before any runner switches `runner.turn_handover` or `runner.wakes` on, and a server rollback below AR-04 turns both keys off first (D-AR-11) |
| 17 observability | log codes `turn_handed_over`, `turn_adopted`, `launch_check_refused`, `wake_created`, `wake_refused`, `wake_check_failed`, `wake_check_refused`, `wake_input_check_failed`, `wake_fired`, `wake_expired`, `recovery_wake`, `usage_limited`, `usage_warning`, `usage_status_unknown`, `continuation_without_wake`; a metric for pending wake-ups, chain and idle counts |

New rules (numbers assigned in AR-01, after R-AG-42): a restart never ends a turn unless asked
(handover); no CLI starts before the record names its keeper and the keeper's identity file names
it (launch handshake); an adopted process ends like an own one, with the same recovery attempts
within the once-per-run limits the record proves (adoption rule); a runner that did not accept a
run launches no process of it before the person, tool class, billing mode and SR2 checks of that
moment (launch check); a wake-up is a server record and fires only with the checks of a new run;
a `command` check runs on the host only after the person, tool class and SR2 checks of that
moment, a `pr_checks` read only after the person and owner checks of that moment; a wake input
into an active run has its own token and checks; every end of a wake-up other than `fired` is
visible in the thread; autonomous inputs need an owner or the channel switch and stop visibly at
the chain cap and the idle cap; lost background work leads to a recovery run or a visible note,
never to a silent end; a usage-window end wakes the thread at the reset; every run end carries
the agent's end state and the pending flag, and a `waiting` end with nothing pending is visible.

## 8. Tests

Behaviour tests, each against the real artifact where one exists (runner scenarios with a real
runner process and the fake Claude of 09 16; server tests against PostgreSQL).

Restart:

1. SIGTERM to a real runner with a live turn (the scratch probe of section 2.2 as a scenario test):
   without `runner.turn_handover` the turn is stopped, `stop_escalated` is logged and the run ends
   `cancelled` with a reason; the record is removed only after the process exited; no process
   group is left.
2. With the key, portable variant: the runner runs in its own process group like a launchd job;
   SIGTERM to the runner, which exits 75, then SIGKILL to the runner's whole process group (harder
   than what launchd did in the probe of section 2.4, so it covers both); keeper and CLI are still
   alive; a new runner starts; the turn is adopted. The fake CLI runs with
   `background_survives_eof`, so its background job ends only by its gate and never merely
   because stdin ended, and it records every end of input it sees. Before the restart the turn
   starts the background job and its `result` is checkpointed, so the job's
   `background_tasks_changed` line lies before the stored `offset`. After the restart, while the
   job still runs, a follow-up and then a wake input (D-AR-13) are written; each reaches the turn
   with its echo and its answer, and each `result` leaves stdin open (the adopter's replayed
   `BackgroundWork` holds it). Only then the test opens the job's gate: the job ends with its
   notification turn and answer, and stdin closes by today's rule after it. Asserted: the fake
   recorded no end of input before the job's notification turn (`stdin_eof` and
   `eof_with_background` absent until then), the replayed state equals the old runner's (running
   tasks, `seen`, queued inputs, `last_result_at` set, failure mark, usage error mark, task descriptions, nudge
   mark, usage mark; the whole `ProcessState` field by field apart from the read times), nothing
   of the replayed lines was posted again, the run is in the first heartbeat's `active_runs`, and
   the old runner sent no heartbeat after the handover began. Variants, each with the same
   assertions where they apply: (a) two restarts in a row, the second after the first adopter
   wrote the record again: `process_start` is unchanged in the record, the second adopter's
   replayed state equals the first one's, and the turn goes on as in the main case; (b) the
   background job ends and its notification turn is answered, then the restart falls inside the
   quiet window and no input comes after it: stdin closes `runner.followup_grace` after the
   adoption, not before (no `stdin_eof` earlier), the CLI ends and the run completes; (c) an
   error `result` while the job runs, then the job's notification turn before the checkpoint,
   then the restart: the adopter stops the group `runner.stop_grace` after the adoption, logs
   `turn_stopped_after_error` with the lost task, and the run fails as by the old runner. macOS
   variant in AR-09: the same under a
   scratch launchd job (own label, own plist of the assumed form of section 2.4, without
   `AbandonProcessGroup`) restarted with `launchctl kickstart -k`, once with a runner that exits
   at SIGTERM and once with one that ignores it until launchd kills `uv`, as in the probe of
   section 2.4; then `launchctl bootout` leaves the turn alive and `stop --turns` ends it; the
   test never touches a runner job that serves turns.
3. The same with SIGKILL of the runner. Then with SIGKILL of the keeper: the turn sees end of
   input, the lost tasks are counted, and a recovery run (or the note, without permission)
   follows. Then the CLI ends on its own, once in the runner that started it and once in an
   adopting runner: the keeper exits with the CLI's code, `<launch>.exit` carries the wait status,
   and in both runners `exit_label` and the classification match today's for the same fake run.
   The 18.6 stop of a turn signals the one group, and no process of it is left.
4. Handover in each phase of D-AR-14 (`preparing`, `waiting_lock`, `transcribing`,
   `compaction_retry`; `launching` below): after the restart the run continues in the new runner,
   and a wake run caught there keeps its chain. For `preparing` and `waiting_lock` the run stays
   `queued` (in `waiting_runs` while it waits), the new runner starts it from the `ACCEPTED`
   inbox entry, the server sees exactly one `run.start` for it, no launch check runs, and the
   phase record is removed; for `transcribing` and `compaction_retry` the run stays in
   `active_runs`; the compaction retry is posted and
   completes the run. Launch window (D-AR-3 launch handshake, phase `launching`): the runner is
   killed with SIGKILL at a test-only crash point after each step (record of `launching` written;
   keeper started, its identity not yet recorded; identity and `go_sent` recorded, go byte not
   written; go written, CLI identity not yet recorded, once with a keeper that writes
   `<launch>.cli` and releases the CLI only after the new runner's last look and before its
   SIGKILL of the keeper's group, which must lead to the adoption rule and no second launch of the
   attempt; CLI identity recorded, initial inputs not
   yet fed; inputs fed, `inputs_written` not yet recorded; the attempt's CLI ended, the next
   attempt's `launching` record not yet written), then a new runner starts. The same
   crash points run at the launch of the first attempt, of a compaction attempt and of the retry
   attempt after it (the fake CLI fills its context in the first attempt), and of a
   `ResumeSameReference` and a `NewSession` attempt. At the crash points after the CLI identity
   is recorded, the fake CLI of the first attempt fills its context either while no runner runs
   or after the adoption; both ends follow the adoption rule of D-AR-14, so the new runner picks
   the compaction and then the retry with the next attempt numbers. Asserted in each case: the
   fake CLI's start log shows at most one start per attempt, and exactly one per attempt after
   the run completes; the relaunched attempt has the recorded number, kind and command line, and
   no earlier attempt and no preparation is repeated; a CLI that started is adopted and never
   launched again; where no CLI started, no keeper and no unreleased child is left and the same
   attempt launches once in the new runner with the next `<try>`; the fake CLI's input log shows
   every initial input of every attempt exactly once, with its original `uuid`, and before any
   follow-up; no process of a keeper group is left without a record naming it; the run stays in
   `active_runs` throughout. Adoption rule variants: (a) an orderly handover (SIGTERM, exit 75)
   of a turn in phase `process`, after which the adopted fake CLI fills its context: the new
   runner runs the compaction and the retry, and the run completes; (b) the same when the
   record's recovery flags show the compaction already used in this run: the second
   `context_full` ends the run `runtime_error` without a new process, and lost work leads to
   D-AR-7; (c) a record without `process_start` is adopted as today and finished without
   recovery. Keeper variants: the keeper is killed after releasing the CLI and before
   `<launch>.fed`: `inputs_written` stays `false`, the runner writes no follow-up into that pipe
   and closes its write end, the CLI ends, and no input is written twice; the fake CLI stops
   reading in the middle of an initial input larger than the pipe buffer and then exits: the
   keeper is not blocked, writes no `<launch>.fed`, writes `<launch>.exit` and exits. Launch
   check variants (`run.launch.check`): before a relaunch after an outage the agent's tool class
   was lowered from `host`, or the person lost `trigger`: nothing launches, the run ends `failed`
   with the reason, `launch_check_refused` is logged; with the server unreachable nothing
   launches until the check answers; for a run the server already ended (`stale`) the refusal
   finishes nothing and the recovery run (or the note) follows through `run.wake.recover`.
5. A runner that comes back after `run.orphan_timeout`: adoption stops the live turn of the
   `stale` run, and a recovery run (or the note) follows through `run.wake.recover`.

Wake-ups:

6. `bothy-wake --after`, `--at`, `--pr`, `--until-success`: stored on the server, listed, fired
   once, the wake run resumes the same session (`--resume` with the same ref) and its first input
   names the label and the result.
7. A wake-up of each kind (`--after`, `--pr`, `--until-success`) created before a runner restart
   and before a runner crash fires after it; the runner learns every pending one, `--after` and
   `--at` included, from `run.wake.list_pending` at start and after a reconnect, so a `waiting`
   end after the restart gets no nudge while such a wake-up is pending, and a cancel during the
   outage reaches it through `run.wake_changed`.
8. Error paths, each asserting what is stored, logged and shown: server unreachable at create (the
   request stays in the SDK outbox and is sent later; the command says "accepted, not confirmed");
   create refused (not allowed, cap of pending wake-ups, `command` below `host`, `pr_checks` by a
   non-owner's run: nothing stored, exit code non-zero with the reason, `wake_refused` logged);
   condition check failing (`gh` missing or failing: retried with backoff, after
   `runner.wake_check_failures` failures the wake-up ends `failed` with the reason, a note in the
   thread, a notification); check before execution refused (between two executions of a pending
   `command`, the person loses `trigger`, the agent's tool class drops below `host`, or SR2 fails
   for the person; and a `pr_checks` whose person is no longer an owner): the command or `gh`
   read does not run again (the fake command records every execution), the wake-up ends
   `cancelled` with the reason, an audit entry and `wake_check_refused` exist, the note appears;
   a `pr_checks` created by an owner's `scoped` run passes `run.wake.check` and fires (no tool
   class `host` required);
   server unreachable before an execution: nothing runs and the check counts towards
   `runner.wake_check_failures`; fire refused (person lost `trigger`, channel archived, chain cap:
   `refused`, note, notification); deadline reached (`expired`, note); runner offline at fire time
   (the run waits `queued`, box "Agent offline"); cancel by the person (state `cancelled`, no run).
9. Wake input (D-AR-13): a wake-up firing into an active run of the same person is written as one
   input line with its token; each failed check (wrong run, thread, wake, person, billing mode or
   tool class, expired token) skips it with `wake_input_check_failed` and the wake-up returns to
   `pending`; an active run of another person keeps the wake-up `pending` until it ends.
10. With the key on, `CronCreate`, `CronDelete`, `CronList`, `ScheduleWakeup` and `RemoteTrigger`
    are in `--disallowedTools`; without it the command line is byte-equal to today's fixture.
11. Usage window, against a synthetic `rate_limit_event` fixture shaped like the frame of
    section 2.3 with `status` `rejected` (`overageStatus` stays `rejected`), followed by an error `result`: a
    running background task is not stopped and the run stays `running` while it runs;
    `run.wake.create` is called once with reason `usage_limit` and `not_before` after `resetsAt`;
    when the task ends the run ends `usage_limited` with `resetsAt`; without permission no wake-up
    is created and the note appears. Variants: the wake-up fires while the process is still held
    and arrives as one wake input; `overageStatus: allowed` with `status: rejected` changes nothing;
    an unknown `status` value is logged as `usage_status_unknown` and sets no mark (an error
    `result` after it is `runtime_error` as today); a later `allowed` frame clears the mark and
    cancels the outage's pending wake-up; after an error `result` under the mark, an `allowed`
    frame clears the mark while the task still runs: no stop follows, also not after
    `runner.stop_grace`, and the task keeps running; repeated `rejected` frames of one outage call
    `run.wake.create` once with the same outage number (a later `resetsAt` moves the pending
    wake-up), and a second outage after the mark cleared, after the first wake-up fired or was
    cancelled, gets a new wake-up of its own; an outage in a first attempt, then a compaction
    attempt with a second outage, gets a new wake-up with the next outage number; at `run.usage_wake_max` the next one is `refused`
    with the note; the mark found in the stream file after a restart leads to `run.wake.recover`; a `rejected` frame
    before the handover's checkpoint is restored by the replay of D-AR-2 in the adopter. An
    `allowed_warning` frame changes nothing but the log.

Guard and limits:

12. End state: a `result` with `STATE: waiting` and nothing waiting gets exactly one nudge line; a
    second such `result` closes stdin, the note appears, `continuation_without_wake` is logged; a
    `waiting` end of a notification turn of a background task started in answer to the nudge gets
    no second nudge (one per waiting phase), while a new follow-up or wake input starts a new
    phase. A handover between nudge and answer keeps the nudge mark (replay). A
    missing marker behaves as `waiting`. `STATE: done` and `STATE: blocked` close stdin as today
    and the summary carries the state. The marker line is never posted.
13. Chain cap: a held run plus repeated wake inputs and a sequence of wake runs both stop at the
    cap, counting inputs, not runs; a reply of the same person resets the count; a message of
    another person does not; `0` means no chain cap.
14. Idle cap and delay: three autonomous inputs without tool calls other than `bothy-wake` end the
    chain with a note also when the chain cap is `0`, both as wake runs and as wake inputs into
    one held run (the flag is reported per input); a held process whose notification turns only
    restart a background `sleep` stops at the idle cap with stdin closed and the note, also with
    chain cap `0`; `--at` in the past, `--after 0` and
    `--until-success true` fire no earlier than `run.wake_min_delay`.
15. `runner.cli_bg_wait_ceiling` reaches the turn's environment only when set.
16. Byte-equal fixtures of command line, environment, heartbeat and stdin handling with none of the
    new keys.

## 9. Rollout on a running installation

The order below is for an installation whose runner already serves live turns; a new installation
can switch the keys on from the start.

1. AR-02 (the stop fix) lands first. The restart that installs it still runs the old stop path,
   so the operator restarts only while no turn runs (ruling 4).
2. AR-04 server image: the heartbeat schema accepts the capabilities `turn_handover` and `wakes`,
   and `run.launch.check` and `run.wake.check` exist, with the new tables. This step comes before
   any runner switches `runner.turn_handover` or `runner.wakes` on, because the heartbeat's
   `details.capabilities` is closed (09 4.8): an older server refuses the heartbeat of a runner
   that reports `turn_handover`, and a runner that adopts a run cannot pass the launch check of
   D-AR-14 without `run.launch.check`. The operator checks the server's version before step 3.
3. AR-03 lands; its restart also happens only while no turn runs (ruling 4); then
   `runner.turn_handover = true` in the installation's `runner.toml` and a restart, under the same
   rule, because the runner that receives this SIGTERM still runs with the key off; from then on
   restarts need no wait. The operator's check: a harmless Bash background task in a thread
   survives a restart of the runner (under launchd `launchctl kickstart -k`), shown by its
   notification turn and the log line `turn_adopted`.
   AR-09 writes steps 1 to 3 into the service guide as the operator's checklist.
4. AR-05 and the key `runner.wakes = true`; owner threads may use wake-ups at once (ruling 2).
5. AR-06 recovery and usage-window wake-ups, AR-07 end state with `runner.end_state = true`.
6. An operator may raise `run.autonomous_chain_max` above its default for steering sessions that
   work through long days (ruling 1). Steps 4 to 6 are AR-10.
7. Rollback per step: turning a key off returns to the previous behaviour; pending wake-ups stay on
   the server and can be cancelled from the thread; a server rollback below AR-04 needs both runner
   keys off first (D-AR-11).

## 10. Ticket cut

Stage **AR Autonomous runs**. Every ticket is one pull request from `main`; AR-01 wrote the
normative text, the ADR, the rulings, the plan section and the briefs of the others, so the later
tickets run from briefs in `docs/briefs/` like every milestone.

The ticket table (AR-01 to AR-10, with what each one is done with), the dependencies and the
launch arguments live in [PLAN](../PLAN.md) section 22; the briefs are in the
[brief index](../briefs/README.md). AR-05 also depends on A1-05, because hosted agents get
`bothy-wake` after the intersection of 26 4.4. Waves: AR-02 and AR-04; then AR-03 and AR-08; then
AR-09; then AR-05; then AR-06 and AR-07; then AR-10. AR-02 may land at any time. The launch
arguments leave out AR-01 because it is merged before every other ticket starts.

## 11. Owner rulings

The owner delegated these decisions; they are ruled as follows and go to
`docs/decisions.d/` with AR-01. None is left open.

1. **Cap on autonomous inputs.** Repository default of `run.autonomous_chain_max` 25; an operator
   may raise it; `0` (no chain cap) is an allowed value. Reason: the default keeps a fresh
   installation cheap and visible; a higher value lets a steering session work through a long
   day and still stops visibly if it loops; `0` is allowed because the idle cap (cannot
   be switched off), the minimum delay and the pending cap still stop a loop that does nothing,
   so a thread with endless real work is not cut off.
2. **Who may run on alone.** Owners always; other persons only in channels with
   `allow_autonomous_runs`, which is off by default and changed with `manage`. This covers every
   autonomous input: wake runs, wake inputs, recovery and usage-window wake-ups. Reason: owners
   already carry the cost and SR2 decisions; for anyone else the opt-in must be explicit and
   visible per channel.
3. **GitHub access for `pr_checks`.** It uses the host user's existing `gh` login, read only
   (`gh pr checks` and the head commit), and only for runs of owners (D-AR-5). Reason: no new
   token, account, GitHub app or public webhook is needed; the owner restriction keeps the
   owner's login away from other persons' runs.
4. **Restart only while no turn runs, until the handover is on.** The operator rule for the
   restarts that install the handover tickets: the update to AR-02, the update to AR-03 and the
   restart that switches `runner.turn_handover` on happen only while no turn runs (section 9; 16
   states the rule). Reason: until the handover is on, a restart while turns run kills their
   CLIs' background work; after it, restarts need no wait at all, which is the point of the
   handover.
5. **Recovery runs.** Allowed within the autonomy rules of ruling 2. A recovery run lists the lost
   work and re-runs nothing; the agent decides what to do again. Reason: the owner wants work to
   go on after a loss, and R-AG-17 forbids only repeating side effects blindly, which this does
   not.

## 12. Backlog (minors and open points)

Open points of the first version:

1. Resolved by the probe of section 2.4: at `kickstart -k` the runner under `uv run` gets SIGTERM
   (uv forwards it), and after `ExitTimeOut` launchd sends SIGKILL to `uv` only. AR-09 keeps the
   probe as a test.
2. Whether a plain background shell can be held to the CLI's background ceiling instead of the
   short sweep is unverified; irrelevant after D-AR-3, noted for AR-01.
3. Whether the CLI's children inherit the lock descriptors is unverified; the keeper closes its own
   copies after the fork (D-AR-3) and must hold no lock descriptor, with a test in AR-03.
4. Durable CLI crons (`durable: true`, `.claude/scheduled_tasks.json` in the working folder) are not
   covered by `--disallowedTools CronCreate` once written by an earlier session; AR-05 checks
   whether such files exist in served folders and removes them from the runner's view or documents
   them.
5. `docs/design/README.md` is the overview of the design system, not an index of design notes;
   this note is listed in the `mkdocs.yml` navigation only, like [builtin-agents.md](builtin-agents.md).

Minors of the red team review of the first version, each for the ticket named:

6. Partial stdin line at handover (AR-03): a runner killed in the middle of a write leaves half a
   JSON line in the named pipe, and the next runner appends to it. On SIGTERM, finish or abandon
   in-flight writes as whole lines; a line larger than `PIPE_BUF` is not written atomically and
   needs a decision (split refused, or a length-prefixed handoff file).
7. Keeper error paths (AR-03): the leak when the CLI dies on its own and the blocking open without
   a reader are resolved in D-AR-3 (keeper exits after the CLI, `O_NONBLOCK` with `ENXIO`); the
   tests of item 3 and test 3 cover them.
8. No heartbeat during handover (AR-03): stated in D-AR-2 and asserted in test 2; a heartbeat
   without the runs would be persisted and end them `interrupted` (09 5.5).
9. Handover on every SIGTERM (AR-03, AR-09): resolved in D-AR-2. The runner exits 75 after a
   handover, so `KeepAlive {SuccessfulExit: false}` restarts it whenever the job stays loaded
   (`kickstart -k` restarts even after exit 0, probe of section 2.4); a `bootout` leaves the turns
   alive (probe), so the guides run `stop --turns` first, and a skipped one ends in adoption or,
   after `run.orphan_timeout`, in a stop with a recovery run.
10. The `command` kind (AR-01, AR-05) runs agent-supplied code outside any run: environment (the
    18.2 allow list), timeout per check, interval, concurrency, output size and the processes left
    at a restart are to be specified. Who may run it is not open: D-AR-5 checks `trigger`, tool
    class and SR2 before every execution.
11. `pr_checks` may fire too early (AR-05): right after a push no check may be registered yet; not
    reproduced. Record the head commit at create (D-AR-5 stores it) and fire only when checks exist
    for that commit and none is pending.
12. Unverified CLI behaviour stated as fact (AR-01, `real_cli` checks): that `--disallowedTools`
    removes the timer tools under `bypassPermissions` on `host`; that the CLI treats a named-pipe
    stdin like an anonymous pipe; that `Bash(bothy-wake:*)` cannot be bypassed by an environment
    prefix such as `TMPDIR=... bothy-wake`.
13. Request-file channel (AR-05): bind each request to its run by a per-run nonce or by the
    writer's process group, so one turn cannot file wake-ups for another run's folder (harmless
    under `host`, relevant under `scoped`).
14. References corrected in this revision: D-AR-1 cites 09 18.6 for the stop (not 09 1 item 4),
    the stop command is `claude-code-runner stop --turns`, and backlog item 1 states what the
    probe showed.
15. The answer to a nudge is posted as an extra reply in the thread (D-AR-8); whether clients group
    it with the run's answer is for AR-08.
16. `chat` sessions cannot request wake-ups (D-AR-6 states it); whether `chat` should get a
    restricted `bothy-wake --after` is a later product question.
17. A runner that hangs at SIGTERM outlives the job (AR-09): in the probe of section 2.4 launchd
    sent SIGKILL to `uv` only, and a runner child that ignored SIGTERM kept running next to the new
    runner. Today's runner handles SIGTERM, and with D-AR-2 it exits at once; whether the runner
    needs a single-instance lock in its state folder, or the service guide a group kill after a
    timeout, is for AR-09. This is not caused by this design.
18. The pinned CLI strings contain internal scenarios that set rate-limit headers (for example
    `weekly-limit-reached`); whether a `real_cli` check can use them to show the `rejected` frame
    and the `result` text without spending quota is unverified (AR-01).

Minors of the red team review of the third revision (the major and the other minors are resolved
in the text: replay of D-AR-2, idle count per input and notification turns in D-AR-9, one nudge
per waiting phase in D-AR-8, `list_pending` of every kind in D-AR-5, AR-09 and AR-10 cut by
rollout step, D-M2-35-3 amended only in its last clause, unknown usage status allowed and logged):

19. systemd (AR-01, AR-09): with the default `KillMode=control-group` the keeper does not survive a
    restart, since `start_new_session` leaves the process group but not the cgroup; limit the
    promise to launchd or document `KillMode=process` or an own scope in 16.
20. Inventory gaps (AR-01): the `--max-budget-usd` cap of hosted agents (D-A1-20) ends a run held
    for days visibly but is missing from section 3, as are the `--tools` intersection of D-A1-18
    (which may drop `Bash(bothy-wake:*)`), `disableWorkflows` and `list_pending` per hosted agent.
21. Usage-window deadline (AR-04): with `not_before = resetsAt + run.wake_min_delay` and a
    `seven_day` reset close to `run.wake_max_wait`, the wake-up could expire before it fires;
    count the deadline of a `usage_limit` wake-up from `not_before`.
22. AR-02 changes the stop without a key (by intent, 09 18.6): until the handover is on, a restart
    with live turns ends them within seconds instead of at the CLI's background ceiling; ruling 4
    covers it: those restarts happen only while no turn runs.

Minor of the red team review of the fourth revision (its major and the other minors are resolved
in the text: `ProcessState` with `last_result_at` and the failure mark in D-AR-2, `process_start`
in the ledger's record, test 2 variants a to c):

23. Idle count and waiting agents (AR-07): a notification turn that only starts a new background
    waiter (for example "wait for the rerun of the checks") counts as idle under D-AR-9, so an
    agent that waits for several CI runs in a row ends after `run.autonomous_idle_max`; AR-07
    checks whether reading a task's output is a tool call that already makes such a turn count
    as work (unverified) and otherwise decides whether a new waiter counts as work.

Minor of the red team review of the fifth revision carried over from the fourth (the majors and
the other minors of the fifth are resolved in the text: the adoption rule and the launch check
of D-AR-14, the named pipe per launch and the keeper's feed without blocking in D-AR-3, a
generic operator view in sections 2, 9 and 11):

24. Ends of a wake-up check (AR-04, AR-05): a refused `run.wake.check` ends the wake-up
    `cancelled` without a notification, while a refused fire ends it `refused` with one; decide
    one end state and notification for both, and add "channel archived" to the checks of
    `run.wake.check` (the fire checks of test 8 already name it).
