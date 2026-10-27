### Changed

- CI runs `services/server/test/terminal` in three parts instead of two, so
  `TestHostPairAgainstServer` (400 to 720 s on the runners) runs alone: in a part with the other
  tests the part ran up to 936 s, close to half of its `-timeout`, which the time budget check
  of the shards fails.
