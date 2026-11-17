### Changed

- Background mode: an input that the runner did not write (a line of another program on the
  inbox socket, something a person typed into the session, a notification that is not the
  session's own work, an input nothing attributes) no longer ends the run. The runner logs the
  warning `background_foreign_input` with a fixed word for whose input it was and does nothing
  else: the session is not stopped, the answer is posted, and the next message of the thread
  continues the same session. The cause `background_foreign_input`, the mark
  `<runner.state_dir>/sessions/<session id>/foreign-input`, the log lines
  `session_not_continued`, `session_mark_failed` and `background_foreign_input_unread`, and
  the replacement of a marked session by a new one are gone; a mark an earlier version wrote
  has no effect and goes with the retention sweep. On the owner's host every input the earlier
  rule stopped was the owner's own or a notification of Claude Code itself (an agent of the
  session, a plugin), and each stop cost the thread its conversation. This replaces the
  stopping and the "never continued" rule of the entries FIX-RUNNER-BG-INBOX-AUTH and
  FIX-RUNNER-BG-SIDE-NOTICE above.
