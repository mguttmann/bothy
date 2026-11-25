### Fixed

- Background mode: a tool call that would wait for a person's approval no longer stops the
  whole session. Claude Code asks for approval even in bypass mode when its check of a removal
  cannot parse a command with certainty (for example an edit through a heredoc that holds
  `rm -f "$$f"`); the session then waited, the runner stopped it, and every background agent
  of the session ended with it. Every settings file of a background session now sets
  `permissions.denyInsteadOfPrompting`, so Claude Code refuses such a call at once, the model
  reads the refusal and the turn goes on. When the runner still stops a session that waits
  for a person, its log has the warning `background_session_stopped` with the cause and what
  the session waited for.
