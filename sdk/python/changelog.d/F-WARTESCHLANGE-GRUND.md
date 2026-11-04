### Added

- `build_heartbeat` and `RuntimeStatus` carry `run_slots` and `waiting_runs` (09 4.8, 5.2.1),
  checked before the send like every other member; the generated models gain `RunSlots` and
  `WaitingRun`.
