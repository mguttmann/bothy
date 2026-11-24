# FIX-RUNNER-CLI-331 Background mode: a final answer the CLI writes late

The daily switch of a runner to Claude Code 2.1.331 on 2026-11-24 was rolled back: a follow-up
written into a running background turn ended the run `failed` with `runtime_error`, while a
plain run and a run with an attachment passed. The same failure had already hit a production
run on 2.1.330 on 2026-11-23; the 2.1.330 probe of 2026-11-22 passed by timing.

Evidence, read in the transcripts of these runs:

- The passing 2.1.330 probe: the turn's final `assistant` record, then the follow-up's
  `queue-operation` `enqueue` and `dequeue`, then `stop_hook_summary` and `turn_duration`.
- The failed 2.1.331 probe: `enqueue` and `dequeue` first, the final `assistant` record after
  them, right before `stop_hook_summary`; that record carries the time the answer came, before
  the `dequeue`'s. The failed 2.1.330 production run shows the same order (three `dequeue`
  records before two answer records seconds older). The CLI may write the record of a turn's
  final answer seconds after the answer came; queue records are written at once.
- The mapper saw the `dequeue` while the last request had no output yet and gave the start at
  once (the rule of FIX-RUNNER-FOLLOWUP-AT-TURN-END defers only after a final answer). The
  `result` of the turn before then counted as the follow-up's answer, the stdin and stop rule
  released (stopped) the session while the model worked on the follow-up, and the run failed.

Rulings:

1. A `dequeue` while the open turn's last request has no output yet is deferred like one after
   the final answer. The time of the first deferred `dequeue` is kept.
2. An `assistant` or `api-request` record with an earlier `timestamp` than that `dequeue` was
   written late: the start stays deferred and follows the turn's `result`. A record written
   after it, or a record or `dequeue` without a readable time, gives the start on before that
   record. The time is used only to order these records, never to wait. A `dequeue` while a
   request waits is therefore no longer given on at the `dequeue` itself (ruling 1 of
   FIX-RUNNER-FOLLOWUP-AT-TURN-END said "at once" for a working turn).
3. A `user` record with `queueTranscriptOnly` (an input the CLI only notes and never sends to the
   model: the notification of a task, on a resume, with or without `producer`, and within a
   session too; seen with 2.1.326, 2.1.330 and 2.1.331) ends the turn when such inputs alone
   opened it, with a `result` of `num_turns` 0 and no text, as a print stream does. The CLI
   may dequeue several at once, after the late answer of a turn or after a turn ended; the turn
   ends with the last of the inputs that opened it, also when their noted records come before
   the end of the turn before. Each deferred `dequeue` is judged by its own time. A `dequeue`
   in a turn that already works joins it and ends nothing. Without this a start deferred
   behind the late answer of the prompt kept the run open after its answer (a real 2.1.326
   transcript; the stdin rule waited for a result that never came).
4. The runner reads `timestamp` of `assistant`, `api-request` and `queue-operation` records (the
   recording of the pinned CLI lists it) and `queueTranscriptOnly` of `user` records (read in
   real transcripts, not recorded).
5. Such a notification without `producer` stays an input of another sender for the log
   (`background_foreign_input` with origin `other`); the origin rule is not relaxed.

Open minors (no reproduced fault; for the next run in this area):

- A `dequeue` while a request waits, then no output at all before `turn_duration` (an
  interrupt): the `result` now comes before the input's start. No real transcript shows it,
  and the old order took that `result` as the input's answer.
- A later CLI that wrote the input's own `user` record between the `dequeue` and the late
  answer would show its echo before the turn's `result`. No real transcript shows it.
- The fake CLI's `late` line is not reset after the first turn; only the first turn reads it.
- The update chain's log check counts every `background_foreign_input` line as a failure, also
  the `other` line of such a notification after a resume.
- CLI versions up to 2.1.325 wrote noted records right after `enqueue`, without a `dequeue`.
  Such a record between the `dequeue` of a runner line and its request would end that line's
  turn. Not seen in background transcripts from 2.1.326 on.
- A noted record that carries several task ids after as many `dequeue` records would leave the
  count short and the turn open. Not seen as a complete case.
