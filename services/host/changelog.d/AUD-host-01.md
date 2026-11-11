### Added

- On macOS and Windows the host service keeps its log: `bothy-host run` writes every log line
  also to `bothy-host.log` in the state directory, and at its size bound the file moves to
  `bothy-host.log.1` and a new one starts (AUD-HOST-01/2, /3). Before, launchd dropped the lines
  and a Windows service had no place for them; Linux keeps them in the journal as before.

### Fixed

- `bothy-host install`, `install --owner-account` and `session-user add` over a running
  installation on macOS load the service again: after `launchctl bootout` they wait until launchd
  no longer knows the label before `launchctl bootstrap` (AUD-HOST-01/1). Before, `bootstrap`
  failed with exit code 5 while the old job still ended, the command stopped and the service
  stayed unloaded until someone loaded it by hand.
- The READMEs and 16 4.4 say that a release build gives a terminal only in the owner account
  mode until a recorded platform run proves the peer check of a platform (AUD-HOST-01/4).
