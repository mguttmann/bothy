### Fixed

- `TestLiveBoxQueueFullGivesSnapshot` no longer fails with "failed to read frame payload:
  unexpected EOF" on a slow runner. The reader stops reading for a whole round of patches, so the
  server's write waits as long as the round takes; a `gateway.write_timeout` of 5 minutes ended the
  connection in the middle of round 2 on a loaded runner. The test now sets the write deadline, the
  heartbeat deadline and the presence timeout beyond any run of the package (24 hours), since it
  checks none of them.
