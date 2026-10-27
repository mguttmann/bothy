### Fixed

- Tests: the phone login flow test waits for the Emergency entry of the owner before it scrolls to
  "Log out", so the entry that arrives with `getMe` after the Me page shows no longer pushes the
  button out of the window and turns `apps (iOS)` red.
