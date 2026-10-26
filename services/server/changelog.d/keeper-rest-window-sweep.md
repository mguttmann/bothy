### Fixed

- A REST poll of an agent consumer no longer loses its new window to the idle sweep while it
  takes the consumer over: the next poll repeated the delivered events within
  `agent.ack_timeout` and without `redelivered`.
