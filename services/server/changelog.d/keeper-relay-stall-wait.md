### Fixed

- `TestRelay_FullBufferHoldsTheHostBack` waits for the relay's stall timer by its duration
  (`fakehost.Timers.StartedSince`) instead of a count of pending timers, which stayed unchanged
  when the deadline of the opening stopped meanwhile.
