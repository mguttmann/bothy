### Fixed

- `bothy-host install` and `session-user add` on macOS pick an ID for a new account that no
  existing group has: the account gets one ID as its UniqueID and as the PrimaryGroupID of its
  own group, dscl refuses a PrimaryGroupID that another group has, and on every Mac GID 400 (the
  first ID install tried for the service user) belongs to a system group, so install failed with
  `run dscl .: exit code 55`.
- The platform suite registers its uninstall before install, so a failed install no longer
  leaves accounts and services behind for the later tests.
