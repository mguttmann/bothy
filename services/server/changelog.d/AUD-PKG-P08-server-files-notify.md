### Fixed

- The S3 driver fails a write when the source reports a read error together with the last byte,
  instead of completing the object.
- A multipart upload created for a first part is aborted at once when its id cannot be stored (a
  database error, or the file was deleted meanwhile), instead of staying open.
- An export package gets its `files` row before its object is written and becomes deliverable only
  after length and SHA-256 match; a refused package is purged through its row, and a package whose
  store ended early is removed by the cleanup after `file.failed_row_ttl`, so no object with
  personal data is left without a row that names it. When the row is gone by the time the bytes
  are written, the package deletes its object itself.
- With the S3 driver the scan pass completes the multipart upload of a confirmed file before it
  checks clamd, so a scanner outage no longer keeps the bytes in an incomplete upload that a
  storage lifecycle rule may abort.
- Reopening the session of an import file sets a failed file pending again with `file.updated`
  for the uploader, and aborts after the commit the S3 multipart upload that the old session still
  named, instead of dropping its id and leaving the upload open.
- A push through APNs whose request was cancelled while the provider token was due no longer
  pauses the environment as a configuration error; it is a transient error, as with FCM. The
  same holds for a request cancelled while the token is renewed after `ExpiredProviderToken`.
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

- Migration `00123` adds the partial index `files (deleted_at, id) WHERE deleted_at IS NOT NULL`,
  so the catch-up and the backlog gauge of `files.cleanup_pending` no longer read the whole
  `files` table on every run.
- The notify module README, its code comments and chapter 13 describe the push and gateway paths
  as built; unused notify code is removed.
- The terminal module README names its parts and extension points as built and has one retention
  section owner.
- A contract test holds the member sets and enums of the app reply decoder and the members of the
  app manifest equal to `docs/api/openapi.yaml`.
