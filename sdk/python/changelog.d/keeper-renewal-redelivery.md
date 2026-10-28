### Fixed

- A run no longer stays `queued` when the server redelivers its start event while the adapter
  renews an expired token: the renewal grant used to overwrite the newer redelivered token in
  the inbox, `startRun` answered `superseded` with the grant and no further delivery came. The
  adapter now starts with the most recently received token (09 4.5.1);
  `InboxStore.replace_token` takes `replaces` (replace only while the stored token is still the
  renewed one) and returns the entry. Seen as a hang of the hermes-adapter test
  `test_start_answers_expired_then_renewal` in CI.
