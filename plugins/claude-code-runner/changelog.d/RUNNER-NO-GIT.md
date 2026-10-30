### Added

- Configuration key `runner.default_workspace` (default empty, off): the folder of a forum the
  runner operates without a project binding, `"."` for `runner.projects_root` or a folder name
  under it. Such a forum is then a normal Claude Code chat (09 18.1). A guest run in a channel
  the runner does not operate never uses it and ends with the cause `channel_not_served`.

### Changed

- Git is optional: a bound folder without `.git` is ready; the readiness reason
  `not_a_git_worktree` is no longer sent (09 18.1, R-AG-35).

### Fixed

- A run the runner ends without a turn, or with `adapter_error`, names its cause in the thread
  (the `summary` of `finishRun`), for example a forum without a project folder, a missing or
  unwritable folder, a busy session or a refused answer, instead of a bare "Error in the
  adapter" (09 18.9).
