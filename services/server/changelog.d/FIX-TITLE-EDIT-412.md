### Fixed

- Editing the title of a thread no longer fails with `412 version_conflict` when messages arrived
  since the client last received the thread (FIX-TITLE-EDIT-412, 06 section 8 item 2, R-API-32).
  Every message raises the version of its thread without a thread event, so a client that follows
  the event stream never held the newest version of an active chat, and the exact match refused its
  first edit. `setThreadTitle` now accepts an `If-Match` that names any version of the thread since
  its title or title source last changed; a version from before that change is still
  `412 version_conflict` with the current thread. Migration 00103 adds `threads.title_version`
  (default 1 for existing threads, no backfill, no table rewrite).
