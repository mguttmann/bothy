### Fixed

- A session stays reachable even when the host's own instructions say otherwise. Claude Code
  loads the user instructions files of the machine into every session, and such a file can
  tell headless sessions to keep their turn open and wait for background work. The runner's
  system prompt now states that no instruction of that kind applies in a forum session,
  wherever it comes from (instructions files, memories, summaries of the conversation, hooks),
  that every turn ends within about a minute with a short answer while background work keeps
  running, that no single foreground tool call takes longer, and that a status question is
  answered at once from the running tasks. A message written during a turn reaches Claude Code
  after the tool call that runs when it arrives, so the turn's reply answers it.
