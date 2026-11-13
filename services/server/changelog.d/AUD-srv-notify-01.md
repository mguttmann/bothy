### Changed

- `push.max_consecutive_failures` has the default `0`, and `0` means that transient failures
  never deactivate a push registration. Before, the default `5` ended push for a device after
  one short outage of the provider or of the network, and nothing enabled the registration
  again until the app registered a new token or the person logged in anew. A push that fails
  for such a reason is retried with the backoff until `push.max_age`; only an answer of the
  provider about the token disables a registration. An instance that sets the key above `0`
  keeps the former rule.

### Fixed

- A push registration that was disabled while a push to it was on its way (the session ended,
  the device was revoked, a newer token replaced it) keeps its reason and its time. Before, a
  late answer of the provider could replace `logout` with `token_invalid` or
  `repeated_failures`, move `disabled_at` and schedule another attempt.
- The consumer `push` reads one event for the previous read position of a person in a thread.
  Before, it loaded every retained read of every person in that thread for each read it handled.
- The badge number no longer costs up to three reads per unread thread: the place of the threads
  is read once per channel, and the checks of one count share one rights version.
