### Fixed

- CI: the desktop platform tests of the apps run a command once more when its native assets build
  failed before any test ran, and print the logs of the build hooks, so a transient download error
  of a build hook no longer turns `apps (Windows)` or `apps (macOS)` red.
