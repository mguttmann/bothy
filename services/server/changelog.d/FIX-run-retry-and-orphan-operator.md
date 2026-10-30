### Added

- `POST /runs/{run_id}/retry` (`retryRun`) starts a failed or cancelled run again for the same
  triggering message, with the rights of triggering (`agent.invoke`): only the newest run of its
  thread, idempotent, the caller as `triggered_by` (09 5.7, addendum of 2026-10-30).
- `bothy import discord switch` refuses with exit `2` before any write when imported threads of
  the channel name an operator other than the planned operator of the mapping, naming their count
  per agent ID; `--force` switches anyway with a warning (22 13.3.1).

### Fixed

- A thread whose fixed operator is no longer allowed in its channel (not the channel's operator,
  no write of its own on the channel, no active and no completed run in the thread), such as an imported thread after a switch-over
  without the planned operator, no longer starts a run that fails at once for every message; the
  message gets `trigger_state = failed` with the new reason `operator_not_in_channel` (09 3.4
  step 4a).
