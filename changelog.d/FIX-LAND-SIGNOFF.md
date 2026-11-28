### Added

- `tools/hooks/land_pr.py` lands a pull request: it squash-merges with the title plus the pull
  request number and the checked body without the template's HTML comments, the maintainer's no-reply address as author
  and the checked head commit, and reads the commit on `main` back.

### Changed

- `make commitlint` requires the sign-off of the pull request body in one line, as the only
  `Signed-off-by` line of the body and with a GitHub no-reply address.
