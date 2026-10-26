### Fixed

- The test harness runs no round of `status.probe` behind a test by default
  (`harness.Options.StatusProbe` turns it on): a round appended `status.updated` to the event
  log while tests counted the events after a seq.
