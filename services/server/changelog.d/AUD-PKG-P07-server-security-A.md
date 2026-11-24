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

### Changed

- Backup and restore guide: after a restore to an earlier point in time the checkpoint sink is
  moved to a new location with the checkpoints of the restored database, so the chain check does
  not stay broken; the sink must never lose objects. The alert runbook of
  `BothyAuditChainBroken` covers each failure reason and the restore case.
