### Fixed

- Team copies carry the attachments of their source: an owner's screenshot in a team channel
  reaches every member's work thread with the message, and a member's reply keeps its files in
  the mirror and in the inputs of the other members (09 3.9.2). The server relays the files to the
  copy in the new table `message_attachment_relays` (migration 00112) instead of duplicating them;
  the member's agent reads them with `file.read` on its work thread. The job `runs.team_input`
  waits for attachments that are not final yet (up to `file.invocation_hold_max`), and an owner's
  message of attachments only is copied too instead of failing the job.
