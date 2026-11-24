### Changed

- The collector container of the Compose stack has a memory limit, set with the new variable
  `BOTHY_OTELCOL_MEMORY_LIMIT` (default `1g`); its `memory_limiter` percentage now counts against
  that limit instead of the memory of the whole host (AUD-PKG-P10-repo-ops-host).
- The example edge of the multi-node overlay is sized for `perf.ws_connections_per_node` on both
  nodes (`worker_connections`, `worker_rlimit_nofile`, the container's `nofile` limit) and keeps
  idle connections to the nodes for REST requests (AUD-PKG-P10-repo-ops-host).

### Fixed

- A promoted macOS build starts its phased rollout at the promotion: `appdist.py promote-appcast`
  sets the `pubDate` of the entry to that time (AUD-PKG-P10-repo-ops-host).
- A Windows release (app and host) stops before signing when `windows.timestamp_url` of the
  distribution is empty, so no release is signed without a timestamp (AUD-PKG-P10-repo-ops-host).
- The summary and runbook of `BothyMalwareScanFailing` say that it counts scans that end with
  `error`, not uploads that stay pending (AUD-PKG-P10-repo-ops-host).
