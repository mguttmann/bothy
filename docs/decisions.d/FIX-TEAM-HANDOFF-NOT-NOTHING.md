# FIX-TEAM-HANDOFF-NOT-NOTHING A request that mentions a member is never answered with silence

Owner report of 2026-11-22: a hand-over in a team channel was lost. Read in the live system: one
member answered another member's question with a request that mentioned it ("@helper: ...
what I need back from you: the two fixed paths and the commit ..."). The server started a `team`
run of the mentioned member, which completed. The first input line of the Claude Code session
held the team part with "It mentions you (@helper)." and the quiet rules of
FIX-TEAM-CHANNEL-NOISE (D-TCN-3), which end with "If you have nothing new to contribute (..., or
you would only acknowledge it, repeat a status or say that you wait), your whole final reply is
exactly NOTHING_TO_ADD". Four seconds later the session answered exactly `NOTHING_TO_ADD`. In its
previous answer the member had said that it waits for the owner's go before it starts, so "say
that you wait" matched. The server dropped the reply as the silent reply (D-TCN-2): it was
neither mirrored nor handed back, and the requesting member never learned that the paths would
not come. The Hermes adapter had the same rule text. Specification:
[09-agents.md](../spec/09-agents.md) 3.9.4, 3.9.9, 17.14 and 18.5.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-THN-1 | 2026-11-22 | The quiet rules of the Claude Code runner and of the Hermes adapter say that an input that mentions the member with `@` and asks it for something is never answered with `NOTHING_TO_ADD`: the member does what it asks, or says briefly what it needs for it or what blocks it (the owner's go is the example), and mentions the member who asked when it gives something back. A mention to answer such a request is allowed, also when the answer is that the member waits. The silent reply stays for an owner message without a mention, a context input and an input that names the member without the `@`. The line that says an input mentions the member says the same. This refines D-TCN-3. | The rule "say that you wait" covered the case of the report; the requester has to learn what blocks its request, or it waits for an answer that never comes. |
| D-THN-2 | 2026-11-22 | A code safeguard backs the prompt, in the runner and in the adapter, not in the server: when the run's first answer is only the silent reply (the server's rule `IsSilentTeamReply`) and the run's trigger mentions the member, the answer goes out as a short fixed text instead: for a member's input `@<source>: I got your request, but this run ended without an answer to it, so nothing of it is done yet. Send it again or ask the owner.`, for an owner's message the same without a mention, each with a note that the runner or adapter wrote it. The runner reads the mention from the trigger's text (as its team part does), the adapter from `team.addressed`. | A prompt is followed most of the time, not always, and a lost hand-over is silent: nobody sees it. The fixed text makes the miss visible to the owner in the team channel and hands it back to the requester, which can ask again. Not in the server: it would have to write text in the member's name into the member's work thread, while the runner and the adapter know the run's trigger and write the answer anyway, so the change stays in the two places that write the rule text. A second model turn to repair the answer was rejected: it costs a turn for every miss and needs a new path into a finished turn. |
| D-THN-3 | 2026-11-22 | The safeguard covers the run's first answer only. A later answer of the run (to a follow-up) is covered by the prompt alone; the runner's lead-in of a follow-up that mentions the member carries the rule. | The first answer is the answer to the trigger, which the server's `team.addressed` and the team part describe; the answer to a follow-up cannot be tied to one input without a new record of which input a result answers. |
| D-THN-4 | 2026-11-22 | A silent reply to an input that is itself such a notice (it carries the notice's closing words `the session replied only that it had nothing to add.)`, the same in the runner and the adapter) stays silent. In the adapter the posted answer keeps Hermes' own text as the text to compare edits with, so a later edit of Hermes to the same silent text is never sent and never turns the notice back into silence. | Red team of 2026-11-22: the notice mentions the requester and starts its run; a silent answer there would send a notice back, and two members would ping-pong notices without any model asking for anything until the brake pauses the channel. Hermes' edit transport edits a posted answer again with the same text, which differed from the notice and became an edit to `NOTHING_TO_ADD` (an edited answer is not handed on). |

Not covered: the code cannot tell a request from a mention that asks for nothing (`@web thanks`);
such a run that answers only the silent reply now posts the fixed text. A requester that asks
again after the fixed text and gets it again starts runs until the brake (`team.pingpong_max`)
stops the pair. An edit of Hermes to a different text is passed on as before; an edit of the first answer
to the silent reply after another text was posted is not checked. A runner that replays an
unsent first answer after a restart without its run context sends it unchanged.
