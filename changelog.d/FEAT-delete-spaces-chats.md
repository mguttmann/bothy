### Added

- Deleting in the apps (FEAT-delete-spaces-chats, 01 6.18, R-PR-33): on macOS, Windows, iOS and
  Android the menus of spaces, channels, project forums and chats and the "More" button of a
  history end with "Delete space...", "Delete channel...", "Delete project..." or "Delete chat...",
  shown only to whoever may delete the item. The confirmation names how many channels, chats,
  messages, files and agent sessions go. For a project an owner can also move its folder to the
  trash of the runner's host (off by default); the box is disabled with the runner's reason when
  the folder cannot be moved, and after the undo window the app says whether the runner moved or
  kept the folder. A "Deleted <name>" snack bar offers "Undo" for a short time (at most
  `ui.delete_undo_toast`, default 10 seconds).
- On iPhone and Android a long press on a sidebar or post row opens its menu; dragging a row
  still works.

### Changed

- A deleted space, channel or chat leaves the sidebar of every device at once, its local content
  is removed, and an open history closes with "This chat was deleted."; an undo brings it back in
  its old place and its history loads again from the server. A deleted item never comes back
  through a later sync, and a deleted chat's unread count no longer stays on its channel.
