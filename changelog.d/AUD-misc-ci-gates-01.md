### Changed

- Every CI job installs the same exact uv (`version` of each `astral-sh/setup-uv` step, 0.13.7)
  instead of the newest release at the time of the run; Renovate moves the pin after the
  cooldown (AUD-CG1/4).
- `build-verify.yml` keeps the time limit, the runner and the make commands of `ci.yml` for
  every job both have (`apps (iOS)` 90 minutes, `test (server)` and `test (apps)` 60 minutes,
  `AREAS=none` in two test jobs) and validates the Gradle wrappers as `ci.yml` does; a test
  holds the two files to each other (AUD-CG1/2, /3).
- docs/quality/gates.md has the section "Red runs": who reads a red run of `main` or of a
  scheduled workflow, and which runs report nothing by themselves (AUD-CG1/7).

### Fixed

- A manual run of `ci.yml` on `main` no longer cancels the push run of `main` in progress: only
  a newer run of the same pull request cancels a run (AUD-CG1/1).

### Security

- The daily pin proposal of the Hermes adapter (`hermes-pin.yml`) runs the unreviewed upstream
  code in a container without root, without capabilities and with a read-only checkout
  (`tools/ci/sandbox.sh`) instead of as a plain step, which could reach the credentials of a
  run on `main`; only regular fixture files leave the job (AUD-CG1/5, /6).
