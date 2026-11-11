### Fixed

- The entry scan of HD11 no longer starts a program per entry on macOS: it reads the owner and
  the type of an entry without the extended ACL, which it never judged. Before, the scan of a
  large home started `/bin/ls` once per entry, and the scan of one million entries did not end
  within an hour (AUD-MISC-MISC-01/2).
- Platform suite: the person of TM-T24 has a name Windows accepts for a local account; the
  longer name made `net user` refuse the account, so the test failed on Windows before it checked
  anything (AUD-MISC-MISC-01/3).
- The README says that no platform has a recorded passing run of `host-platform.yml` and what a
  Linux machine needs above `/opt/bothy-host/bin` (AUD-MISC-MISC-01/1, /4).
