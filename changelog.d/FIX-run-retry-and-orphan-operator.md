### Added

- The line of a failed or cancelled run that is the newest run of its thread has a button
  "Retry" ("Erneut versuchen") on the desktop and mobile apps; it starts the run again for the
  same message through the outbox, sends once per activation and shows the reason when the
  server refuses (01 6.2, 09 5.7). The fake server of the sync engine answers `retryRun`.

### Fixed

- A message in a thread whose agent no longer works in the channel shows "The agent of this
  thread no longer works in this channel" instead of starting a run that fails at once
  (`operator_not_in_channel`, 09 3.4 step 4a).
