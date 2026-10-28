### Fixed

- The start wait for the database (`WaitReady`, `Connect`) keeps the cause of an earlier attempt
  (for example a missing database) also when the last attempt ends with an i/o timeout of the
  socket instead of the end of its context, and logs such an attempt with `error.type` timeout
  (flaky `TestWaitReadyKeepsTheCause`).
