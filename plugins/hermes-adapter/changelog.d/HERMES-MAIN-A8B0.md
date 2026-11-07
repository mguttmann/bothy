### Changed

- The pin moves from Hermes Agent `v0.22.5` to the main commit `a8b0bc15053c` (no release tag,
  D-HERMES-MAIN-1), recorded again, with the stubs and the module list of that commit; Renovate
  keeps proposing the next release tag for it.
- The adapter works on `v0.22.5` and on `a8b0bc15053c`; its runtime code is the same for both,
  so a Hermes installation can switch before or after the adapter.
- A bind of a session that ran in another thread of the same gateway process still fails open
  with a new session and `bind_failed` on both versions: `a8b0bc15053c` would move the session,
  but the other thread's next `/new` ends it there (D-HERMES-MAIN-4).
