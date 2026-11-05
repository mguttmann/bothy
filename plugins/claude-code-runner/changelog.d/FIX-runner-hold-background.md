### Fixed

- Background work started by a turn is no longer stopped right after the answer when the turn
  resumed a session whose previous process had lost background work. Claude Code first ran a
  short turn of its own for each stopped task, and the runner took its empty result for the end
  of the turn and closed stdin, so Claude Code ended the new work after ten minutes and the next
  turn started the same way again. The runner now waits while the message is still queued in
  Claude Code (`command_lifecycle`) and keeps stdin open for the work the turn starts.
