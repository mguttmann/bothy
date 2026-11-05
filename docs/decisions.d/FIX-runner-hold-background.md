# FIX-runner-hold-background Background work survives a resume after lost work

Owner report of 2026-11-05: in one thread the Claude Code session started its workflows and
background jobs again and again, and after every turn they were gone. The runner logged
`stdin_held_for_background` for other threads that day, but not for any recent turn of this one.

Cause, read in the stream files and the CLI's stderr of the affected session: a run of the
session had ended while background work ran (a runner restart interrupted it), so the CLI
reported the stopped tasks when the next turn resumed the session. Before it started the written
input it ran one turn by itself per stopped task: `command_lifecycle` `queued` for the input,
then per task a `system/init` and an empty `result` (`num_turns` 0), and only then `started` and
the input's own turn. The runner took the first of those results for the end of the turn
(`RunTurn._idle`, `followups.stdin_close_due` with no pending follow-up) and closed stdin. The
process had no background work yet, so the hold of FIX-runner-responsive-turns did not apply.
The input's turn then started workflows and background jobs; with stdin already ended the CLI
waited `CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS` (600 s) and terminated them ("Background tasks
still running after 600s; terminating" in stderr), which again left stopped tasks for the next
resume, so every later turn of the thread repeated it. The stream reader also skipped
`command_lifecycle` as an unknown type, so the runner could not see that the input was still
queued.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-runner-hold-background-1 | 2026-11-05 | The stream reader knows `command_lifecycle`; `BackgroundWork` keeps the `command_uuid`s whose last state is `queued`, and while one is queued no `result` closes stdin (`input_waiting`, checked in `RunTurn._idle` before the stdin rule of note 5). Any other state (`started`, `completed`, `cancelled`) ends the wait. | The early results answer no input; the input's turn is still to come and may start background work, which the existing hold then covers. In every stream file the affected runner had stored on 2026-11-05, every queued command reached `started` and a terminal state, so the wait cannot keep stdin open for good. |
| D-FIX-runner-hold-background-2 | 2026-11-05 | A `result` with an error still closes stdin at once, also while an input is queued. | Unchanged recovery of 18.9 (D-FIX-runner-responsive-turns-6). |
| D-FIX-runner-hold-background-3 | 2026-11-05 | No new log code: the turn of the input then logs `stdin_held_for_background` as any other turn with background work. | The fix restores the existing path; the stream files show the early results. |

Deploy note: nothing to configure. A session already caught in the loop leaves it with the first
turn on the fixed runner: its early results no longer close stdin, so the work of that turn is
kept and the next resume has no stopped tasks to report.
