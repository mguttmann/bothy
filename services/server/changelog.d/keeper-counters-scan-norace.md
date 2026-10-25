### Changed

- The counters scans of the module (`TestCountersOnlyUsedByPhaseHooks`) run in `make arch` without
  the race detector, in their own package `test/countersscan`; under `-race` in the test shards
  they had grown to over 13 minutes and pushed `internal/wire` past its time budget.
