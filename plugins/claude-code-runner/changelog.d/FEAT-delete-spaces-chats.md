### Added

- Moving a project folder to the trash (FEAT-delete-spaces-chats, 09 18.15): every
  `runner.trash_check_interval` the runner checks each bound folder it serves and reports
  `trash_check` in the readiness entry (refusal codes `outside_root`, `symlink`,
  `workspace_missing`, `contains_other_project`, `foreign_worktree`, `uncommitted_changes`,
  `unpushed_commits`, `check_failed`). For a request in `HeartbeatAck.project_trash` it stops a
  running turn of the forum, checks again and moves the folder with one rename into the trash of
  its host (`~/.Trash`, the freedesktop trash with a `.trashinfo`, a `.bothy-trash` folder under the projects root on Windows),
  or into `.bothy-trash` under the projects root when that trash is on another file system or
  not writable; it never copies or removes. The outcome goes out in `project_trash_results`;
  `<runner.state_dir>/trash.json` keeps handled requests, so a restart neither repeats a move nor
  loses a result. Log codes `project_trash_moved`, `project_trash_refused`,
  `project_trash_deferred`, `project_trash_failed`.
- Configuration key `runner.trash_check_interval` (default `60s`, above zero); the runner also
  sends `sendHeartbeat` at this interval to receive folder moves.
