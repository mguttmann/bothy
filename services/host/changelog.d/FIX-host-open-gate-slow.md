### Fixed

- Opening a terminal session on a loaded macOS host no longer times out in the gate: one run of
  the checks reads the user database once (one `dscacheutil` per run instead of one per lookup)
  and every group ID once, the gate run ends with the opening window and then refuses with
  `open_timeout` without reporting or ending anything, and the LaunchAgent of the owner account
  mode runs with `ProcessType` `Adaptive` instead of `Background` (FIX-HOGS/1 to FIX-HOGS/3).
  An installed LaunchAgent keeps its value until `bothy-host install --owner-account` writes it
  again.
