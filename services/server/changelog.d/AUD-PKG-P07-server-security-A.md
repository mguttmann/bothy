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
