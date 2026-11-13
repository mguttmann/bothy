### Added

- Four keys set how a run with background work waits for the account of Claude Code:
  `runner.account_retry_first` (default `1m`), `runner.account_retry_second` (`2m`),
  `runner.account_retry_interval` (`3m`) and `runner.account_retry_max_wait` (`20m`; `0s`
  switches the retry off).

### Fixed

- A run with background work no longer ends when the account of Claude Code does not work for
  a moment. When a turn ended with a usage or rate limit, a login that was gone, switched-off
  subscription access or an overloaded API, the runner closed the turn's input and stopped the
  process `runner.stop_grace` later, so every background task, subagent and workflow of the
  session ended, and nothing went on until a person wrote. That also happened in the turn
  Claude Code starts by itself for the notification of a background task. The runner now
  keeps the process and its tasks, shows in the live box that the run waits for the account,
  with the kind of the error and the reset time of a usage limit, and tries the turn again
  after short pauses (the keys above). It never waits for the reset time itself. A stop works
  at every moment.
- A run that such an error does end, because the account did not come back within
  `runner.account_retry_max_wait` or because the session had no background work to keep, names
  the error in its summary: its kind, the reset time of a usage limit, how often the runner
  tried again and how many background tasks were lost. Before, the thread showed a runtime
  error without a text.
