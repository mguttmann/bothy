# FIX-runner-silent-stop Make a silent stop of a run visible (issue 553)

Owner report (issue 553): a runner turn can end with a promise to continue later ("once CI is
green I merge") while no background task runs. The process ends `runner.followup_grace` after
the answer and nothing wakes the session again, so the thread stops without a sign. A restart of
the runner ends the background work of running sessions and they never report back. The
`SessionEnd` hook of the host's own ledger is reported cancelled at process ends of runner
sessions.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-runner-silent-stop-1 | 2026-11-08 | A run its turn completes is finished with the `summary` of the code `run_ended_idle` ("Run ended; nothing is waiting, so this thread continues only when someone sends a message"), for every completed run, with or without earlier background work. | The run completes only after its process ended, and the process ends only when no background task runs (stdin closes only then, 18.5), so at that moment nothing waits on the thread in either case. The collapsed run line shows a completed run's summary, so the note needs no client change and posts no message of its own. Telling a promise apart from a final answer needs the end state marker of AR-07 (`runner.end_state`), which replaces this generic note with the nudge and `continuation_without_wake` once built. |
| D-FIX-runner-silent-stop-2 | 2026-11-08 | An orderly stop of the runner (`adapter_stopped`) names in its summary how many background tasks the process held when the stop came ("N background tasks were still running and were stopped; their results are lost"). The count is read before the first signal, on the request of the stop (`request_stop`) and on the SDK's cancellation (`_stop_on_cancel`). | The issue offers two ways: wait with the update while a run holds stdin for background work, or tell the thread which runs lost their work. Waiting has no bound (a background task may run for hours) and would block every update; `runner.turn_handover` (AR-03) already keeps the turns and their work across a restart. Without the key the stop is unchanged and the thread now learns what was lost. The count is read before the signal because the CLI reports its task list empty as the stop ends the tasks. |
| D-FIX-runner-silent-stop-3 | 2026-11-08 | Nothing is built for the cancelled `SessionEnd` hook. | Checked with the pinned CLI 2.1.288 against a local stand-in for the API (no login, no model call): after the end of stdin the CLI waits for a `SessionEnd` hook up to the hook's own `timeout` (a hook of 3 s and one of 12 s with `timeout` 30 both finished before the CLI exited). The stored stderr files of the affected runner show "Hook cancelled" in 13 of 309 process ends, all in two long-lived sessions with very large transcripts; the other sessions of the same runner and hook show no such line. The cancellation is the hook exceeding its own `timeout`, a matter of the host's hook, not of the runner's stdin or stop handling. |

Deploy note: nothing to configure.
