### Fixed

- With several api nodes and an edge without hash routing, `bothy.terminal.relay.unpaired` now
  counts both lone sides of an opening that ended at `terminal.open_timeout`: a host data
  connection that the host service closed on the `session.close` of that end, before its node
  learned the end, was not counted.
