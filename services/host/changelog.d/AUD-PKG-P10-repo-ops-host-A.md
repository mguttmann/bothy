### Changed

- `host_service.credential_check_interval`, `entry_scan_timeout`, `auth_status_timeout` and
  `local_confirm_timeout` must be above zero; a configuration with `0s` is refused at start with
  `config_invalid` and the key, as the backoff values already were.
- The log line "server unreachable" of the control connection names the error class as a fixed
  code (`cause`), never an address.

### Fixed

- The control connection bounds one login attempt up to `hello` (dial, TLS, upgrade and the wait
  for `hello`); a server, proxy or edge that accepts and stays silent no longer keeps the host
  offline without a new attempt.
- `bothy-host pair` gives up on a server that never answers instead of waiting for ever.
- A control connection through a proxy on loopback or on an own address no longer marks the host
  as the server host.
- On Linux and macOS a second `bothy-host run` with the same state directory ends before it takes
  the admin endpoint of the running service and logs in with its keys.
- An opening whose PTY helper does not answer after the gate ends at its window with
  `open_failed` and `open_timeout` instead of hanging without a report.
