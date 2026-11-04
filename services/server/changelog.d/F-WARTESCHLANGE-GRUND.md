### Added

- Wait reasons of a queued run in `runs.state_reason` (09 5.2.1): `invocation_held` from the
  acceptance until the release of a held invocation, and `slots_full` or `thread_busy` from the
  new heartbeat member `waiting_runs` of an adapter; `run.start` clears it. Every change raises
  the run's version and appends `run.state_changed` without a trigger state.
- Heartbeat member `run_slots` (`used`, `total`), stored in `agents.reported_details` and shown
  as `AgentRuntimeStatus.run_slots` and `StatsLiveAgent.run_slots`; `StatsLiveRun.state_reason`
  carries the wait reason of a queued run.
