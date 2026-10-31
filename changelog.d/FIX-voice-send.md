### Fixed

- A voice message sent while its recording is still being checked is no longer marked as failed:
  the app keeps it in the outbox and sends it again once the server has checked the file (12
  8.1, addendum of 2026-10-31). "Send again" of a refused voice message sends the same recording, also for a
  Windows recording refused before this fix.
- Voice messages recorded on macOS or iOS show the voice player.
