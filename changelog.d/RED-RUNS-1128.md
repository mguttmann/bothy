### Fixed

- The CI jobs `test (apps)`, `test (SDKs and plugins)`, `apps (macOS)` and `apps (iOS)` get
  higher time limits, so a passing run is cancelled at the limit far less often.
- A TypeScript SDK loop test no longer sends its cancel before the run's handler returned, which
  failed it now and then.
