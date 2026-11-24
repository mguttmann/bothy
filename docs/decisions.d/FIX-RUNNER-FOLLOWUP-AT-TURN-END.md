# FIX-RUNNER-FOLLOWUP-AT-TURN-END Background mode: a follow-up the CLI takes as a turn ends

A live probe of a runner release on 2026-11-18 (Claude Code 2.1.326, background mode) failed:
the turn moved its command to the background and ended, and a follow-up of the run's owner
reached the session within the same moment. The run ended `failed` with `runtime_error` (no
error text, exit `0`): the runner had stopped the session itself while the model was working on
the follow-up.

Evidence, read in the session's transcript and the runner's records of that run:

- The CLI wrote `queue-operation` `enqueue` and `dequeue` of the follow-up before the
  `turn_duration` of the turn that ended, and the follow-up's `user` record (origin `peer`,
  the runner's `uuid`, the owner code) right after it. The runner took the line as its own
  (echo, follow-up `absorbed`); the owner code and the envelope were not the cause.
- The mapper gave `command_lifecycle` `started` at the `dequeue`, then the `result` of the turn
  before. The stdin and stop rule took that `result` as the answer after the input had started
  (the guard of FIX-runner-hold-background assumes the order `result`, then `started`), so once
  the echo came nothing was pending and the rule released the session, which in background mode
  is a stop. The follow-up's turn never answered, and the stopped background command came back
  as a notification in the next run's process.

Rulings:

1. A `dequeue` after the open turn's final answer (an assistant record with `stop_reason`
   `end_turn` and no request since) and before the turn's end is given on after that turn's
   `result` (`started`, then the start of the next turn), the order of a print stream. The rule
   then sees the `result` while the input is still queued, and waits for the result of the
   input's own turn. A `dequeue` while the turn still works (a real transcript of the pinned CLI
   has one mid-turn) is given on at once as before, and a deferred start is given on at once
   when the turn goes on after all (a request or another assistant record), so no start can
   keep a finished turn open. Nothing else changes: whose input a record is stays decided by its origin and `uuid`
   alone, and a forged line in the same shape stays foreign.
2. The regression test reads structure-only records derived from the real transcript
   (`tests/fixtures/transcript/followup_at_turn_end.jsonl`); every text, id, path and number in
   it was replaced.

Open minors (for the next run in this area of the runner):

- `TranscriptReader._turn_follows` scans only lines already read. When a read ends right after
  the `turn_duration` and the follow-up's record comes in the next read, a busy session list
  reports background work for a moment before the deferred start. Harmless for the stop rule
  (it holds), and older than this fix; the mapper could tell the reader that a start is
  deferred.
- The scenario test waits a fixed two seconds to show that the session is not released.
- No test yet for a start that was deferred and then given on by a request before the turn's
  end; the fixture `subagent_notice/s.jsonl` carries no `stop_reason`.

Addendum of 2026-11-24: a `dequeue` while a request waits for its output is deferred too, and
a late final answer is recognised by its earlier `timestamp` (FIX-RUNNER-CLI-331).
