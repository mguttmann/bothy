### Changed

- Contract tests: G7 draws its cases from a fixed seed (`quality.contract.seed`, `CONTRACT_SEED`
  of `make contract-test`), in the owner and user passes and now also in the stateful phase, so a
  failure repeats on a rerun instead of passing with another seed (issue #238). The Go test no
  longer draws a seed of its own and fails without one.
