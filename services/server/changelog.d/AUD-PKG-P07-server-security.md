### Fixed

- The reason of `createGrant`, `previewGrant` and `revokeGrant` is limited to
  `audit.reason_max_chars` characters and answers `400 validation_failed` with `too_long` on
  `/reason` (SEC-AUD-5).
- The audit entry of `grant.revoke` names subject and target of the revoked grant, so the
  permission history of a channel, space or person can be read from the audit log.
- The bound of the principal of the check without a request (R-AZ-29) holds for `Scope`,
  `AdminScope` and `EffectiveRights` as well as for `Authorize`.
- "View as person" with another owner as the target shows that owner's view instead of only
  their explicit grants.
- "Who has access?" marks a trigger grant blocked by SR3 (project-bound channel without a ready
  own dev environment) as `guard_blocked` with rule `sr3`, as the permission hints do.
- The audit entry of an authorization error is written even when the request was canceled, and
  its failure log line names the principal kind.
- A longer `privacy.audit_ip_key_period` no longer leaves the audit log without an IP key: the
  new key starts where the last expired key ended, instead of failing every audited write with
  a client IP until the new period ends.
- The warning `audit_checkpoint_sink_not_configured` also says that `retention.audit` trims
  nothing without a checkpoint sink; configuration reference, privacy chapter and ROPA template
  name the condition.
- The reason of a privacy request and of its decision is limited to `audit.reason_max_chars`
  characters instead of a fixed 1000.
- A privacy export or deletion whose last attempt ended by the job timeout or a worker stop is
  still marked `failed` instead of staying `running`.
- An export package whose request did not take it (failed completion, or the request ended
  meanwhile) is deleted at once, also when the job's context has ended.
- An enrollment code redeemed on a device that is already known and pending is refused with
  `409 conflict` and reason `device_pending` and stays usable, instead of being spent while the
  device stays pending.
- `identity.unlink` no longer counts the keyless web device of a console login as a device that
  can approve a pairing, so it cannot remove the last usable login method.
- The flush steps of the database phase in the shutdown (`audit_denials`, `stats_flush`) get
  their own budget, the new key `shutdown.database_flush_timeout`, which the wait for cancelled
  jobs no longer takes; before, a hanging job could spend the whole total and the open denial
  aggregations were neither written nor reported as dropped.
- The incoming webhook routes of the internal listener answer `503 unavailable` with
  `Retry-After` once the shutdown began, instead of writing while the jobs and the pool stop and
  failing with an internal error after the pool closed.
- The roles `worker` and `all` check the login of `bothy_maint` (`migrate.maint_password_ref`) at
  the start; a missing or wrong password ends the start instead of failing every run of the audit
  trim later.
- The gateway writes one info line `gateway connection closed` per connection with the close
  code, the outcome, the principal kind and the duration (new log attribute
  `bothy.connection.duration`), so the disconnects of a device can be followed in the log.
- The challenge cleanup keeps the used passkey challenge of a live session's current step-up, so
  a console session no longer loses its user-verified step-up (and gets `passkey_required` at the
  terminal) when the job runs between the step-up and the opening.
- The deletion of an account rewrites the stored `user.updated` events of the person to the
  pseudonymized handle and name, so the old handle and display name no longer stay in the event
  log until `sync.event_retention`.

### Changed

- Backup and restore guide: after a restore to an earlier point in time the checkpoint sink is
  moved to a new location with the checkpoints of the restored database, so the chain check does
  not stay broken; the sink must never lose objects. The alert runbook of
  `BothyAuditChainBroken` covers each failure reason and the restore case.
