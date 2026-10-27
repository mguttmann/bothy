### Changed

- CI runs `services/server/test/agents` in four parts instead of two, one `TestConformance`
  group each: a part of two groups ran about 1200 s and crossed half of its `-timeout`, which
  the time budget check of the shards fails. The stale timings of the two old parts are gone.
