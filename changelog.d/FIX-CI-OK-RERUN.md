### Fixed

- After "Re-run failed jobs" of `ci.yml` the required check `ci-ok` is the conclusion of that
  attempt: `ci-ok` of a later attempt creates its own completed check run instead of updating
  the pending mark of the first attempt, which GitHub kept as a newer, failed `ci-ok`.
