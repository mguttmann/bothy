### Changed

- Background mode: a session of the runner's own agent under `scoped` and `host` can send
  `SendMessage` to an agent it launched with the `Agent` tool, by the `agentId` of the spawn
  result, so a running background agent gets more to work with instead of a rework round
  afterwards. A hook in the run's settings file refuses every other recipient: another Claude
  Code session of the host (by name or by a `uds:` or `bridge:` address), a workflow agent of
  the session, and an agent named by its name. `ListAgents` and `SendFile` stay disallowed.
  Where hooks cannot run (the host user's `disableAllHooks`, a managed setting that switches
  hooks off, a hosted turn) `SendMessage` stays disallowed as before.
