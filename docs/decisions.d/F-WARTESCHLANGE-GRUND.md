# F-WARTESCHLANGE-GRUND Why a queued run waits

The owner saw a run that showed only "Queued" with an hourglass. The cause was the runner limit
`runner.max_parallel_runs`: four turns ran, two runs waited for a slot, and `runs.state_reason`
was empty. Nobody could see the reason or the runs that held the slots.

Wait reasons found in the code before the change:

- Server: a run is `queued` from acceptance until `run.start`. While an attachment or the server
  transcript of the triggering message is not final, the server holds `agent.invocation`
  (`runs.invocation_held_until`, R-DM-44, R-DM-47). A thread has at most one active run
  (R-DM-14), so a second run in the same thread never waits on the server: the message becomes a
  follow-up or is refused.
- Runner: `prepare` takes the thread lock and the session lock (`flock`, waiting up to
  `runner.session_lock_wait`), then one slot of the semaphore of `runner.max_parallel_runs`.
  Both waits happen while the run is still `queued` on the server.
- Presence: an agent that is `offline` or `unknown` takes no run, so its queued runs wait for it.
- Rate limits: neither the runner nor the server delays a start because of a provider rate limit;
  the CLI retries inside a running turn (`metrics.last_api_retry`). There is no rate-limit wait
  reason for a queued run.

| No | Date | Ruling | Reason |
|---|---|---|---|
| F-WG/1 | 2026-11-04 | A queued run carries its wait reason in `runs.state_reason`: `invocation_held` (set by the server at acceptance when it holds the invocation, cleared at the release), `slots_full` and `thread_busy` (reported by the runner). `run.start` clears the column as before (the reason of a start is empty). No migration: the column exists and has no CHECK. | One column, one place; every view that already reads the run (live box, "Running now", live topology) gets the reason without a new join. |
| F-WG/2 | 2026-11-04 | The runner reports its waits in two new heartbeat members: `run_slots` (`used`, `total`; `total` is `runner.max_parallel_runs`) and `waiting_runs` (`run_id`, `reason` per run that waits in `prepare`). The runner sends a heartbeat at once when a run starts or stops waiting. | The heartbeat is the agent's report of its own state (09 4.8) and already has an immediate send; a run patch is not possible before `run.start` in the SDK. |
| F-WG/3 | 2026-11-04 | The server applies `waiting_runs` like `project_readiness`: when the list differs from the last one this process applied for the agent, or that application is at least `agent.heartbeat_persist_interval` old, in its own unit of work. A listed run that is still `queued` and belongs to the agent gets the reason; a queued run of the agent whose stored reason is a runner reason and that is no longer listed gets none. Each change raises `runs.version` and appends `run.state_changed` without a trigger state. An absent member counts as the empty list. | Full state as every heartbeat member; an older adapter that never reports the member sets nothing. The event lets the clients update the box without a new event type. |
| F-WG/4 | 2026-11-04 | `run_slots` is stored in `agents.reported_details.run_slots` and shown as `AgentRuntimeStatus.run_slots` and `StatsLiveAgent.run_slots`. `StatsLiveRun.state_reason` carries the wait reason of a queued run. | The slot count is a property of the runner, not of one run; for `slots_full` the count used equals the total by definition. |
| F-WG/5 | 2026-11-04 | `agent_offline` is not stored: a client shows it when the agent of a queued run is `offline` or `unknown`, ahead of a stored runner reason and after `invocation_held`. A queued run without a reason shows "waiting for the agent to start it". One function in `design_system` (`dsWaitReason`, `dsWaitText`) decides this for every client. | The presence changes without a write to the run; clients already receive it (`agent.presence_changed`, `StatsLiveAgent.presence`). |
| F-WG/6 | 2026-11-04 | The runs that hold the slots are the visible runs of the same agent in `running`: the live topology takes them from the frame, the thread view and "Running now" from the device's synced runs (the runs table that "Running now" fills from `listActiveRuns`, which keeps only runs the caller may read). Each opens its thread; a run without a readable chat is not a link. Slots in use that the device does not show are counted, not named. | Both sources hold only runs the caller may read (25 section 4 rule 6, D-M2-05-9), so a blocking run the caller may not read is counted but never named; the device's runs table needs no request per box. |
| F-WG/7 | 2026-11-04 | The trigger state badge of the triggering message stays "Queued"; the reason is shown in the live box, the "Running now" row and the topology. | `messages.trigger_state_reason` is allowed only for `failed` and `cancelled` (CHECK of 00032); changing that is a migration of another table for a text the live box above the message already shows. |
