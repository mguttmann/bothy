### Added

- `GET /api/v1/search`: a hit of `kind` `file` carries the new optional members `thread_id` and
  `message_id`, the message the file is attached to and its thread. The `File` of a hit is the
  attached form without `scope_channel_id`, so a file hit named no place a client could open.
