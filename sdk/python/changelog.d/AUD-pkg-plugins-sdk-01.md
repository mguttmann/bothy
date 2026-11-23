### Fixed

- `RunHandle.finish` counts as done only once the server answered. When `finishRun` gets no
  answer, or a `5xx`, `401` or `429`, the handle stays unfinished with `finish_pending` and keeps
  the first end asked for; the adapter keeps the run in its heartbeat and sends that end again
  (`retry_finish`) with waits from `sync.reconnect_backoff_base` up to
  `sync.reconnect_backoff_cap` until the server stores or refuses it.
- An event of a known type that does not decode is passed on as `UnknownEvent`, logged with
  `WARNING` and acknowledged, instead of stopping the adapter or coming back forever.
- A failing heartbeat callback no longer ends the heartbeat: it is logged with `WARNING` and the
  last valid heartbeat goes out.
- `Adapter.run` closes the stores and the client when its start fails, and a network error while
  ending a cancelled run no longer stops the adapter.
- Live-box patches of a run carry the run's token also when a task without the run's binding
  started the throttle timer.
- The `idempotency_key` parameter of the generated operations sends the key as text; before, httpx
  refused the UUID with `TypeError`.
- `WebhookKeyCache` bounds a fetch through a generated core client with `sdk.http_timeout`, so a
  server that never answers no longer holds every webhook check.
- The User-Agent version is held equal to `VERSION` by a test.
- The guide example stops the adapter on `SIGTERM` as on Ctrl+C.
