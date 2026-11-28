### Added

- Metrics `bothy_webhook_delivery_total` (outcome) and `bothy_webhook_transition_total` (cause)
  and the alert `BothyWebhookSubscriptionStopped` for subscriptions the system paused or
  disabled; an unmatched cost reconciliation entry is logged with its run and reason.
- Queue `files_text` (`jobs.queue.files_text.max_workers`, default 2): transcription and text
  extraction run there and no longer delay the malware scan of new uploads.
- Table `status_component_probe` (migration `00128`): `status.probe` keeps the run of failing
  checks in the database, so with several workers a flapping check no longer turns degraded too
  early.

### Changed

- `installApp`, `upgradeAppManifest` and `validateManifest` refuse `delivery.mode: gateway` for an
  app with events, or with commands or components without `interactions_url`; existing
  installations are unaffected.
- The default of `file.scan.max_concurrent` is 2, the workers of the queue `files`.
- Backup manifests are `backup_manifest.v2`: they name every table but count only `event_log` and
  `audit_events` exactly; manifests of earlier runs stay readable for the restore test and
  `bothy restore finalize --relocation`. An earlier `bothy-backupd` does not read a v2 manifest.
- `bothy-backupd` retries a failed backup run twice, 15 minutes apart, and after a restart a run
  the stop canceled or interrupted; times missed while it was down are still not caught up.
- The file storage port offers only what the server uses: fs multipart, the unused presign
  operation and the unused session transition check are removed.

### Fixed

- Pending webhook deliveries end as dead with `event_expired` when the app is uninstalled or their
  event left the event log; the retention job disables such subscriptions with cause `gap`.
- `files.thumbnail` no longer waits for a free image slot under its job timeout; it is snoozed for
  a few seconds without using up an attempt.
- A stored file digest of the wrong length is an internal error instead of being read as zeros.
- The s3 object mirror retries failed requests and no longer stops at the first failing object; a
  run that could not copy some objects fails with `s3_mirror_objects_failed_<n>`, and the next run
  tries them again.
