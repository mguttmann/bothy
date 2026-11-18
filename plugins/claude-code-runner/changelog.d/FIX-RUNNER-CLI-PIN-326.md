### Changed

- The pinned Claude Code version is 2.1.326 (was 2.1.325); the recordings of the CLI come from
  2.1.326. 2.1.326 writes no `turn_duration` after a turn that ended with an API error, so a
  background turn now ends at its API error record.
- With `runner.launch_mode = "background"` a run never uses print mode. Every cause that used
  print mode before ends the run `failed` with the cause in the thread. Turns with channel
  skills run as background sessions with their plugin folders. An `organization` run
  (`background_api_key`), a full context (`background_compaction`) and every run of a hosted
  agent (`background_restricted` below `host`, `background_hosted_bypass` for `host`: the
  session drops the permission bypass without the user settings) end `failed` with their cause.
- 2.1.326 refuses a background launch whose folder of chat files (`--add-dir`) has no
  workspace trust; such a run ends `failed` with `background_files_untrusted`. Trust
  `<runner.state_dir>/files` once as the runner's host user.
- The example configuration and the README name `claude-opus-5-5[1m]` as the model.
