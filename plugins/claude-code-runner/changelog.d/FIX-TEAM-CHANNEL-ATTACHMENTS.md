### Fixed

- A team run stores the attachments of the context messages its session has not seen as local
  files with their paths, like the trigger's: a member that got the owner's screenshot as a
  context input reads it in its next run. A file the server refuses is named as unavailable with
  the problem code, and the runner logs the code only (`attachment_unavailable`).
