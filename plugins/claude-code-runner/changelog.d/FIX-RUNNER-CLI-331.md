### Fixed

- Background mode: Claude Code may write the record of a turn's final answer seconds after the
  answer came (seen with 2.1.326, 2.1.330 and 2.1.331), so a follow-up that arrived meanwhile
  was dequeued before that record in the transcript. The runner took the end of the turn as
  the answer to the follow-up, stopped the session while the model worked on the follow-up,
  and the run failed with `runtime_error`. The start of such an input now follows the end of
  the turn before when the answer record carries an earlier time than the dequeue, and the
  session stays until the follow-up's own turn ends.
- Background mode: inputs the CLI only notes (`queueTranscriptOnly`, such as the notifications
  of tasks left by the process before, also several dequeued at once) now end the turn they
  alone opened. Before, such a turn never ended and a run whose session held background work
  stayed open after its answer.
