### Fixed

- Background mode: Claude Code may write the record of a turn's final answer seconds after the
  answer came (seen with 2.1.326, 2.1.330 and 2.1.331), so a follow-up that arrived meanwhile
  was dequeued before that record in the transcript. The runner took the end of the turn as the answer to the
  follow-up, stopped the session while the model worked on the follow-up, and the run failed
  with `runtime_error`. The start of such an input now follows the end of the turn before when
  the answer record carries an earlier time than the dequeue, and the session stays until the
  follow-up's own turn ends. An input the CLI only notes (`queueTranscriptOnly`, the
  notification of a task left by the process before) now ends the turn it opened, so such a
  start after the late answer no longer keeps a run open after its answer.
