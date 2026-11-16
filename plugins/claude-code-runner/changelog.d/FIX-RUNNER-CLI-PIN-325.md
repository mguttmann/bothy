### Changed

- The pinned Claude Code version is 2.1.325 (was 2.1.321). A runner with 2.1.325 reports
  `online` and `check-config` prints `runtime ok`; every other version, 2.1.321 included,
  reports `runtime_version_untested`. The recordings of the CLI come from 2.1.325. The stream
  mapping, the recognised error texts and the detection of account errors are unchanged:
  2.1.325 removes and renames nothing the runner reads and adds only members it does not read,
  among them `api_error_type` of an API error.
