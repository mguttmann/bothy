### Fixed

- The apps keep the chats a person uses in their offline cache. Having a chat on screen now
  counts as its use, so the cleanup at the start of an app no longer removes the messages of a
  chat once the device has stored it for longer than `client.cache_max_age`, however often it
  was read; such chats were missing in the sidebar and offline until a new message arrived.
- An open chat nobody opened for `client.cache_max_age` stays listed in the sidebar and is
  still found by its title; only its messages and files leave the device and load again when
  the chat is opened.
- When an app reloads everything from the instance (after a long time offline, or after an
  update that asks for it), the files it had cached from a channel the person can no longer
  see are deleted at once, not only by the cleanup by age.

### Changed

- Storing a message no longer reads the whole offline search index, and the lookups of the
  local database that read a whole table (by channel, by chat and by the ID a device gave its
  own message) use an index. The local database moves to schema version 16; the first start
  after the update fills the new lookup table once and keeps the offline data.
- An open chat no longer redraws for messages that arrive in other chats.
