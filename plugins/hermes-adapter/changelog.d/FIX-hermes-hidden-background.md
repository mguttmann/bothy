### Fixed

- Work a Hermes agent starts in the background is visible. A run whose turn started subagents
  (`delegate_task`) or background processes stays running, with its stop button, until that work
  is done; each delegation shows as a subagent step and each process as a background job. The
  turn Hermes starts with the result runs inside the same run, so its steps, costs and answer
  belong to it, and a follow-up written meanwhile gets its own turn in the run. Stop interrupts
  the subagents and ends the processes. A turn without an open run can no longer call any tool:
  no delegation, no scheduled job, no terminal command. New settings
  `hermes_adapter.background_poll_interval`, `hermes_adapter.background_settle` and
  `hermes_adapter.background_progress_interval`.
