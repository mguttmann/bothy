### Changed

- A `request failed` log record of an error over a database error now carries its SQLSTATE as
  `db.response.status_code` (the code only, never the message), so a `500 internal_error` names
  its database cause (PL/56).

### Fixed

- The serve processes of the `internal/wire` tests and the harness servers keep an idle
  transaction open up to `testkit.Patience` (`database.idle_in_tx_timeout`), so a loaded runner
  that stalls a process inside a transaction no longer loses the session and answers a write with
  `500 internal_error` (flake `TestInvocationDelivery/TestDelivery_RESTPoll`, PL/56).
