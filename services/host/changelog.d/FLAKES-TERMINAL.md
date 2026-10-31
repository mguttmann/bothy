### Fixed

- The control connection no longer loses a `session.open` that the server sends right after the
  first report of a start. That report comes from the start sequence, and the connection counted
  it as sent only after the write returned; a `session.open` read in between counted as one before
  the report, so the host service closed the connection with `4002`, logged in again and opened no
  data connection (flaky `TestPlatformSuiteLinux`, `TestPlatform_TMT29l`: `no data connection 1
  of the session`). The report now counts as sent from its write on.
