### Fixed

- The CI jobs `apps (iOS)` and `apps (macOS)` restore the Swift package cache, keyed on the
  workspace lock, so a run without a dependency change does not download the sentry-cocoa
  binary targets from GitHub releases again. A failed download of them failed the iOS job on
  main twice in one run and the macOS job once, shown only as "already exists in file system".
