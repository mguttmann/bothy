### Fixed

- CI: the Android phone integration tests of the client run as an instrumented test through
  Gradle instead of `flutter test` on the emulator, so they no longer depend on a connection from
  the runner to the app's VM service. On the API 36 emulator that connection dropped while the app
  started, and main runs 41308492022 and 41443318002 failed before any test ran. A failed Android
  attempt also prints the end of the adb server log.
