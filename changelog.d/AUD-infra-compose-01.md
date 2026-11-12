### Added

- Push can be switched on in `.env` of a Compose installation: `BOTHY_PUSH_APNS_TEAM_ID`,
  `BOTHY_PUSH_APNS_KEY_ID` and `BOTHY_PUSH_APNS_TOPICS` for APNs, `BOTHY_PUSH_FCM_PROJECT_ID` for
  FCM, with the key files `apns_key.p8` and `fcm_service_account.json` in `secrets/`. The
  installation guide has a section on push with the command that checks it. Empty values leave
  push off, as before.

### Fixed

- An installation without the virus scan delivers uploads again. With `--scan off`, which is
  also the default of `install.sh` below 4 GiB of Docker memory, the server still waited for a
  virus scanner that did not run, and every upload stayed pending. The new variable
  `BOTHY_SCAN_MODE` sets `file.scan.mode`; `install.sh` writes `off` together with leaving the
  profile `scan` out and says that uploads are then delivered unscanned. An installation set up
  by hand without the profile `scan` sets `BOTHY_SCAN_MODE=off` in `.env`.
- The upgrade guide says that the procedure replaces only the images and leaves the files of an
  installation bundle as they are; the installation guide no longer promises the Grafana panel
  plugin of a new release through an upgrade.
