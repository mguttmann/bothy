### Fixed

- CI: the Android platform tests start a command up to three times when its earlier starts failed
  before any test ran (a stalled app start or a test file that did not load). The adb connection to
  the emulator drops now and then while the app starts, and main run 41308492022 failed after two
  such starts in a row. A failed test still never runs again; the other systems keep two starts.
