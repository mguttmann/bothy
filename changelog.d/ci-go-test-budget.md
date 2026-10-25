### Changed

- CI fails a Go test package that runs longer than half of its time limit, naming the package
  and its time, instead of waiting for the limit to kill it with a dump of whichever test ran
  last. The limit of each Go test binary rises from 20 to 40 minutes, so the budget (20 minutes)
  ends a slow package before the former effective limit (22 minutes) and the hard limit only
  catches hangs.
- The server test shards are balanced by fresh durations, `internal/wire` runs in four parts and
  `test/agents` in two (its conformance scenarios now spread over four top-level tests), and a
  shard with an end-to-end package of the terminal suite builds the suite's images once before
  its tests instead of inside each package.
