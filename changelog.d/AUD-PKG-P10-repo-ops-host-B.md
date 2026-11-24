### Fixed

- CI jobs that run Gradle restore a cache of their downloaded dependencies per job, and both
  Gradle builds retry a repository request that fails with 408, 429 or a 5xx six times with a
  growing pause, because Maven Central refused downloads with HTTP 429 (AUD-PKG-P10-repo-ops-host).
- The Android emulator jobs fail without a device instead of passing without a test, and `ci-ok`
  requires the SonarQube scan in a pull request that has the secrets for it
  (AUD-PKG-P10-repo-ops-host).

### Changed

- `make test` runs the tests of the setup wizard `install.sh` only with the area `tools`, so a full
  CI run starts their container once; the workflow policy check refuses a write permission on top
  level and the `workflow_run` trigger (AUD-PKG-P10-repo-ops-host).
