### Added

- Owner account of a host (ADR 0073): `HostReportedSessionUser`, `HostSessionUserAssign` and
  `HostSessionUser` carry `owner_account`. An assignment of the owner account needs the explicit
  opt-in `owner_account: true` equal to the report (`400 validation_failed` with
  `owner_account_opt_in_required` or `owner_account_not_on_host`), and only an active owner makes
  one, only for himself (`403 forbidden`, `reason: owner_account_owner_only`). Migration 00089
  adds `host_session_users.owner_account`.
- SR12 counts such an assignment only while the host reports the name with the same flag, and
  `terminal.open` and `terminal.attach` with it need an active owner (`403 terminal_denied`,
  `reason: owner_account_owner_only`, audit reason `terminal_owner_account_owner_only`); losing the
  owner role ends those sessions. Their allow entries, and the assignment entries, carry
  `owner_account = true`.
