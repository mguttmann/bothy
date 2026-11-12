### Added

- Alerts for failed backups in the Prometheus rules of the observability profile:
  `BothyBackupRunFailed` (ticket, per kind of run), `BothyRestoreTestFailed` (ticket, per failed
  check of the automatic restore test) and `BothyRestoreTestAuditChainFailed` (page, the check
  of the audit chain). The backup runbook promised alerts for a failed restore test that no rule
  provided; a failure showed only through the age of the last success.

### Fixed

- backup-restore.md names per backup only the checks that exist: the automatic restore test
  proves the local repository only, not the off-site copy, the logical dump or, with the driver
  `s3`, the objects (section 7 item 5).
- backup-restore.md and upgrade.md name the window of a point-in-time restore: without an
  off-site target and with the default `backup.pg.local_retention_full`, every full backup,
  also one run by hand before an upgrade, ends the way back to a time before it.
- 16 section 12 names the limit of the automatic states of the status page: they are as fresh
  as the last round of `status.probe` and stay while no worker runs it.
