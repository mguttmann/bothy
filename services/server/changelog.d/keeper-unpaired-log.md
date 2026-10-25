### Fixed

- The relay logs `terminal relay side unpaired` for every side it counts in
  `bothy.terminal.relay.unpaired`: a lone side whose opening ended at `terminal.open_timeout`
  on another node, learned through the log tail before the side's own timer, was counted
  without the record.
