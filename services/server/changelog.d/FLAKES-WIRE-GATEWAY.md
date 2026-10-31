### Fixed

- `TestGatewayMetrics_QueueDroppedAt4014` (gateway WebSocket adapter) no longer fails with
  "dropped 2, want 3" on a loaded runner: it sends the heartbeats that fill the outbound queue
  only once the blocked writer holds the first `heartbeat_ack`, so the count no longer depends on
  whether the writer took that frame before or after the overflow. Test only, no product change.
