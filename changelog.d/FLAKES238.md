### Changed

- A request that fails with 503 `unavailable` (reason `deadline_exceeded`) because a connection
  timed out while the request still had time now logs the Go types of the error chain as
  `exception.type` on its `request failed` line, never the error text, so the log names the
  connection that timed out (issue #238).
