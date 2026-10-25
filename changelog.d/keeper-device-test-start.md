### Fixed

- CI: the platform tests of the apps run a command once more when a test file failed to load
  before any of its tests ran (for example "Failed to start Dart Development Service" on the
  Android emulator), or when it printed nothing for 15 minutes right after the build of the app
  (flutter waiting for the app on the iOS simulator); every `flutter test` on a device runs one
  test file, which a tools test keeps so.

### Changed

- CI: the iOS simulator tests of the apps run in their own job `apps (iOS)` (system `ios` of
  `platform-tests.json`), in parallel with the desktop tests of `apps (macOS)`.
