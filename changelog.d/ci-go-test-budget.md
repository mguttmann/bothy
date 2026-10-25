### Changed

- CI fails a Go test package that runs longer than half of its time limit, naming the package
  and its time, instead of waiting for the limit to kill it with a dump of whichever test ran
  last. The limit of each Go test binary rises from 20 to 30 minutes, so the budget (15 minutes)
  is stricter than the former effective limit.
- The server test shards are balanced by fresh durations, `internal/wire` runs in four parts and
  `test/agents` in two, and a shard with an end-to-end package of the terminal suite builds the
  suite's images once before its tests instead of inside each package.
