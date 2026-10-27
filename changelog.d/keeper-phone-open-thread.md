### Fixed

- Tests: the app integration tests tap a target of a page that was just pushed only once it
  stands still on the screen (`untilStill` in `test/support/ready.dart`, also part of
  `tapWhenEnabled`). A page slides in, and a tap during the slide landed beyond the screen and
  did nothing: the thread row of the phone tests (`openThread`, `apps (iOS)` voice test) and
  "Log in again" on E7 (`apps (macOS)`, `logout_wipes_test`).
