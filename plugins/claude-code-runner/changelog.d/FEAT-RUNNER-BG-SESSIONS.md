### Added

- Background mode, off by default: with `runner.launch_mode = "background"` a turn of the
  runner's own agent in billing mode `personal` runs as a Claude Code background session
  (`claude --bg`, a research preview of Claude Code) instead of a `claude -p` process, in every
  channel or only in those of `runner.background_channels`. The live box, the answer and the
  costs come from the session transcript and the session list (read every
  `runner.background_status_interval`), follow-ups and the retry after an account error go to
  the session's inbox socket, a stop is `claude stop`, and a new runner process adopts a
  background turn of a killed one with every right. `organization` turns, hosted agents, turns
  that load a channel skill folder (`runner.channel_skills_dir`) and the compaction of a full
  context stay in print mode. The mode runs only on the pinned CLI
  version; a run it is not available for uses print mode and says why in its live box
  ("Background sessions are not available (`<code>`); this run uses print mode."), and for a
  cause of the host the heartbeat reports `degraded` with `background_unavailable`. Without
  the key nothing changes. Before switching it on, read the README section "Background
  sessions": the bound folders need workspace trust, tool class `host` needs the acceptance of
  bypass mode, and the runner launches only under a supervisor one of its own launches started.
