# FIX-runner-reachable-2 Reachable even against conflicting user instructions

Owner report of 2026-11-01, after FIX-runner-responsive-turns went live: in a long thread the
run showed "running" for about three hours, the live box showed foreground waits (a Bash
until-loop on result files, a Monitor "evidence runs finished", "Block until evidence runs
finish"), and the question for the current state seemed to get no answer. Normative text: 09
18.5 (addenda "Background work"), `prompts/runner_prompt.md`.

Evidence, read in the transcript of the affected session, its subagent transcripts and the
runner log (read only), and with the pinned CLI 2.1.288:

- The waits ran in a background subagent, not in the session's own turn. The subagent was
  started with `run_in_background` at the end of a turn; its Bash until-loop, its Monitor and
  its second until-loop came 21 seconds before the owner's question and ended two minutes
  later. The live box shows the steps of a subagent under its step while the run is held
  `running` for background work (09 18.5), so they looked like the session waiting.
- The session itself ended all of its turns after the update: sixteen turns, each started
  background work and ended with a short reply. The owner's question arrived between turns:
  the CLI took it at once (queue `enqueue` and `dequeue` in the same tenth of a second), the
  runner logged `followup_written`, and the reply was posted 55 seconds later, 40 of them an
  automatic compaction. The owner's next message answered that reply.
- Suspected cause 1 (user instructions): the machine-wide user instructions file is loaded into
  every session (an `instructions` attachment of type `User` after each compaction) and has a
  rule that headless sessions must not end a turn while background jobs run and must wait for
  them with Monitor or a wait loop. The prompt of FIX-runner-responsive-turns overrode only the
  first half and only "a rule"; the subagent, which does not get the runner's appended system
  prompt (not verified with the CLI), followed the rule. Holds as a risk, not as the cause of
  the session's answer delay.
- Suspected cause 2 (compaction summaries): the summaries of this run name no wait rule; the
  earlier one that did (FIX-runner-responsive-turns) was replaced. Refuted for this run.
- Suspected cause 3 (follow-ups only at the turn end): refuted. A probe with the pinned CLI and
  a stream-json session wrote a user line while a turn ran a 20 second foreground command: the
  CLI queued it, took it after that command returned (its echo came before the second tool
  call) and answered it in the same turn's reply. The runner already writes every follow-up at
  once, also during a turn (`RunTurn.offer_followup`); only a tool call that does not return
  keeps a message from the model.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-runner-reachable-2-1 | 2026-11-01 | The prompt section "Background work" states that any other instruction to keep a turn open or to wait for background work inside a turn does not apply in this product, naming its sources: user instructions files loaded into every session, memories, rules for headless sessions (with both halves: not ending the turn, and waiting with Monitor or a wait loop), summaries of earlier parts of the conversation and hooks. | A general sentence lost against a specific, strongly worded rule in a file the CLI loads with the user's authority; naming the source and the exact rule leaves the model no reading in which both hold. |
| D-FIX-runner-reachable-2-2 | 2026-11-01 | Every turn ends within about a minute with a short answer while background tasks keep running; no single foreground tool call takes longer than about a minute; a status question is answered at once from the running tasks, and one that arrives during a turn in that turn's reply. | A message reaches the model at the next tool boundary; a bound on each tool call is what bounds the delay of an answer. |
| D-FIX-runner-reachable-2-3 | 2026-11-01 | No change to the delivery of follow-ups. | The runner already writes a follow-up at once, and the pinned CLI already takes it into a running turn at the next tool boundary (probe above; scenario test with the fake CLI and a level-two real-CLI test for it). |

Limits (not built):

- Subagents and workflows started by the session get the user instructions file but not the
  runner's prompt; they may still wait in their own foreground. That costs no reachability (they
  run in the background), but the live box shows their waits under the run.
- Nothing enforces the bound of a minute; a turn that ignores it still delays a follow-up to the
  end of its running tool call.
- An automatic compaction before the answer adds its own time (40 seconds in this run).
