# demo-agent-resilience Demo agent survives transient server errors

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-demo-agent-resilience-1 | 2026-10-27 | Transient errors of the demo agent are `429`, `502`, `503`, `504` and a request without an HTTP answer. `receive`, `heartbeat`, `connect` with its first heartbeat and the watcher's patches wait them out with a backoff from 0.5 s doubling up to 30 s, reset after a success. Worker and reviewer scripts are not retried: a repeated `start`, `post` or `finish` is not idempotent, and a failed script still ends only its own run. | A single `503` of `receive` ended every agent of the demo; the long-lived loops are the parts that must outlive an outage. |
| D-demo-agent-resilience-2 | 2026-10-27 | `serve` runs every agent on its own: a non-transient error of one agent is logged without content and ends only that agent. The `demo-agent` service has `restart: unless-stopped`, which brings the process back when it ends. | One agent must never end the others; the restart policy covers what the process itself does not. |
