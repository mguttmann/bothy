### Fixed

- The image tests of `bothy-postgres` (`test/postgresimage`) no longer depend on the time of
  day: `bothy-backupd` in these tests ran its default schedules on the wall clock, so a test that
  ran at 03:00 or 03:30 UTC got a `logical` or `objects` run it did not start (flaky
  `TestBackupKinds/one_succeeded_backup_runs_row_per_run`). Every stack process of the package now
  gets every schedule key of the configuration set to one that never fires, and
  `TestServerEnvHoldsEverySchedule` fails when a new schedule key is not held.
