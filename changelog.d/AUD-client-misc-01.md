### Fixed

- A released app reports the version of its package. Before, every release build reported
  `0.0.0`, so an instance with a minimum app version refused every app as too old, and the
  device list, telemetry and crash reports named `0.0.0`. The release steps now stop when the
  built app does not carry the version it is released as.
- Stop of a run tells what became of it: the button is busy or gone while the stop is on its
  way, a second click sends nothing, and when the server refuses the stop a notice says why and
  Stop is offered again. A refused stop no longer stays in the outbox.
- An owner who started the app without a network gets the owner's entries (the emergency page,
  built-in agents, delete, the switch for admin notifications) once the connection is back,
  without starting the app again. The notification levels and quiet hours are loaded again when
  their first load failed and after the device loaded everything anew.

### Changed

- Twelve integration tests of the desktop app that no job ran now run on macOS and Windows in
  one app build, and a test file that no platform job reaches fails the tools tests.
