### Added

- A daily scan of the images an instance runs: the workflow `images.yml` builds bothy-server and
  bothy-postgres and runs Trivy over them and over the third-party images of the Compose files.
- `tools/distribution/ios.py promote --group beta` adds an uploaded build to the TestFlight group
  beta when a release stopped before that step.

### Changed

- Semgrep runs from its own locked project (`tools/semgrep`), with every dependency pinned by
  hash.
- `docs/quality/toolchain.md` has dated reviews of the pins and of the Trivy and WiX holds, and
  a test fails once a date has passed. Sparkle and bundletool are listed there and found by
  Renovate.
- The release job of the Apple apps may run 240 minutes.

### Fixed

- The store requests of a release retry a refusal for the rate and, for reads, a passing server
  fault, instead of ending the release half way.
- The appcast is replaced in one step, so an abort no longer leaves a truncated feed, and a
  failed restore of the branding files no longer hides the error of the build.
