# FIX-runner-responsive-turns A working session stays reachable

Owner request of 2026-11-01: a run showed "running 30:00" while Claude Code sat in blocking
foreground waits ("Wait for merge, push and control run to finish"), and the owner's message got
no answer. Normative text: 09 18.5 (addendum "Background work"), `prompts/runner_prompt.md`.

Cause, read in the code and in the stream of the run: the runner closed stdin after every
`result` without pending follow-ups (`followups.stdin_close_due`, called from
`RunTurn._idle`), and the pinned CLI stops its background work when its stdin ends. A turn that
moved work to the background therefore lost it as soon as it answered, and nothing in the runner
prompt said otherwise, so Claude Code kept long work in the foreground of one turn (foreground
commands up to their 600 s timeout, foreground subagents, until-loops and a Monitor that only
waited), and the thread saw no answer until the whole turn ended. Follow-ups wait in the CLI's
queue until the next tool boundary of the running turn.

Second cause, read in the live transcript of the affected session (entrypoint `sdk-cli`): in
personal mode the turn loads the user's own Claude Code settings, and a user-level Stop hook
there answered every attempt to end a turn while background tasks ran with `decision: block`
and the instruction to wait for them inside the turn with the Monitor tool or a Bash
until-loop. The transcript shows that block nine times, a compaction summary that keeps it as a
working rule ("wait inside the same turn with foreground Bash until-loops"), and then exactly
the foreground wait the owner saw. The hook skips any turn whose environment carries
`CLAUDE_PRODUCT_RUN`, but the runner's allow list (`environment.py`, `_KEPT` and
`build_environment`) dropped that variable even when the runner's own environment had it. A
prompt sentence alone cannot beat a hook that blocks the stop: the runner now sets the marker
itself.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-runner-responsive-turns-1 | 2026-11-01 | After a `result` without error, stdin stays open while the last `system/background_tasks_changed` lists a task; for a process that had background work also while a turn runs (`system/init` to `result`) and until `runner.followup_grace` passed without a stream line. Tasks marked `ambient` or `skip_transcript` do not count. Place: `background.py` (`BackgroundWork`), `runturn.py` (`_idle`). | The CLI stops background work when its input ends and starts the notification turn by itself shortly after the last task ended; the quiet window keeps that turn. A reference bridge that runs Claude Code the same way showed the order (task list empty, notification, `system/init`, `result` without input) and the stop of the work at the end of stdin with the pinned version 2.1.288. |
| D-FIX-runner-responsive-turns-2 | 2026-11-01 | The quiet window is `runner.followup_grace` (default 30 s); no new key. | It already is the window in which stdin stays open for follow-ups after a `result`; one value keeps the configuration small. |
| D-FIX-runner-responsive-turns-3 | 2026-11-01 | Follow-ups during background work take the existing path (written at once, `delivered`, `absorbed` at the echo); each becomes a turn of its own and its `result` is posted. The answer of a notification turn is posted like any other; an empty one is not. The run stays `running` until the process ends. | Same session, same process, no new server state: the run is truthfully working while its background work runs, and the stop button, costs (from the run's last `result`) and permission handling stay as they are. |
| D-FIX-runner-responsive-turns-4 | 2026-11-01 | While a task runs, `LiveBox.keepalive` patches `/metrics/progress_events` once per `runner.progress_patch_interval`. | Between turns the stream can be silent for a long time; without progress the server would show the stall notice of `run.stall_after`. |
| D-FIX-runner-responsive-turns-5 | 2026-11-01 | The runner prompt gets a section "Background work": a persistent process, long work in the background, end the turn at once with a short reply, never wait inside a turn, report a notification briefly, answer status questions from the running tasks; a rule or hook that keeps headless turns open because of background tasks does not apply. | The runner can only keep the process; Claude Code has to end its turns for the session to be reachable. The wording follows the reference bridge's guidance, in English like the rest of the prompt. |
| D-FIX-runner-responsive-turns-6 | 2026-11-01 | A `result` with an error still closes stdin at once (the recovery of 18.9 stays as it is), which ends running background work. | Keeping the process after an error would need a recovery that cannot restart the CLI; out of the scope of this fix. |
| D-FIX-runner-responsive-turns-7 | 2026-11-01 | `build_environment` sets `CLAUDE_PRODUCT_RUN=1` in every turn's environment, in both modes, independent of the runner's own environment. Place: `environment.py`; normative text 09 18.2 "Environment". | It is the marker by which user-level hooks recognise a product conversation and let its turn end; set by the runner, it needs no deploy step and survives the allow list. It is no secret and widens nothing a turn may do. |

Addendum of 2026-11-01 to D-FIX-runner-responsive-turns-6: live, the pinned CLI kept a process
with background work for about ten minutes after its stdin ended, so a failed turn held the run
and its follow-ups that long. docs/decisions.d/FIX-runner-auth-failed-held.md bounds that wait.

Limits (not built, recorded for a later ticket):

- The prompt asks Claude Code not to wait in the foreground, but nothing enforces it: a turn that
  still waits keeps a follow-up in the CLI's queue until the next tool boundary. The reference
  bridge adds a hook that refuses pure wait commands and an answer deadline that interrupts the
  turn; neither is part of this fix.
- A background service that never ends (a development server) keeps the run `running` and its
  slot of `runner.max_parallel_runs` until someone stops the run; there is no upper bound.
- A stop during background work relies on the CLI ending its own tasks on SIGINT or SIGTERM;
  background shells live in process groups of their own, so a CLI killed with SIGKILL can leave
  them behind. Not probed with the real CLI in this ticket.

Deploy note: nothing has to be set for the runner service; the runner sets the marker in each
turn. A session that already carries the "wait inside the turn" rule in its compaction
summaries can keep following it until a new session starts; ending such a session once (stop
the run, next message starts fresh) clears it.
