### Fixed

- Background mode: a notification that an agent of the session (a subagent or a workflow
  agent it launched) took mid-turn no longer counts as another sender's input. The CLI writes
  the removal of such an input into the session's transcript and the attachment that
  attributes it only into the agent's own file, so the next turn of the session stopped the
  run with `background_foreign_input` and the thread's next message started a new session
  without context. The runner now reads the agent files of the session for that attachment and
  judges it by the same rule; an input of another sender that an agent took, and an input
  nothing attributes, still stop the session.
