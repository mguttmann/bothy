### Added

- Moving a project folder to the trash (FEAT-delete-spaces-chats, 09 18.15):
  `buildHeartbeat("online", { projectTrashResults })` sends the outcome of each folder move
  (distinct channels, `trashed` or `refused` with a reason of the form `REASON_FORM`), and a
  `projectReadiness` entry may carry `trash_check` with a reason of the same form.
- Generated types of the deletion of spaces, channels and chats.

### Fixed

- A `projectReadiness` entry keeps its optional `reason` (09 18.12) instead of being refused as
  an unknown key.
