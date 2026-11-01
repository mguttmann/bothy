### Added

- Spaces, channels, forums and chats can be deleted and, until `chat.delete_undo_window`, restored:
  `deleteSpace`, `restoreSpace`, `getSpaceDeletion` and the same for channels and threads. A
  deleted space or channel is archived in the same rights change; `space.archived`,
  `channel.archived` and `thread.updated` carry `deleted_at`, a restore publishes the update
  without it. A deleted chat leaves every list, and its history and new messages answer 404.
- The job `messaging.purge_deleted` removes the content of a deletion after the undo window
  through the message deletion path, clears thread titles and channel topics and sets
  `purged_at`; the rows stay.
- A deleted forum can ask its runner to move the project folder to the trash: the server keeps
  the runner's folder check from the heartbeat, hands the move over in `HeartbeatAck.project_trash`
  once the deletion is final, and stores and audits the outcome from `project_trash_results`.
- New configuration keys `chat.delete_undo_window` and `chat.delete_purge_interval`.
