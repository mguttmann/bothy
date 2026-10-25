### Changed

- CI fails a Go test package that runs longer than half of `quality.test.go_timeout`, naming
  the package and its time, instead of waiting for the limit to kill it with a dump of whichever
  test ran last. `quality.test.go_timeout` doubles, so half of it is still below the former
  effective limit and the full limit only catches hangs.
- The server test shards are balanced by fresh durations, `internal/wire` runs in four parts,
  `test/agents` and `test/terminal` in two (its conformance scenarios now spread over four top-level tests), and a
  shard with an end-to-end package of the terminal suite builds the suite's images once before
  its tests instead of inside each package.
