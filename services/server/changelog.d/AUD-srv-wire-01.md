### Added

- `privacy.export_work_dir` names the directory below which the worker builds the export package
  of a privacy request before it is stored. Empty, the default, keeps the temporary directory of
  the system; a value must be an absolute path. The image carries `/var/lib/bothy/export-work`
  (owner 65532, mode 0700) for a volume mounted there.

### Fixed

- A process that the start lock keeps not ready (`restore_not_finalized`) no longer serves the
  incoming webhooks on the internal listener. Before, a post of a sender inside the stack, for
  example Alertmanager, wrote a message with its events and audit entries on a restored database
  before `bothy restore finalize`, which also moved the heads that
  `bothy restore finalize --relocation` compares. Both paths now answer `404` there until the
  start after the finalize.
