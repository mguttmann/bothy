### Added

- Configuration key `runner.cli_telemetry` (default `false`). While it is `false`, every Claude
  Code CLI the runner starts gets `DISABLE_TELEMETRY`, `DISABLE_ERROR_REPORTING` and
  `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`, so the CLI sends no usage metrics, no error reports
  and no other non-essential traffic to its vendor; `true` sets none of them (09 18.2).
