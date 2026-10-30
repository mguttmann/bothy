### Added

- `bothy_sdk.testing.sdkserver` drives the real server of this tree for tests (`RealServer`,
  `build_sdkserver`, `start_sdkserver`): the scenario server of the SDK's real-server tests plus
  the world of the program, the run of a message, uploads and any operation as Alex or the owner.

### Changed

- The FakeServer answers `getRunContext` of a run whose trigger is not a message with `500
  internal_error`, and its `thread_context` holds only the messages before the trigger, as on
  the real server (09 4.6).
