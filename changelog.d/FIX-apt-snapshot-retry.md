### Fixed

- The host image of the terminal test suite no longer fails the CI when `snapshot.debian.org`
  is overloaded: APT runs in a retry loop and falls back to the current Debian packages after
  three failed attempts, and a failed build names the APT errors.
