# Open minors of the red-run follow-up (RED-RUNS-1128)

Collected minors without a fix in RED-RUNS-1128; the next run in the CI area reads and takes them
along.

- Nothing warns when a CI job creeps towards its `timeout-minutes`; four jobs reached their limit
  before anyone saw it (RED-RUNS-1128/1). A note from `ci_ok.py`, which already reads every job
  of the run, could name a job above a share of its limit.
- `tools/ci/red_runs.py` fails the whole hourly report on a single `5xx` of the GitHub API
  (run 45435643214, `502`); `flakes._gh` has no retry.
- Gradle in the Android jobs fails at once on `429 Too Many Requests` from Maven Central (runs
  45230736728, 45257930304, 45339899878); not seen since 2026-11-24.
