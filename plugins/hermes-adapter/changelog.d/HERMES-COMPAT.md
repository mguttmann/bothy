### Changed

- The adapter supports the current Hermes Agent release line: the pin moves from `v2026.9.24`
  to `v0.22.5`, recorded again, with the stubs, the toolset table and the module list of that
  version.
- The adapter runs on Python 3.11 and newer (before: 3.12), the oldest Python Hermes admits;
  its own tests run on 3.11, the Fake-Hermes tests and the recording on the Python of the
  pinned Hermes environment (3.14 for `v0.22.5`).
- The recording accepts exactly one listener of Hermes, its local runtime API on a loopback
  port, and keeps the Unix sockets of a scenario home within the 100 bytes Hermes `v0.22`
  binds; a new scenario records the upstream declaration next to the own access policy.

### Fixed

- `--hermes-version=latest` resolves the newest release again after Hermes changed its tag
  scheme from dates (`v2026.9.24`) to versions (`v0.22.5`); release candidates and canaries
  never count.
- With Hermes `v0.22` a message of an organization profile reached no turn: the adapter now
  declares `authorization_is_upstream` while Hermes keeps no allow list of its own for the
  platform, so the token check and the server's authorization stay the access control and the
  allow list of a personal profile still refuses every other person.
- A bound session (`/resume`) no longer leaves its first turn without an end: the adapter waits
  until Hermes has finished processing a command before it sends the next message.
- A bind of a session that already ran in another thread of the same gateway process fails open
  with a new session and `bind_failed`: Hermes `v0.22` does not move such a session to another
  thread, and the bound turn stalled.
