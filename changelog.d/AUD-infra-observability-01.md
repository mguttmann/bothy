### Added

- Alert `BothyBackupTelemetryMissing` (`page`): `bothy-backupd` has sent no telemetry for 10
  minutes. Until now nothing reported a backup service that was down or could not export,
  although it sends every value the backup, WAL archive, restore test and volume alerts read.

### Fixed

- The alerts `BothyBackupStale`, `BothyWalArchiveStale` and `BothyRestoreTestStale` stayed quiet
  when their value was missing: a backup kind or a restore test that had never succeeded, and
  a backup service that no longer reported. They now also fire when no value was reported for as
  long as their threshold (26 hours per backup kind, 15 minutes, 8 days). On an installation
  with the profile `observability`, `BothyBackupStale` therefore fires for every backup kind
  that has not succeeded yet, also for `objects` with the driver `s3` and no off-site target,
  whose runs always fail (`docs/operations/install-compose.md`, section 4).
