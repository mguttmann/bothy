### Changed

- `internal/wire` wires each module in its own file `module_<name>.go`; `serve.go` iterates the
  module wirings. The start steps, their order and the behaviour of `bothy serve` are unchanged.
