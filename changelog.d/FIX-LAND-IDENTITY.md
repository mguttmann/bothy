### Changed

- `make commitlint` refuses a `Signed-off-by` or `Co-authored-by` line in the body of a bot's
  pull request and a sign-off whose no-reply address is not the pull request author's: GitHub
  makes the pull request author the author of the squash commit, and the deny list on `main`
  accepts only a sign-off that repeats it.
- `tools/hooks/land_pr.py` lands a bot's pull request without a sign-off and without an author
  address, refuses a pull request of any other account than the maintainer's, and checks after
  the merge that every sign-off of the commit on `main` repeats its author.
