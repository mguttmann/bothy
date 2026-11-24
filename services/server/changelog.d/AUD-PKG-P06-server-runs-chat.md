### Fixed

- `channel.created`, `channel.updated`, the `current` of a 412, the snapshot and view-as carry
  `allow_autonomous_runs` like `getChannel`, also when it is false.
- Restoring a deleted space or channel no longer leaves two live siblings with the same sort key
  when one was created at its place during the undo window; a later move between them no longer
  fails with an internal error.
- Deleting a draft, also by sending it or in the erasure of an account, blanks the text of its
  stored `draft.updated` events.
- `getAgent`, `listAgents` and the presence of agents another api node fed show a stored online
  or degraded state whose heartbeat is stale as offline, as the status page does.
- `agent.presence_changed` is published after the tracker's entry lock is released, so a slow
  hub no longer holds presence reads and heartbeats.
- A stored `operator_config` with a key a later release wrote no longer turns agent lists into
  internal errors after a rollback.
- `Threads.Locate` reports a deleted thread as deleted.
- The two sort orders of the thread list read an index (migrations 00120 and 00121) instead of
  sorting every thread of the forum.
- `PATCH /api/v1/messages/{message_id}` of a message with attachments accepts an empty content
  (removing the caption), as sending it does; without attachments an empty content stays
  `400 validation_failed` with `empty`.
- Search, the mention list and the channel export no longer log and count a hit of a deleted chat
  as `filter_mismatch` (an error line and `bothy.authz.errors`) while its messages wait for the
  purge; such a hit is dropped silently.
- `POST /api/v1/search/assist`: an earlier question in `history` longer than
  `search.query_max_chars` is `400 validation_failed` with `too_long` on `/history/<i>/q` instead
  of `500`; a failing second search with the assistant's terms keeps the picks and the hits of the
  first search instead of answering an error.
- `agent.invocation` of a run started again with `retryRun` by another person names that person in
  `triggered_by`, as the run, `run.state_changed` and the token do.
- `createRunWake` and `recoverRunWake` refuse a `not_before` later than `run.wake_max_wait` after
  now (except for `usage_limit`) with `400 validation_failed` on `/not_before`; such a wake-up could
  only expire.
- `createRunWake`, `recoverRunWake` and `fireRunWake` without a body answer `400 validation_failed`
  like the other run operations instead of failing inside the handler.
- The `createThread` answer of the message handlers uses the shared thread conversion, so it can no
  longer miss a member such as `deleted_at`.
- Index on `runs.triggering_interaction_id` (migration `00122`): the answer of an interaction and
  the deletion of an interaction no longer read the whole `runs` table.

### Changed

- The error lines of the agent presence tracker carry `error.type`.
- A page of `listThreadMessages`, the search hits, the context window, the mention list and the
  channel export read the mentions of all their messages in one statement instead of one per
  message, and `listThreadMessages` reads the thread and channel of its page once for all
  message checks.
- `agent.followup` is declared in the event registry of the runs module like the other events it
  emits.
