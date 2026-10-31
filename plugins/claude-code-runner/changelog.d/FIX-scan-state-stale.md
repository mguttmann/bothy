### Fixed

- A follow-up with attachments reaches the running turn with its files. The server delivers a
  follow-up as soon as the message is stored, often while a pasted screenshot is still being
  checked; the runner now waits until every file of the follow-up is final, at most
  `runner.followup_file_wait` (default `2m`), downloads the servable ones like the attachments
  of the trigger and lists them after the text as `local file: <path>`, or as
  `unavailable: file_blocked` or `unavailable: file_not_ready`. Before, a follow-up passed on
  only its text, and one of attachments alone was dropped (09 18.5).
