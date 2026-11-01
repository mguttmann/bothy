### Added

- The live frame `stats.live` carries per run the agent tree `tasks` (subagents, workflows and
  their agents, background jobs) with parent, kind, state, times and current tool, and
  `tasks_truncated`; the label of a node only for a receiver with `run.read` on the run (25 5.2).
  New configuration values `stats.live_max_tasks`, `stats.live_task_max_depth` and
  `stats.live_task_keep`. Under the payload bound the trees of the oldest runs are dropped before
  any run.
