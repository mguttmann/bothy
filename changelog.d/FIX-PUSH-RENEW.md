### Fixed

- The apps on iOS, macOS and Android check their push registration at every start and every
  return to the foreground and register again when the server no longer has it or disabled it
  after transient failures (`repeated_failures`). Before, such a device got no push until the
  person logged out, and a registration that failed was tried again only at the next start.
