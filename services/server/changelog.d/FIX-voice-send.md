### Fixed

- A voice message recorded in the Windows app is accepted again: an MP4 of a generic brand
  (`mp42`, `isom`) that the uploader declared as MP4 audio is stored as `audio/mp4`, waits for its
  duration like every voice recording and then passes the voice check, instead of being refused
  as `voice_format_unsupported` (12 6.1, addendum of 2026-10-31). A declared-audio file with a
  video track is still no voice recording.
- Recordings from macOS and iOS (`audio/x-m4a`) are served inline for playback like `audio/mp4`
  (12 9.3).
