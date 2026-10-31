### Fixed

- Pasted screenshots no longer stay at "Being checked" in the composer and in a message that
  waits to be sent. Once an upload is confirmed, its chip follows the file's state from the server
  (`file.updated`) instead of the upload entry that has left the outbox (12 2, addendum of
  2026-10-31).
- File states are honest: "Being checked" only while the malware scan runs, "Preparing" while the
  server still prepares a checked image or recording, and "Check failed" for a scan that could not
  be completed, apart from "Blocked" for a found threat.

### Added

- A bitmap pasted from the clipboard on the desktop is sent as a JPEG when that is much smaller,
  so large screenshots upload and load quickly; the setting "Send pasted images as originals"
  (`composer.paste_originals`) keeps them as they are. Files attached as files are never changed.
