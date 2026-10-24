### Fixed

- A message sent while the owner moves its thread no longer fails with `500 internal_error`, and
  the push consumer no longer fails a round for a message of a thread moved after it was posted:
  the permission check of a resource in a thread uses the thread's current channel instead of
  refusing the channel the caller read before the move.
