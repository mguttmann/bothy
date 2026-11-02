### Changed

- A terminal session in the owner account mode (ADR 0073) stays attachable for
  `terminal.owner_account_detach_grace` (default `24h`) after a detach and lives at most
  `terminal.owner_account_max_session_duration` (default `168h`); idle pauses while it is detached
  and counts again from the reattach. The hello of its data connection carries the owner detach
  value. Sessions of separate session users keep `terminal.detach_grace` and
  `terminal.max_session_duration`.
