# FIX-TEAM-CHANNEL-NOISE Fewer runs and posts in team channels

Owner report of 2026-11-22 (translated): "somehow too many answer in the chat, even when they
were not addressed ... please look at everything and fix everything". Read in a live team channel
with four members (three Claude Code runner members, one Hermes member) on the server build of
that morning: an owner message that mentioned only one member started a run of every member, and
three of them answered that it was not for them; members mentioned each other only to say that a
message went to someone else, that they stayed out or that they waited, and every such mention
started a run of the mentioned member, which answered again (one pair alternated four times with
"nothing new, I wait"); and the Hermes member posted its memory plugin's recall line ("recalled 43
memories") as a message of its own in the team channel, once per run. Specification:
[09-agents.md](../spec/09-agents.md) 3.9.3, 3.9.4, 3.9.9, 17.14 and 18.5.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-TCN-1 | 2026-11-22 | An owner message that mentions the handle of one or more members (the mention rule of 3.9.4) goes only to those members; one that mentions no member goes to every member as before. A mention of a handle that is no member's does not narrow. The brake still resets on every owner message. Every member it does not address gets it as a context input of FEAT-TEAM-CONTEXT-ALL (no run, no follow-up), so every member still sees the whole conversation. This replaces owner answer 2 of 2026-11-21 (D-TEAM-1, "mentions never narrow who gets it"). | Owner word of 2026-11-22 (above); the owner's status call without a mention must still reach everyone. The mentions are read from the message itself before any copy (`messaging.TriggerMessage.ContentMD`), so the job never copies to a member it then has to take back. |
| D-TCN-2 | 2026-11-22 | A member's reply whose whole text is the silent reply `NOTHING_TO_ADD` (any case, white space, `*`, `_`, backticks and a closing full stop around it; `domain.IsSilentTeamReply`) is neither mirrored nor handed on; nothing is noted or audited, also for a run a non-owner fed (nothing would be copied). The check is in the server's team job, the one place every runner's and adapter's replies pass. | One mechanism for every runtime instead of one per runner; the reply stays in the member's work thread, where the owner can still read it. The marker is not one of Hermes' own silence markers (`NO_REPLY`, `[SILENT]`), because Hermes replaces a bare silence marker by a warning text where its lane expects a reply, which would then be mirrored. |
| D-TCN-3 | 2026-11-22 | The team part of the Claude Code runner and the team lines of the Hermes adapter carry the same quiet rules: a mention only for a hand-over with an expectation, a member that is only referred to named without the `@`, never a mention to say that a message was not for it, that the writer stays out, agrees or waits, and `NOTHING_TO_ADD` as the whole reply when the run has nothing new to contribute. The line for an unaddressed owner message says that an owner message without a mention reaches every member and offers the silent reply. | The chains in the report came from mentions that handed nothing over; the server cannot tell a hand-over from a reference, so the model is told, and the silent reply ends a chain at the first member with nothing to add. |
| D-TCN-4 | 2026-11-22 | The brake defaults (`team.turn_budget` 200, `team.pingpong_max` 40) stay. | Owner answer 3 of 2026-11-21 wants them high, only to catch loops; D-TCN-1 to 3 remove the causes of the runs instead of cutting legitimate back and forth. |
| D-TCN-5 | 2026-11-22 | The Hermes adapter implements `send_or_update_status`, the method Hermes' status callback uses when a platform has it (`gateway.run._send_or_update_status_coro`): a status line becomes the draft of the thread's active run (as `_interim_send` progress, D-ADAPTER-INTERIM-1) and is dropped outside a run; it is never an answer and never a report, in every channel. | Without the method Hermes fell back to `send`, so every recall line became a message of the run (also in ordinary channels, 28 such messages on the day of the report), and in a team channel a mirror. A status line is progress, not an answer. Read in the pinned Hermes, not changed there. |

Not covered: the model decides whether it has something to contribute and whether a mention is a
hand-over; a member that ignores the quiet rules still starts runs, which the brake catches. A
member's own follow-up reply (a second answer of a run that took a follow-up) is mirrored like any
reply unless it is the silent reply.
