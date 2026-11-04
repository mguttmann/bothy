### Added

- `buildHeartbeat(state, { runSlots, waitingRuns })` sends `run_slots` and `waiting_runs`
  (09 4.8, 5.2.1), checked before the send like every other member.
