### Added

- `bothy-backupd` counts every failed backup run in the new metric `bothy.backup.failures`, with
  `bothy.backup.kind` the kind of the run (`pg_full`, `pg_diff`, `logical`, `objects`), so a
  full backup that keeps failing shows next to differential backups that succeed. A run that a
  stop of the service canceled does not count.

### Fixed

- A backup run or restore test whose process was killed (a kill, an out-of-memory stop, a crash
  of the host) no longer stays `running` in `backup_runs` for ever, and the dump of the database
  and the password file of `pg_dump` no longer stay in the volume `backup-scratch`: when
  `bothy-backupd` starts, it empties the dump directory and closes every such row as `failed`
  with the code `interrupted`.
- `bothy-backupd backup`, `restore-test` and `expire` stop cleanly on Ctrl-C or `SIGTERM`: the
  run finishes its row as `failed` with the code `canceled` and removes its files. Before, the
  process ended at once and left both behind.
- The series of `bothy.backup.failures` and `bothy.backup.restore_test.failures` exist with the
  value 0 from the start of the process, so an alert rule sees the first failure after a start.
