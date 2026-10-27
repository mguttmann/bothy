### Fixed

- `TestServeWorkerRolesRunConsumers/all` no longer fails with a duplicate key of `event_log` seq 1
  (main run 40540821549). The test inserted its event at the fixed seq 1, past the `event_seq`
  counter, while the running server could append an event of its own at that seq. The test
  helper now takes the next seq of `event_seq` in the counter transaction, as the server does,
  and the test waits for the consumer to handle that seq.
