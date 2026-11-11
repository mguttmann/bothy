### Fixed

- A stop now reaches a run that went on after the runner had stopped a failed process of it. When
  a turn filled the context while its background work was still running, the runner stopped that
  process, compacted the context and tried the turn again; stopping the run, revoking access or
  deleting the forum then had no effect until the retry ended by itself.
- An unexpected error of the runner while a turn runs (for example a state folder without space
  left) no longer leaves the Claude Code process running without a run. The turn is stopped
  first, then the run ends `failed` with `adapter_error`, also for a turn the runner took over
  after a restart, which before stayed `running` with nobody following it.
- A turn of a built-in agent that a crashed or killed runner left running is stopped at the next
  start of the runner and its record removed. Before, it ran on unseen and out of reach of a
  stop, kept its thread locked, and its record and temporary files stayed for good.
- An orderly stop of a runner that hosts several built-in agents stops their connections
  together, so the stop no longer takes longer with every agent that has a turn running.
