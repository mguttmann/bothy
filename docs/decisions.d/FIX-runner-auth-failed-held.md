# FIX-runner-auth-failed-held A failed turn never holds the run open for background work

Found by the red team of FIX-runner-reachable-2 and seen live on 2026-11-01. Normative text:
09 18.5 (addendum "Background work") and 18.9.

Evidence, read in the runner log, the session transcript and the server's rows (read only):

- At 21:36:14 (local time) a turn of a long thread ended with a short answer and left
  background work running (a workflow and subagents); the run stayed `running` for it
  (FIX-runner-responsive-turns).
- At 21:36:48 a subagent's notification started a turn by itself; the login of Claude Code on the
  host was briefly gone, and the turn ended with the API error "Not logged in" (`error:
  authentication_failed`, an `assistant` message with the API error flag) and an error `result`.
- The runner closed stdin at that `result` (`RunTurn._read`: a failed turn absorbs nothing more)
  and set the phase `closed` (`RunTurn._close_stdin`). The pinned CLI did not exit: it kept the
  process for its background work for about ten minutes (task notifications kept arriving in its
  queue until 21:46:49).
- At 21:37:54 the owner asked a question. The runner logged `followup_not_deliverable`
  (`RunTurn.offer_followup` refuses every follow-up in phase `closed`); the follow-up stayed
  `queued`, and nothing answered.
- At 21:46:49 the process ended and the run finished `failed` with `runtime_not_authenticated`
  and no summary; the server deferred the question into a new run (09 section 7), which resumed
  the same session and answered at 21:47:27, more than nine minutes late. The thread never said
  that the login had failed, and the background work of the old process was gone without notice.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-runner-auth-failed-held-1 | 2026-11-01 | A process that is still alive `runner.stop_grace` after a `result` with an error is stopped like a stop of the run (SIGINT, then the escalation of 18.6; log code `turn_stopped_after_error`); its end is classified as before (the stop does not make it `stopped`), so a lost login still ends `failed` with `runtime_not_authenticated` and a full context still compacts. Place: `runturn.py` (`RunTurn._read`, `RunTurn._idle`). | The run is held by the process, not by the turn: as long as the CLI keeps the process for background work, the run stays `running` and refuses every follow-up. A bound on that wait is what gives the owner an answer or an error in time; the stop grace is the time a process already gets to end by itself. |
| D-FIX-runner-auth-failed-held-2 | 2026-11-01 | No retry of the failed turn inside the run. The run ends, and the follow-ups it did not take stay `queued`; the server defers the first into a new run of the thread, which resumes the same session (09 section 7, 9.1). | The failed turn was the CLI's own notification turn: there is no input to send again, and a later turn of the same process cannot be forced. A new run gets the full pre-check (login, billing mode) and a fresh process, and it is what the server already does with deferred follow-ups. |
| D-FIX-runner-auth-failed-held-3 | 2026-11-01 | A run that a turn process ended with `runtime_not_authenticated` carries a `summary`: Claude Code lost its login on the runner host during the run, and once the login works again the next message starts a new run that continues the same session. A `failed` end after which background tasks were still running names them as lost (count only), also for `runtime_error`. Place: `causes.py` (`turn_end_cause`). | The app shows the summary under the reason in the thread (R-AG-35), so the owner learns what happened and what to do without the runner posting a message of its own. A task that ends with the process reports nothing, so saying it is lost is the only honest report. |
| D-FIX-runner-auth-failed-held-4 | 2026-11-01 | Task steps of the live box that are still open when such a process ended become `cancelled`. Place: `livebox.py` (`LiveBox.end_open_tasks`). | A failed run must not show background work as running. |

Limits (not built):

- A turn that fails while the login is gone is not repeated when the login comes back; the owner
  sends the next message (the summary says so).
- An error `result` of an adopted turn (after a runner restart) does not start the bound; the
  adopted process keeps the run until it ends, as before.
- The count of lost tasks comes from the last `system/background_tasks_changed` of the stream;
  a task the CLI did not list there is not counted.
