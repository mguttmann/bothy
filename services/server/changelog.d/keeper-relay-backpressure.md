### Fixed

- A reattach to a terminal session with a full scrollback no longer fails with `4014` when the
  device reads slower than the host sends: the relay stops reading from the sender while the queue
  toward the other side is full and closes the receiver only when it takes nothing within the new
  `terminal.relay_stall_timeout` (default `10s`).
