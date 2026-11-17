### Changed

- The pinned Claude Code version is 2.1.326 (was 2.1.325); the recordings of the CLI come from
  2.1.326. 2.1.326 writes no `turn_duration` after a turn that ended with an API error, so a
  background turn now ends at its API error record.
- With `runner.launch_mode = "background"` a run never uses print mode. Every cause that used
  print mode before ends the run `failed` with the cause in the thread. Hosted agents of class
  `host` and turns with channel skills run as background sessions with their plugin folders,
  settings, MCP file and tool flags; the runner holds the cost cap of a hosted agent itself.
  An `organization` run (`background_api_key`), a full context (`background_compaction`) and
  a hosted agent below `host` (`background_restricted`, no inbox socket under `--restricted`)
  end `failed` with their cause.
