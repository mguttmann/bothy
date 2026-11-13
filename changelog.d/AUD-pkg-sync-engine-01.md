### Fixed

- An app no longer stays connected without receiving anything after it could not store a page
  of events: it closes the connection, shows that it is reconnecting and repeats the page. An
  event it can never store is passed by loading the current state from the instance once.
- A request of the sync that gets no answer ends after `client.sync.read_timeout` (new, default
  `30s`) and is retried, instead of holding every later read behind it. While an app reloads
  everything from the instance, a lost connection is noticed at once.

### Changed

- The fake server of the client tests is compared with the real server: scenario files that
  the server test records (`services/server/test/fakeconformance`) are replayed against the
  fake. The fake now advances a read position only forward and deletes an empty draft of a
  chat, as the server does.
