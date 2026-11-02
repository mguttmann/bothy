### Fixed

- CI: the state checks of the Claude Code runner scenarios skip an entry that vanished between
  listing and looking at it. The runner keeps polling while a scenario walks its state directory,
  and SQLite creates and removes the rollback journal of the inbox around every transaction, so
  `test_state_files_private_and_retention` failed once with `FileNotFoundError` on
  `inbox.sqlite3-journal`. Every entry that still exists is checked as before.
