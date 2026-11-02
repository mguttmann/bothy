### Added

- Owner account mode (ADR 0073, docs/spec/24-terminal.md 2.4): `host_service.owner_account`
  names the existing account of the instance owner; the host service and its only session user
  run as that account, with administrator rights. `bothy-host install --owner-account` sets it up
  as the account itself, without administrator rights (macOS: a LaunchAgent, Linux: a systemd user
  unit), and `bothy-host restart` restarts it as the account. The PTY helper is a child process of
  the host service; the peer check compares the account and the PID of that child.
- The report marks the owner account with `owner_account: true`, and the host service accepts a
  session token for it only with `own = true`, checked before the binding
  (`session_user_mismatch` otherwise).

### Changed

- In the owner account mode the self-checks that refuse the mode by design pass with warnings:
  HD1 `owner_account_mode`, HD2, HD3, HD9, the helper part of HD8 and the HD10 codes
  `peer_check_unavailable`, `ptrace_scope_low` and `helper_unprotected`; a subscription login in
  the owner's account is a warning of HD6. HD1 fails with `owner_account_mismatch` when the service
  runs as another account or names another session user. The separate-user setup is unchanged.
- The mode refuses `host_service.require_local_confirm = true` and is not available on Windows.
