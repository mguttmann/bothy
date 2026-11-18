### Fixed

- Background mode: the CLI writes the attachment that attributes a queued notification right
  after the request that carries it, not before. The runner judged the input at that request
  and stopped the session as foreign although the notification was its own (a background
  agent or command it launched). An input is now judged at the first output of the request
  that carries it, or at the end of the turn; an input nothing attributes still stops the
  session before any answer to it.
- Background mode: a supervisor that ended on its own between `claude daemon status` and the
  runner's look at its process (an idle supervisor exits after its last session) counted as a
  supervisor of another program, so the run used print mode and the host reported
  `background_unavailable`. The runner now reads the status once more in that case.
