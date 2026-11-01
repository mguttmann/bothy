# Statistics in the admin console

Status: draft M0. Rules: [17-observability.md](../spec/17-observability.md) 8. Presentation in
the console: [01-product.md](../spec/01-product.md); visualizations, own measurement and live stream:
[25-visualization.md](../spec/25-visualization.md). Permissions: `stats.read` and `cost.read.aggregate`
([05-actions.md](../spec/05-actions.md) 4.2, 4.12).

## 1. Principle

The statistics read their own tables in PostgreSQL, never a telemetry backend. Since the
addendum of 2026-10-10 this also includes `stats_minutes`, the minute aggregates of the own
measurement (section 7). Reason: The
observability stack is optional, sampled and has short retention periods; the statistics must be
the same and complete in every installation and follow the same permissions as the data.

## 2. Key figures

| Area | Key figure | Source | Grouping |
|---|---|---|---|
| Usage | Messages per day by origin (`human`, `agent`, `app`, `import`) | `messages` (without deleted ones) | Space, channel, day |
| Usage | New threads, done threads | `threads` | Space, channel, day |
| Usage | Active people (at least one message) | `messages.author_principal_id` | Space, week; only from `privacy.stats_min_group_size` |
| Usage | Storage of the files | `files.size_bytes` via `scope_channel_id` | Space |
| Runs | Runs by end state, duration (median, p95) | `runs` | Agent, space, day |
| Runs | Failure reasons | `runs.state_reason` for `failed` | Agent, day |
| Tools | Tool calls by name, share of `failed` | `run_steps` with `kind = 'tool_call'` | Agent, tool, day |
| Costs | Valid sum, share of estimates, by model | `cost_entries` without a successor | Space, agent, model, day, currency |
| Error rates | Runs `failed` per runs | `runs` | Agent, day |
| Error rates | Webhook deliveries `failed` and `dead` | `webhook_deliveries` (while within the retention period) | App, day |
| Error rates | Scans `error` and `infected` | `files.scan_status` | Day |
| Error rates | Push registrations disabled | `push_registrations.disabled_reason` | Provider |

Costs in the mode `personal` are information only, never an invoice
([03-data-model.md](../spec/03-data-model.md) 4.9); the console labels the two modes separately
and estimates as "Estimate".

Source ports (M6-05): every row is read through the `StatsSource` port of the module that owns
its table, never by `stats` itself, and only for the channels the caller may read:

| Rows | Port | Note |
|---|---|---|
| Messages by origin, active people, refused triggers | `messaging.StatsSource` | active people only per week or month, counted inside `messaging` |
| New and done threads, channel to space, thread to channel | `chat.StatsSource` | done is a thread closed in the bucket and still closed |
| Runs, failure reasons, durations, tool calls | `runs.StatsSource` | over the threads of the visible channels (runs has no `channel_id`) |
| Costs | `billing.StatsSource` | valid entries; `personal` while `billing.show_personal_estimates` is on (default) |
| Storage, scans | `files.StatsSource` | scans only for an owner on the instance |
| Webhook deliveries | `apps.StatsSource` | only for an owner on the instance |
| Push registrations | `notify.StatsSource` | only for an owner on the instance |

Limits of the sources: from `run.max_steps_persisted` on, `run_steps` merges older steps; very
long runs therefore count too few tool calls. `webhook_deliveries` is in the class `transient`
and reaches back only as far as its retention period. Deleted messages do not count.

## 3. Privacy

1. No key figure per person. Neither messages nor runs nor costs are grouped by person. The
   triggering person of a run (`triggered_by`) does not appear in the statistics.
2. The console shows counters over people (for example active people per space) below
   `privacy.stats_min_group_size` as "fewer than `privacy.stats_min_group_size`". Reason: small
   groups make individuals recognizable.
3. The cost view per thread is not statistics but part of the thread (`cost.read.thread`).
4. An export of the statistics as CSV contains only the same aggregates.

## 4. Permissions

| View | Action | Who |
|---|---|---|
| Whole instance | `stats.read` on `instance` | Owner |
| One space | `stats.read` on `space` | `manage` on the space |
| Total costs | `cost.read.aggregate` on `instance` | Owner |
| Costs of a space | `cost.read.aggregate` on `space` | `manage` on the space |

Every query filters to the channels that `authz` returns for the principal; a space manager
sees no figures from channels they may not read.

## 5. Queries and performance

1. Queries use the existing indexes (`runs (agent_id, finished_at)`,
   `cost_entries (space_id, occurred_at)`, `messages (channel_id, created_at)`).
2. Results are valid for `stats.cache_ttl`; the period is at most `stats.max_range`.
3. Daily aggregates: If a query becomes too expensive (load test), a nightly job writes rows into
   a table `stats_daily` (space, channel, agent, day, key figure, value) and the console reads
   from there. The table is not yet in the core contract; it comes as an additive migration only
   when the load test requires it (integration decision of 2026-10-10).
4. The time zone of the day boundary is `stats.time_zone` (default `UTC`, IANA name;
   [17-observability.md](../spec/17-observability.md)). `locale.default` does not set it. A
   change applies from the restart on; daily aggregates already written keep their old boundary.
5. The cache (M6-05) lives in memory per api process. Its key is the operation, the aligned
   parameters, the caller's rights version, whether the scope is instance-wide and a hash of the
   caller's channel set; an entry expires after `stats.cache_ttl`, and at most
   `stats.cache_max_entries` entries are kept (the least recently used one is evicted).
   Authorization runs before every cache read, so a revoked permission takes effect at once.

## 6. Relation to telemetry

For operational questions (latency, 5xx, connections) the console links to `telemetry.dashboard_url`,
if set. The statistics themselves never compute with telemetry values.

## 7. Own measurement and visualizations

1. The server itself counts values without a domain table (delivery latency, reconnects,
   acceptance by the push provider, availability of the API) and the rates of the live views, and
   writes them per minute to `stats_minutes` ([25-visualization.md](../spec/25-visualization.md), section 3).
   The table has no column for people; tool names are not in it.
2. New queries: `getStatsLive` (live frame), `getStatsTimeline` (minutes and runs of a
   period), `getStatsSlo` (traffic light "Own measurement", owner only), `getStatsBackends`
   (backend view per agent). All with `stats.read` and the same permission filter as section 4;
   cost fields only with `cost.read.aggregate`. In addition, the gateway subscription
   `stats.subscribe` with the ephemeral event `stats.live`. Built: `getStatsLive` and the
   subscription (M6-15); `getStatsTimeline`, `getStatsSlo` and `getStatsBackends` (M6-14,
   rulings `D-M6-14-1` to `D-M6-14-7`). The backend view counts the switches of the backend
   policy, which have no channel, for the agents it shows (`D-M6-14-3`).
3. Rows without a channel (SLI, gateway values, changes of the backend policy) are visible only
   to whoever has `stats.read` on the instance.
4. The minimum group from section 3 applies only to counters over people; the live frame, the
   timeline and `stats_minutes` contain none. Active people are still shown only by `getUsageStats`.
5. Long periods, exports and the tables from section 2 are still computed from the domain tables;
   `stats_minutes` lives only for `retention.stats_minutes`.
