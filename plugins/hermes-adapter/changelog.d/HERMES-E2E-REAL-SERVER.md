### Fixed

- An answer in the thread that a mention in the main thread opened (`auto_thread`) is posted
  again: the adapter quoted the triggering message, which lies in the main thread, the server
  refused the reply (`not_in_thread`) and the run lost its answer. The answer now quotes only a
  message of the run's thread.
- A reply that Hermes' delivery ledger sends again ("Recovered reply ... may be a duplicate")
  after a failed send or a restart is no longer posted a second time as a report when the adapter
  took it over as an answer already.
- A follow-up during a run (with `busy_input_mode` `steer`) and a typed answer to an open
  question reach the running turn again: the adapter looked their text up in the run context,
  which holds only the messages before the trigger, and passed nothing. It now reads the message.
- A skill command no longer fails before its start: the adapter read a run context, which the
  server does not define for a command run (`500`); the turn now has the fixed note and the
  channel instructions as its system part.

### Added

- An end-to-end suite runs the adapter's own code with the fake Hermes runtime against the
  server of this tree (`make adapters-e2e`, CI job `test-adapters-e2e`): mentions with
  `auto_thread`, replies, forum posts, reports, files and voice out, voice in, commands, cards,
  follow-ups, edits, titles, cancel, retry and the recovery path.
