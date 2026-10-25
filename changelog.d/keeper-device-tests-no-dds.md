### Fixed

- CI: the integration tests of the apps on a device (macOS, Windows, the Android emulator and the
  iOS simulator) run `flutter test` with `--no-dds`, so a Dart Development Service that fails to
  start on a slow device no longer turns the job red before any test ran; a tools test keeps
  every such command on it.
