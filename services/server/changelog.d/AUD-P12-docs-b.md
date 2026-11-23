### Fixed

- The API contract closes the envelope `{op, d}` of every gateway frame that a client sends, as
  the server already refuses a further member with 4002; the route generator refuses an open
  envelope.
