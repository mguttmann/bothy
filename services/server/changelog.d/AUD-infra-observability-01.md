### Fixed

- `bothy-backupd` reads the last successful runs for the metrics
  `bothy.backup.last_success.timestamp` and `bothy.backup.restore_test.last_success.timestamp`
  again every minute until it could read them once. It read them only at its start: when the
  database did not answer at that moment, the metrics stayed empty until the next successful run
  of each kind, up to a week for the restore test, and the alerts on them had no value.
