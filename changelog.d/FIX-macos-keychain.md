### Fixed

- The macOS app built without a team signature (ad-hoc, the local build) connects again: when
  the Data Protection keychain is not available to the app, the device keys and the app secrets
  go to the login keychain without an access group, and the keys are reported to the server as
  not hardware-bound.
- A found instance whose local setup fails on the device no longer shows "No instance at this
  address": E1 names the keychain or the local data as the cause, and the app log has the error.
