### Fixed

- `install` no longer switches a parked profile back on: a platform section with
  `enabled: false` keeps it, and the summary says
  `platform enabled: no (kept off; install --enable switches it on)`. A section without the key
  (a first install) is switched on as before, and the new `--enable` switches a parked profile on.
