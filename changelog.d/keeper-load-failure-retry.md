### Fixed

- CI: the platform tests of the apps run a command once more when a test file failed to load
  before any of its tests ran (for example "Failed to start Dart Development Service" on the
  Android emulator); every `flutter test` on a device runs one test file, which a tools test
  keeps so.
