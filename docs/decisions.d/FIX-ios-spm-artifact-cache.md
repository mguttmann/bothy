# FIX-ios-spm-artifact-cache The Mac app jobs restore the Swift package cache

| No | Date | Ruling | Reason |
|---|---|---|---|
| FISC/1 | 2026-11-05 | The jobs `apps-ios` and `apps-macos` of `ci.yml` and `build-verify.yml` restore `~/Library/Caches/org.swift.swiftpm` with `actions/cache`, keyed on `runner.os`, `runner.arch` and `hashFiles('pubspec.lock')`, without `restore-keys`, before `tools/ci/apps_platform.py` runs. | Main run 42044761383 (commit 385186d9a): in `apps (iOS)` both attempts failed with `failed downloading '.../sentry-cocoa/releases/download/9.30.2/<target>.xcframework.zip' ... <cache>/artifacts/<entry> already exists in file system`; in `apps (macOS)` the first attempt failed the same way and the second passed. The job had no Swift package cache, so every run downloaded the binary targets of sentry-cocoa from GitHub releases. SwiftPM reports a failed download this way: its HTTP client moves the body of a 5xx response to the cache path and retries, the retry finds the path taken, and the fetch removes the entry again (`Workspace+BinaryArtifacts.swift`, `URLSessionHTTPClient`, `HTTPClient` in swift-package-manager 6.1; reproduced with two resolves against one cache path). So a leftover entry was not the cause, and the retry of `apps_platform.py` met a new failed download. `sentry_flutter` pins sentry-cocoa exactly, so the workspace lock decides the version; `actions/cache` saves only after a passing job, so only checked artifacts are cached. |
| FISC/2 | 2026-11-05 | The distribution trial jobs (`mobile-trial-ios`, `host-trial-macos`, `desktop-trial-macos`) and `release.yml` get no Swift package cache. | Minimal change: the trials run only for changes of the distribution files, and a release build should not depend on a cache. |

## Open minors

- On a cache miss (new lock) the download is unprotected as before.
- The cache adds roughly 0.8 GB of zips to the repository cache, which already is at its limit, so older entries are evicted sooner.
- Swift packages of path members (`packages/*/darwin`) are not in `pubspec.lock`; a new remote binary target there would not change the key.
