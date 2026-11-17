### Fixed

- Background mode: the notification of a background agent, monitor, workflow or timer the
  session launched itself no longer counts as another sender's input, which stopped the
  session and left it unresumable. The CLI queues such a notification without an origin and
  may write it after the first records of the answer; an input is now judged at the request
  that carries it. Inputs of other sessions, of a person and of other origins still stop the
  session.
