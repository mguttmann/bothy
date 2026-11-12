### Added

- The workflow `red-runs.yml` reports the failed runs that no pull request shows: the scheduled
  runs of every workflow with a schedule and the pushes to `main` of `ci.yml`. It is switched
  off for writing: until the repository variable `BOTHY_RED_RUN_ISSUES` is `true` it only lists
  the red runs with their jobs and steps in the log of its job. With the variable set it keeps
  one issue per workflow, `Red runs of <file>` with the label `red-run`, with a comment per
  failed run, and closes it when the workflow passed again. A run that GitHub lists as
  cancelled counts when one of its jobs ran and did not pass (AUD-MISC-TESTING-01/1 to /3 and
  /11).
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
- The driver of the Fake-Agent of the desktop E2E stack no longer ends when the Fake-Agent
  rebased a patch after `409 base_version_conflict`. It read the rebased patch as a failure,
  so the flow with the agent run failed whenever the server changed the run a moment earlier
  (AUD-MISC-TESTING-01/12).
