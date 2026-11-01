### Fixed

- The macOS app no longer stops at its start on a keychain access dialog. After the update into
  the app's own place, a build whose old secrets were written by an app with another bundle
  identifier asked the keychain for them, and the keychain showed a dialog and waited for it;
  without someone to click, the app never reached the server. The old secrets are now read
  without any dialog: an item the keychain refuses counts as not movable, the app logs that once
  and starts with enrollment, and every keychain call of the start has a time limit.
