### Fixed

- A turn that fails while background work runs no longer leaves the chat without an answer.
  When the login of Claude Code on the host was briefly gone, a failed turn kept the run open
  for the session's background work, and the owner's next messages went unanswered until that
  work ended. The runner now stops such a process shortly after the failed turn, ends the run
  with a note in the thread that says the login was lost and how many background tasks were
  stopped and lost, and the next message starts a new run that continues the same session.
