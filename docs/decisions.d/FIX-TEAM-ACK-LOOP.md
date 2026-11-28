# FIX-TEAM-ACK-LOOP Acknowledgements mention nobody, and the notice of a silent answer starts no run

Owner report of 2026-11-25 11:46 UTC in the team channel, with a screenshot: the conductor
handed work to the coding agent with `@`, which is right; the coding agent answered only with an
announcement that mentioned the conductor ("@agentconductor I am checking ... You will then
get ... back from me"); the conductor's run had nothing to add, the safeguard of
FIX-TEAM-HANDOFF-NOT-NOTHING turned that into "@codingagent: I got your request, but this run
ended without an answer to it ...", and the coding agent answered the notice ("Nothing has to
be done here by agentconductor ..."). Owner: "nobody has to be mentioned in this
acknowledgement, because that is what creates these loops every time". Quotes of the channel
are translated. Specification:
[09-agents.md](../spec/09-agents.md) 3.9.4, 3.9.6, 3.9.9, 17.14 and 18.5.

Read in the live channel (main thread of the team channel, read only with the ops client's
stored session), 2026-11-25, every notice of the day and what followed it:

| Time (UTC) | Message that mentioned | Notice | Answer to the notice |
|---|---|---|---|
| 09:08:58 | conductor to coding agent: "@codingagent: Thanks. I continued `run-5684-x` ..." (a thanks and a status) | 09:09:06 runner of the coding agent, "@agentconductor: I got your request ..." | 09:09:15 conductor: "That was no new request ..." |
| 09:40:16 | conductor to hermesmember: "@hermesmember: Thanks for the analysis ... I will tell you after the round" | 09:40:22 Hermes adapter, "@agentconductor: I got your request ..." | 09:40:32 conductor: "That was no new request to you ..." |
| 09:40:40 | reviewagent to hermesmember: the answer hermesmember had asked for (an answer hermesmember did not have to act on) | 09:40:47 Hermes adapter, "@reviewagent: I got your request ..." | 09:41:02 reviewagent: "That is the automatic notice again ..." |
| 11:36:41 | coding agent to conductor: "@agentconductor I am checking ..." (an announcement) | 11:36:47 runner of the conductor, "@codingagent: I got your request ..." | 11:36:55 coding agent: "Nothing has to be done here by agentconductor ..." |

Every mention without a request in it cost one run of the mentioned member, one notice that
started a run of the writer, and one visible explanation. The third row is an answer the requester
did not have to act on: the old rule sent it back with `@`, and it got the same notice.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-TAL-1 | 2026-11-25 | The quiet rules of the runner and the adapter say that `@handle` is written only when that member has to act on the reply or answer it: a hand-over, a result it waits for to go on (the conductor waiting for the coding agent's finding), or a question that needs its answer. An answer or information the member does not have to act on goes without `@`, also when it asked for it. An acknowledgement, a thanks, a notice that work starts or runs ("I am on it", "understood", "it runs again", "I report when it is done") and a status mention nobody, also when they answer a request, because every mention starts a run. The member who asked is mentioned when it has to act on the answer (it waits for the result, the member cannot do it, or the member needs its answer); work that ends later is announced without the `@`. This refines D-THN-1. | Rows 1, 2 and 4: the old rule "mention the member who asked when you give something back" read as covering the announcement of an answer. Row 3 (team follow-up of the same day, hermesmember saw it in the adapter): reviewagent answered hermesmember's question with `@`, hermesmember had nothing left to do, and the notice came anyway; "give back the result it asked you for" covered that answer, the test "has to act on it or answer" does not. |
| D-TAL-2 | 2026-11-25 | The notice of D-THN-2 names the member who asked without the `@`: `<source>: your message mentioned me, but this run ended without an answer to it, so if it asked for something, nothing of it is done yet. Send it again or ask the owner.` with the note that the runner or the adapter wrote it, ending as before with `the session replied only that it had nothing to add.)`. It stays visible to the owner in the team channel and hands nothing on. This replaces the mention of D-THN-2. | The code cannot tell a request from a thanks or a result (row 3 is a result), so "do not fire when the trigger was no request" would need a judgement the safeguard cannot make. Dropping the notice would make a lost hand-over silent again (the report of FIX-TEAM-HANDOFF-NOT-NOTHING). Without the `@` the owner still sees every miss, and no notice can start a run, so no chain can follow it. |
| D-TAL-3 | 2026-11-25 | A silent answer to a notice stays silent (D-THN-4) and keeps its check of the closing words, for a notice an older runner or adapter still posts with a mention. | The runner and the adapter are installed apart; the check costs nothing. |
| D-TAL-4 | 2026-11-25 | The server's safety brake stays unchanged. | Each chain was two or three inputs between the same pair (rows 1 and 2: A to B, B to A, A to B; the answer to the notice mentioned nobody), while `team.pingpong_max` is 40 by default (no override found in the production files; the API does not show the value) and `team.turn_budget` 200 (`getChannelTeam` of the channel: `turn_budget` 200, `member_turns` 1 after the owner's message), and every owner message resets both. The brake stops endless circles, as 3.9.6 says; a limit low enough to stop a chain of three would stop every request with its answer and the answer's follow-up. The chain is no fault of the brake but of the mentions, and D-TAL-1 and D-TAL-2 remove them. |
| D-TAL-5 | 2026-11-25 | The notice is not suppressed by a guess whether the incoming message held a request; it fires as before for every silent first answer to a message that mentions the member, and only its missing `@` keeps it from starting a run. | Team follow-up of 2026-11-25 asked to skip the notice when the message recognisably held no request, if that can be told reliably. It cannot: row 2 ("Thanks ... I will tell you after the round") and the announcement of row 4 ("Please do not continue anything until then") mix a thanks or a status with a wish, row 3 is a reply to a question, and a word list or a question mark would miss requests written as statements ("I need the paths") and silence the safeguard exactly where FIX-TEAM-HANDOFF-NOT-NOTHING needs it. A wrong skip loses a hand-over without a trace; a needless notice without `@` costs one visible line and no run. |

Not covered: the prompt is followed most of the time, not always. An acknowledgement that still
mentions a member starts one run of that member; when that run has nothing to add, the notice
shows in the team channel without a mention and the chain ends there. A member that answers the
notice (it reaches the member only as a context input in its next run) is not prevented.

Addendum of 2026-11-28 (open minor of the delta review of 2026-11-25, also found by the review
of pull request 1798): the quiet rules no longer say "unless it asked you for something" after
"never mention a member to say ... that you are waiting", so a plain waiting notice to the
requester goes without `@` as D-TAL-1 says; a member that has to act on a blocker is still
mentioned.
