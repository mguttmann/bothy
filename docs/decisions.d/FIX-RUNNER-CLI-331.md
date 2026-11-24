# FIX-RUNNER-CLI-331 Background mode: the late final answer of Claude Code 2.1.331

The daily switch of a runner to Claude Code 2.1.331 on 2026-11-24 was rolled back: a follow-up
written into a running background turn ended the run `failed` with `runtime_error`, while a
plain run and a run with an attachment passed. 2.1.330 passed the same probe on 2026-11-22.

Evidence, read in the transcripts of both probes:

- 2.1.330 wrote the turn's final `assistant` record, then the follow-up's `queue-operation`
  `enqueue` and `dequeue`, then `stop_hook_summary` and `turn_duration`.
- 2.1.331 wrote `enqueue` and `dequeue` first and the final `assistant` record after them, right
  before `stop_hook_summary`; that record carries the time the answer came, before the
  `dequeue`'s. The record of a turn's final answer is written only after the turn's stop hooks.
- The mapper saw the `dequeue` while the last request had no output yet and gave the start at
  once (the rule of FIX-RUNNER-FOLLOWUP-AT-TURN-END defers only after a final answer). The
  `result` of the turn before then counted as the follow-up's answer, the stdin and stop rule
  released (stopped) the session while the model worked on the follow-up, and the run failed,
  the failure of FIX-RUNNER-FOLLOWUP-AT-TURN-END in another record order.

Rulings:

1. A `dequeue` while the open turn's last request has no output yet is deferred like one after
   the final answer. The time of the first deferred `dequeue` is kept.
2. An `assistant` or `api-request` record with an earlier `timestamp` than that `dequeue` was
   written late: the start stays deferred and follows the turn's `result`. A record written
   after it, or a record or `dequeue` without a readable time, gives the start on before that
   record, as before. The time is used only to order these records, never to wait.
3. The runner reads `timestamp` of `assistant`, `api-request` and `queue-operation` records;
   the recording of the pinned CLI lists it for all three.

Open minors (no reproduced fault; for the next run in this area):

- A `dequeue` while a request waits, then no output at all before `turn_duration` (an
  interrupt): the `result` now comes before the input's start. No real transcript shows it,
  and the old order took that `result` as the input's answer.
- A later CLI that wrote the input's own `user` record between the `dequeue` and the late
  answer would show its echo before the turn's `result`. No real transcript shows it.
- The fake CLI's `late` line is not reset after the first turn; only the first turn reads it.
