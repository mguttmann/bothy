### Changed

- CI runs `services/server/test/agents` in four parts instead of two, one `TestConformance`
  group each: a part of two groups ran about 1200 s and crossed half of its `-timeout`, which
  the time budget check of the shards fails.
- `go_shards.py timings` weighs a top-level test by the longest of its own and its subtests'
  durations: parallel subtests run after their parent returned, so the parent alone read as a
  few seconds and the conformance groups were split by chance. `go_test_timings.json` is
  refreshed from a green main run, which also gives the tests of `test/terminal` their weights
  (its first part ran 1246 s on a pull request while its tests went round-robin by name).
