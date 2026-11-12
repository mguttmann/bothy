### Changed

- Compose stack: `worker` builds the export packages of privacy requests in the new volume
  `export-work` (`privacy.export_work_dir`) instead of `/tmp`, which is memory in the hardened
  containers. `docker compose up -d` creates the volume; it needs the server image of this
  version, which carries the directory for it.

### Fixed

- The data inventory of 20 section 2 lists the tables `agent_definitions`, `audit_ip_keys`,
  `deletion_journal_state` and `gateway_presence`, and section 9 says what the server knows about
  the live connections of people. A test now holds every table of the module register against the
  inventory.
- `docs/operations/backup-restore.md` 5.1 and 16 section 8.2 say how far back a point-in-time
  restore reaches: with the default `backup.pg.local_retention_full` and without an off-site
  target only to the newest full backup.
- Goal 3 of 16 section 1 and RK-51 say what the restore test measures: the duration of a restore
  from the local repository, not the data loss and not the way back from the off-site target.
- 16 section 13 says how the workers of the job queues and `database.max_conns` relate.
