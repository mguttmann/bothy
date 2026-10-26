# Fixture of chapter 02

The rows of the real table (English layout), for TestThresholdsFromChapterTable.

## 22. Configuration values of this chapter

| Name | Place | Default | Meaning |
|---|---|---|---|
| `sync.log_poll_interval` | Server | `1s` | poll of the log tail |
| `perf.reference_profile` | Load test | `4 vCPU, 8 GiB RAM, SSD, PostgreSQL 18 local with fsync and synchronous_commit on, role all` | environment of the load tests; measurement modes in ADR 0063 |
| `perf.sustained_messages_per_s` | Load test | `200` | sustained load for `perf.message_create_p99`, `perf.invocation_delivery_p99` and `perf.run_patch_fanout_p99`; also the load of `perf.push_handoff_p95` |
| `perf.message_create_p99` | Load test | `150ms` | target |
| `perf.event_commit_min_per_s` | Load test | `1000` | target (minimum) |
| `perf.fanout_local_p99` | Load test | `200ms` | target |
| `perf.fanout_cross_node_p99` | Load test | `500ms` | target |
| `perf.invocation_delivery_p99` | Load test | `500ms` | target |
| `perf.run_patch_fanout_p99` | Load test | `500ms` | target |
| `perf.catchup_page_p99` | Load test | `250ms` | target |
| `perf.authorize_cached_p99` | Load test | `100us` | target |
| `perf.ws_connections_per_node` | Load test | `5000` | target (minimum) |
| `perf.ws_idle_conn_mem_max` | Load test | `64KiB` | target (maximum) |
| `perf.idle_rss_max` | Load test | `256MiB` | target (maximum) |
| `perf.startup_ready_max` | Load test | `5s` | target (maximum) |
| `perf.push_handoff_p95` | Load test | `2s` | target |
| `perf.capacity_steady_messages_per_s` | Load test | `300` | offered load of the steady run of the capacity gate (M7-18): the scenario of the capacity load test at this rate |
| `perf.capacity_min_messages_per_s` | Load test | `240` | target (minimum): messages per second answered within `perf.message_create_p99` in the steady run of the capacity gate (M7-18) |

## Sources

| `perf.outside` | Load test | `1` | target |
