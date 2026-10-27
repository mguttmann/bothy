### Changed

- `make apps-platform` runs the iOS share device tests with `patrol --verbose`, so a failed
  `xcodebuild build-for-testing` prints its errors. Before, `apps (iOS)` on main showed only
  `** TEST BUILD FAILED **` with exit code 65, and the cause of the failed build was lost.
