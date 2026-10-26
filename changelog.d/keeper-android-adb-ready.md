### Fixed

- CI: before each attempt of an Android platform test the adb server restarts and the script waits
  for the one device to be listed and booted; a device that never becomes ready fails with the
  reason. A lost emulator connection no longer fails the run and its automatic second attempt.
