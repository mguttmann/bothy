### Fixed

- An update of the macOS or iOS app keeps the person signed in when the keychain refuses one of
  the old stored items. The update into the app's own place read one item that the earlier build
  only wrote, the keychain refused it, and the app treated all old secrets as lost and asked for a
  new enrollment, also at every later start. The update now reads only what the earlier build
  read, leaves a refused item where it is and tries again at the next start, and an installation
  that already went through the failed update gets its old session back.
