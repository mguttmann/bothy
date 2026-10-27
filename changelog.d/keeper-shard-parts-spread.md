### Fixed

- CI: the Go test shards no longer put two parts of a split package (`internal/wire`,
  `test/agents`, `test/terminal`) into one shard while another shard holds none of them. The parts
  of a shard run at the same time, so three `internal/wire` parts in one shard each ran about twice
  as long and one went over the time budget (main run 40527875107).
