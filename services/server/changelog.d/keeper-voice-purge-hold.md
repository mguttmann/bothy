### Fixed

- The voice and privacy test suites give the file purge job a no-op, so a test that reads the row
  of a deleted file no longer races with the purge (flaky `TestVoice_ServiceFailures/message_deleted`).
