### Fixed

- A push through APNs whose request was cancelled while the provider token was due no longer
  pauses the environment as a configuration error; it is a transient error, as with FCM.
- The lease of a webhook delivery attempt and the job timeout of `apps.webhook_deliver` are now
  twice `webhook.delivery_timeout`, so a receiver that hangs until the send limit counts as a
  failed attempt with backoff instead of being claimed again by the lease wake-up.
- `bothy restore finalize` ends every webhook delivery still pending from the old timeline as
  dead with `event_expired`, so webhooks catch up nothing from before the restore.
- `terminal.sweep` no longer stops at the first session whose end fails; it sweeps the remaining
  sessions and reports the failed ones together.
- A terminal signal after a commit (`terminal.host_close`, `terminal.session_open`) waits for the
  outermost commit of the request and is never delivered after its rollback, also with an
  `Idempotency-Key` on a single node.
- A terminal opening or attach that its recheck ends no longer keeps its working directory index
  in memory.

### Changed

- The notify module README, its code comments and chapter 13 describe the push and gateway paths
  as built; unused notify code is removed.
- The terminal module README names its parts and extension points as built and has one retention
  section owner.
- A contract test holds the member sets and enums of the app reply decoder and the members of the
  app manifest equal to `docs/api/openapi.yaml`.
