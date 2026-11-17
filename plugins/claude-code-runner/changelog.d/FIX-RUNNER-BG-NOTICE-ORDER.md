### Fixed

- Background mode: the CLI writes the attachment that attributes a queued notification right
  after the request that carries it, not before. The runner judged the input at that request
  and stopped the session as foreign although the notification was its own (a background
  agent or command it launched). An input is now judged at the first output of the request
  that carries it, or at the end of the turn; an input nothing attributes still stops the
  session before any answer to it.
