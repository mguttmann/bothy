# FIX-hermes-hidden-background A Hermes run holds the background work its turns started

Owner report of 2026-11-06: a Hermes agent said a team was still working, but the app showed no
run and no stop button. Read in the live logs and the server's run table: the Hermes adapter
covered exactly one Hermes turn per run. In the pinned `v0.22.5` every top-level `delegate_task`
runs in the background, so the turn that started a team ended at once and the adapter finished
the run. The subagents then worked for about half an hour without a run. When a delegation
finished, the gateway started a full turn with tools in the thread's session, past the platform's
`handle_message`; such a turn started the next delegation and changed a scheduled job, and its
answers became reports without a run. Reproduced with the Fake-Hermes rig against the pinned
version: the result turn reached the adapter only through `pre_llm_call`, called a tool and
posted its answer without a run. Rule and design: 09 17.12, R-AG-44,
[design note](../design/hermes-background-work.md).

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-HHB-1 | 2026-11-06 | The background work of a run is the set of delegation units (`tools.async_delegation.list_async_delegations`) and background processes (`tools.process_registry.list_sessions`) of the thread's Hermes session that started after the run started. A unit is open while it is live or while its durable `delivery_state` is `pending`; a process while it runs. The adapter reads both every `hermes_adapter.background_poll_interval`. Place: `platform.py` (`background_work`, `_read_background`), `runtime.py` (`BackgroundTask`). | Hermes reports no end of a unit to a platform; its registries are the only source. Work of earlier runs is not counted, so a process left from an earlier run never holds every later run. |
| D-FIX-HHB-2 | 2026-11-06 | After its turn completed the run stays `running` while a unit is open: a step `subagent` per delegation unit and `background_job` per process (title one line, at most `thread.title_max_chars`), the open counts in `/metrics/subagents` and `/metrics/background_jobs`, and `/metrics/progress_events` once per `hermes_adapter.background_progress_interval`. Place: `handler.py` (`_hold`, `_poll_work`, `_show_task`). | Mirrors 18.5; 5.4 shows these steps and counts already; the progress keeps the stall notice of 5.5 away. |
| D-FIX-HHB-3 | 2026-11-06 | A turn Hermes starts in the thread while the run holds work is a turn of the run, from `pre_llm_call` to `on_session_end` (both hooks are now registered; `on_session_end` counts only for the thread's own session, not a subagent's; the session check and the end on `on_session_end` alone are replaced by D-FIX-HRTE-1 to D-FIX-HRTE-3 of [FIX-hermes-result-turn-end](FIX-hermes-result-turn-end.md)). The run ends when nothing is open, no turn runs, and, after the run saw work, `hermes_adapter.background_settle` passed without a turn's start, a status sentence or an answer. | The gateway admits the result turn a moment after the unit ended and gives the platform no other signal. A run without background work ends at once as before. |
| D-FIX-HHB-4 | 2026-11-06 | A follow-up written while the run holds work, and one the first turn did not take (after its answer, or at `queue`) once the run holds work, is a turn of its own in the run, one after the other (`TurnRequest.context_taken`: the user side carries the rules and channel instructions only); `delivered` at the start, `absorbed` when it completed; a failed one ends the run `failed`. A follow-up with a file stays `queued` (D-FIX-HIMG-3). | Without it the person's next message would wait for the background work. |
| D-FIX-HHB-5 | 2026-11-06 | Stop cancels the work: `interrupt_for_session(session_key)` for the delegations and `kill_process` with consumed output for the running processes. Any other end before the work is done (a failed turn, an abort) does the same and ends the open steps `cancelled` (after a stop the server takes no patch any more). | `/stop` through `handle_message` stops only a running agent while one runs; the delegations of an ended turn kept working. |
| D-FIX-HHB-6 | 2026-11-06 | The plugin hook `pre_tool_call` refuses every tool call of a session of the platform (`HERMES_SESSION_PLATFORM`) whose thread has no open run of the adapter, and of one whose thread cannot be told, with `handler.TOOL_REFUSED`; the hook has no `platform` argument, so the entry passes it on without that filter. | The pinned version gives a platform no veto on an injected turn; refusing every tool covers the result turn after a stop or restart, a subagent of an ended run and Hermes' review fork. Fail closed. |
| D-FIX-HHB-7 | 2026-11-06 | The text answer of a turn without a run stays a report without a run (17.3). | The model call itself cannot be refused (D-FIX-HHB-6); posting the text keeps nothing hidden, and the turn could not act. |
| D-FIX-HHB-8 | 2026-11-06 | No run of its own for a result turn. | The server opens runs only for a message, a command or an interaction (03 `runs.trigger_kind`); a new trigger kind is a server change outside this ticket, and D-FIX-HHB-3 covers the result turn while the run is open. |
| D-FIX-HHB-9 | 2026-11-06 | The Fake-Hermes kit routes script turns by the tools a request offers (`BoundTurn`, `requires_tool` and `without_tool`), so a delegation's subagent and its parent, which ask the model at the same time, get their own turns. The routing lives in the kit, not in the recording harness, so the recorded fixtures keep their inputs. | Deterministic scenarios against the real gateway without a new recording. |

Limits (recorded, not built): a process that never ends keeps its run `running` until a stop; a
restart of the gateway or the adapter ends the hold, and a result Hermes replays later runs
without a run (tools refused, text as a report); a turn whose `on_session_end` never comes keeps
the run open until a stop (closed by FIX-hermes-result-turn-end); a follow-up with a file waits for the end of the hold.

Deploy note: nothing to configure; the three new keys have defaults (09 21). Hermes itself is not
changed.
