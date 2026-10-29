### Added

- The FakeServer answers `getChannel` (Allgemein as a text channel with its main thread, a channel
  of `add_forum` as a forum) and `createThread` (a forum post with its first message), lists
  commands in every served channel, and `trigger(command=..., options=...)` and
  `invoke_command(..., options=...)` deliver a command's name and option values.
