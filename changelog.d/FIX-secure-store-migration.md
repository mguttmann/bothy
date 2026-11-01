### Fixed

- Updating the app no longer signs it out. A macOS build that keeps its secrets in the login
  keychain started on the connect screen after the update that moved the secrets into the
  app's own place, because that keychain cannot list its items. The secrets of the earlier
  build now move per instance on every platform, the device keeps its device key, and the old
  items are removed only after the first successful refresh with them. Another app identity
  still never takes them.
