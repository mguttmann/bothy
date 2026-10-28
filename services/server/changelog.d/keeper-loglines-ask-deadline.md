### Fixed

- `TestLogLines` (appdispatch) no longer fails with `bothy.reason:timeout` on a loaded runner. Its
  autocomplete request expects `invalid_reply` from an empty answer of the fake app, but had a
  100 ms deadline, so a slow round trip ended as a timeout. The request now uses
  `testkit.Deadline()` and checks `invalid_reply` at the call.
