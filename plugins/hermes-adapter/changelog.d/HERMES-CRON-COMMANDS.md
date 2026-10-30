### Added

- Hermes commands: the adapter declares `/context` and `/version` as control commands and every
  skill Hermes offers for the platform as a model command with an optional `input`. A control
  command runs in the thread's Hermes session, also during a run, and Hermes' reply becomes a
  message of the agent without a run; a skill command opens a run whose turn is Hermes' own skill
  invocation. `/stop`, `/new`, `/resume`, `/title` and `/status` stay the server's built-in
  commands, and every other Hermes command is refused for a stated reason.
- Reports without a run: Hermes cron jobs and `hermes send` deliver to the platform, also from a
  process without the gateway. A forum gets a new post per report, titled by the report's first
  line; a text channel gets the report in its main thread; `<key>:<channel>:<thread>` posts into
  a fixed thread. The installer's `--report-channel` (and `--report-thread`) set the home channel
  that `deliver=<key>` uses.

### Changed

- A message Hermes sends outside a turn of its thread is posted as a report of the agent instead
  of failing with "no active run in this thread"; a late answer of an ended run still fails.
