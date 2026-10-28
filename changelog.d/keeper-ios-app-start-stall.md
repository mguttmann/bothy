### Fixed

- The CI job `apps (iOS)` no longer runs into its time limit when the first start of the app on
  the simulator stalls. `tools/ci/apps_platform.py` now stops a command that is silent after the
  build or install of the app after `APP_START_SILENCE_SECONDS` (8 minutes) instead of the general
  15 minutes and runs it once more, and the job limit rises from 75 to 90 minutes, so a stalled
  start and its second run fit in it (PL/57).
