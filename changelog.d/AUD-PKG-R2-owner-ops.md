### Added

- The upgrade guide describes how to upgrade an installation made from a bundle by hand (copy,
  verify, unpack, copy over, new variables of `.env.example`, marker, way back).
- The installation guide has a Caddy edge example that compresses only the static web console;
  the server compresses no response (16-operations.md 4.3).

### Changed

- The Windows package takes a missing Visual C++ runtime file from the redist folder of the
  build toolchain and names its version in the build log, never from `System32` (ADR 0078).
- `make lint` checks `.coderabbit.yaml` against a schema pinned to a commit and a SHA-256
  instead of the vendor's current page.
- `make commitlint` requires a `Signed-off-by` line in the pull request body, the text of the
  squash commit, except in pull requests of bots.
- The API breaking-change check G6 compares with the merge base of the pull request while there
  is no release tag; the pull request label `breaking change` releases a deliberate break before
  1.0.
- The alert for red runs writes its issues: `BOTHY_RED_RUN_ISSUES` is `true`.
- `make perf` runs by hand on a maintainer machine before a release; the texts no longer name a
  release workflow step.
