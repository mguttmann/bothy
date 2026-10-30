### Fixed

- A message with several attachments no longer hangs at "Uploading 0 %" when the server limits
  uploads (12 3.8, addendum of 2026-10-30): each file keeps one upload session for its whole life
  and a retry resumes it instead of announcing the file again, a refused status read is retried
  once with its upload instead of twice, every retry waits `Retry-After` plus a random share of
  the backoff so the uploads do not come back together, and the files of a conversation upload
  one at a time. An upload that waits for its next attempt shows "Paused, trying again shortly".
