### Changed

- The `Adapter` constructor builds the heartbeat once and throws `RangeError` when a configured
  name or version breaks its forms; before, the fault showed only as a heartbeat that never went
  out.
- A failed gateway connect, a repeated gateway close, a failed poll, heartbeat or ack and an
  outbox entry that stays pending are logged as `warn` instead of `info`, which the default
  logger drops.
- The README lists the parts of the Python SDK that the TypeScript SDK does not have
  ("Differences from the Python SDK"), names where the gateway bounds frame size and buffered
  events, and `AgentHandler.onEvent` says that the delivery loop waits for it.

### Fixed

- A call to the server that fails while the adapter handles an event, for example the end of a
  waiting run on a cancel without an answer, no longer ends `Adapter.run()`: the event stays
  unsettled and the server delivers it again.
- The REST client clears the timer of every attempt once the attempt is over, reads the body of
  an answer within `sdk.http_timeout` (a stalled body ends as `TimeoutError`, not as an untyped
  error in the caller), and an abort by the caller also ends the wait between two attempts.
- An agent heartbeat that cannot be built is logged as an error instead of failing silently;
  the gateway still sends its connection heartbeat and the polling source keeps polling.
- `InvocationVerifier` keeps a JWKS answer without `max-age` for `agent.jwks_max_age` instead of
  fetching the key list again on every token.
