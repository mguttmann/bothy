### Fixed

- Background mode: when a follow-up reached the session just as a turn ended, Claude Code
  2.1.326 dequeued it before it wrote the end of that turn. The runner took the end of the turn
  as the answer after the follow-up had started, released the session once the follow-up was
  taken and so stopped it while the model was working on the follow-up; the run failed with
  `runtime_error`. The start of such an input now follows the end of the turn before, and the
  session stays until the follow-up's own turn ends.
