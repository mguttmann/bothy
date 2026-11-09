### Changed

- Every request that fails with a 5xx now logs the Go types of the error chain as
  `exception.type` on its `request failed` line, also when the error has a kind (an adapter's
  `internal_error`, a 503 `deadline_exceeded` of the database pool), never the error text, so the
  log names the source of the error (issue #238).
