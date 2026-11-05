### Fixed

- The CI jobs `apps (iOS)` and `apps (macOS)` restore the Swift package cache, keyed on the
  workspace lock, so a run without a dependency change does not download the sentry-cocoa
  binary targets from GitHub releases again. A failed download of them, shown only as "already
  exists in file system", failed both attempts of the iOS job on main.
