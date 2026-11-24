### Fixed

- Background mode with Claude Code 2.1.331: the CLI writes the record of a turn's final answer
  only after the turn's stop hooks, so a follow-up that arrived meanwhile was dequeued before
  that record in the transcript. The runner took the end of the turn as the answer to the
  follow-up, stopped the session while the model worked on the follow-up, and the run failed
  with `runtime_error`. The start of such an input now follows the end of the turn before when
  the answer record carries an earlier time than the dequeue, and the session stays until the
  follow-up's own turn ends. 2.1.330 and earlier are read as before.
