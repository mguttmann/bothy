### Changed

- The host service logs a `session.open` that opened no data connection as a warning
  (`terminal data connection not opened`, with `session_id` and `stage`: `open` for an unusable
  expiry, data path or ID, `dial` when the dial, the hello or the identify failed). Before, such
  an opening ended silently and the server's opening ran into `open_timeout` with no trace on the
  host.
- A failed test of the platform suite prints the journal of `bothy-host.service` since its start
  (Linux, at most 30 lines), so a failure on a CI runner names its cause (flaky
  `TestPlatformSuiteLinux`, `TestPlatform_TMT29l`: `no data connection 1 of the session`).
