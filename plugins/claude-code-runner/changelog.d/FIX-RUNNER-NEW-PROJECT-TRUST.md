### Fixed

- A project folder newly bound to a forum runs in background mode without a step on the host:
  before a background launch in the folder of the forum's binding below
  `runner.projects_root`, the runner records the workspace trust of that folder in the Claude
  Code configuration of the host user, under Claude Code's own lock of that file. Before, every
  run in such a folder failed with `background_workspace_untrusted` until someone ran `claude`
  there once. `runner.default_workspace` and the folder of chat files still need that step;
  the log line `workspace_trust_failed` names why the runner could not record the trust.
