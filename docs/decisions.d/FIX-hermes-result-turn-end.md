# FIX-hermes-result-turn-end The end of a turn Hermes starts itself, on every path

Follow-up of FIX-hermes-hidden-background (#562). A delta red team reproduced with the
Fake-Hermes rig against the pinned `v0.22.5`: when the turn Hermes starts with a delegation's
result ends with an error (the model refused the request with HTTP 400), the run stayed `running`
for good and a follow-up stayed `queued` until a person pressed stop. Cause, read in the pinned
source: the plugin hook `on_session_end` fires only at the end of `finalize_turn`
(`agent/turn_finalizer.py`); the error paths of `agent/conversation_loop.py` (a non-retryable
client error, exhausted retries, an interrupt) return without it, and an exception skips it. The
adapter ended such a turn only on `on_session_end`. Rule: 09 17.12 points 3 and 4, design note
[hermes-background-work.md](../design/hermes-background-work.md).

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-HRTE-1 | 2026-11-06 | A turn of the thread runs while the gateway's running-turn slot of the thread's session is taken (`GatewayRunner._is_session_running` with the key of `_session_key_for_source`); `Runtime.turn_running` reads it, `None` when the gateway or the method is missing. While the run holds work (or tracks a turn Hermes started) and no turn of the adapter runs, a taken slot is a turn Hermes started itself, and a free slot ends it. Place: `platform.py` (`turn_running`), `handler.py` (`_watch_session`). | The slot is claimed before the agent runs and released in the `finally` of `_handle_message` on every path, also for a turn of the session authority's queue. The platform's `_active_sessions` guard is not used: the queue's turns pass it by, and it outlives a turn's `on_processing_complete`. The same signal closes the window where a result turn's `pre_llm_call` came before the hold or during a turn of the adapter. |
| D-FIX-HRTE-2 | 2026-11-06 | A turn Hermes started itself failed when it ended without `on_session_end` after its `pre_llm_call`, or when `on_session_end` reports `failed` or `interrupted`. Then the run ends `failed` with `runtime_error` once `hermes_adapter.background_settle` passed after it; the open work is cancelled, and a pending follow-up is not started (the follow-on run takes it). | As a failed follow-up turn (D-FIX-HHB-4). Hermes sends the error text after the turn, so the settle time keeps it a message of the run; `completed` would hide the failure. |
| D-FIX-HRTE-3 | 2026-11-06 | `on_session_end` counts for every turn of the platform's thread, no longer only for the session ID the last `pre_llm_call` named (replaces that part of D-FIX-HHB-3). | A subagent's hooks carry the platform `subagent` and never reach the platform; the ID check only dropped the end of a turn that a compression moved to a new session ID, which D-FIX-HRTE-2 would count as failed. |
| D-FIX-HRTE-4 | 2026-11-06 | A turn of the adapter that returns while the slot is free clears the open `pre_llm_call` mark, so a turn of the adapter that ended without `on_session_end` is not counted as a failed turn Hermes started later. | The outcome of the adapter's own turn comes from `on_processing_complete`. |

Not changed: unreadable registries still count as no open work (logged), so the run ends after
the settle time (red team minor 3); a turn of the adapter that ends with Hermes' error text is
reported `SUCCESS` by the gateway and ends its run `completed` as before.
