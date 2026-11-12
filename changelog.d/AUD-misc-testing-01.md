### Added

- The workflow `red-runs.yml` opens one issue per workflow, `Red runs of <file>` with the label
  `red-run`, for the failed runs that no pull request shows: the scheduled runs of every
  workflow with a schedule and the pushes to `main` of `ci.yml`. Every failed run adds a comment
  with its jobs and steps that did not pass, the issue lists the jobs by how often they failed,
  and it is closed when the workflow passed again. A run that GitHub lists as cancelled counts
  when one of its jobs ran and did not pass (AUD-MISC-TESTING-01/1 to /3).
- `e2e-desktop.yml` runs the desktop E2E suites headless every night on a Linux runner, against
  the real server of the Compose stack and the terminal's client stack; before, CI only built
  them (AUD-MISC-TESTING-01/6).

### Fixed

- The phone terminal suite of the nightly mobile E2E scrolls the Me list to "Hosts" before it
  taps the entry. On the small screen of the hosted emulator the entry is not built until the
  list scrolls, so the suite had failed at that tap in every night (AUD-MISC-TESTING-01/5).
- The header of `host-platform.yml` no longer calls the runner `windows-2025` Windows 11
  (AUD-MISC-TESTING-01/9).
- The test CA of `infra/compose/e2e.sh` writes a leaf with its subject and authority key
  identifiers. Without them the Python Fake-Agent of the stack refused the TLS front on a host
  whose `openssl` does not add them on its own, and no E2E stack started there
  (AUD-MISC-TESTING-01/10).
