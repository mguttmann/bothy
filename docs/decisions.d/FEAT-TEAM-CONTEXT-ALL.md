# FEAT-TEAM-CONTEXT-ALL Every team member sees the whole conversation

Owner request of 2026-11-22: every agent of a team channel sees everything, so the owner can tell
one member what he agreed with another and ask for its view or a handover. Specification:
[09-agents.md](../spec/09-agents.md) 3.9.4 addendum; design
[cross-project-handoff.md](../design/cross-project-handoff.md) 3.6.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-TCX-1 | 2026-11-22 | A mirrored reply reaches every member other than the source that it does not mention as a context input: the same copy (`CopyMessage`, origin `agent`, the source member's agent as author) and the same `team_inputs` row (source `member`, the sponsor as authority) as a member input, without a follow-up and without a run. No column marks it: no run and no follow-up names it, which is the whole difference. | The run context, the session and the record of `run_feeders` already treat a team input correctly: the context records its authority, so a later reply of the member still passes the owner check (09 3.9.4). Any other storage (an agent's message without a team input row) would record a NULL feeder and silence the member's replies while the copy lies in its context window. No migration. |
| D-TCX-2 | 2026-11-22 | It enters a run only through the run context (09 4.6): a later run reads it in `thread_context` as role `other`, trust `unverified`. The runner and the Hermes adapter are unchanged. | Both render unverified context only as marked data; owner codes go only to a trigger or follow-up of origin `human` by a person (`ownercode.is_owner_trigger`, `is_owner_followup`), so a context input never becomes the owner's word. The label is the author's agent handle, not the team handle (as the adapter's open minor 2). |
| D-TCX-3 | 2026-11-22 | A context input is not counted by the brake and is delivered also while the channel is paused or stopped; a mentioned member whose input the brake refused gets the reply as a context input. | It starts nothing, so it cannot loop; the owner wants every member to see every reply. |
| D-TCX-4 | 2026-11-22 | Points 1 to 3 of 09 3.9.5 apply (sponsor an active owner, readers rule, `agent.invoke` for the sponsor); only an active run of another agent in the work thread does not refuse it. A refusal is audited (`agent.invoke`, stage `team_context`, decision `deny`) without a note; an allowed one with decision `allow`. | The readers rule keeps text away from non-owners exactly as for a trigger; a note per member and reply would flood the team channel. Refusing instead of failing keeps one member's archived home channel from rolling back the mirror and the inputs of the others. |
| D-TCX-5 | 2026-11-22 | Only a reply that passed the provenance check is copied: a refused reply is neither mirrored nor given as context. | The context input carries the sponsor's authority into a later run's record; it may do so only for a text written in a run only active owners fed. |
| D-TCX-6 | 2026-11-22 | A member added later gets no backfill: it sees the replies mirrored after it was added. | The simplest rule; the owner can repeat what matters in the team channel, which reaches every member. |

Not covered: the window of a run context holds at most `history_backfill_limit` messages since the
session's last trigger; with many replies between two runs the oldest copies fall out (the
adapter may load more with `thread.read`). When the sponsor stops being an active owner, every
copy recorded with that sponsor refuses later replies while it lies in the context window, as the
member inputs already do.
