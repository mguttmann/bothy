# Service Level Objectives

Status: draft M0. Decision: ADR 0035. The source of all numbers is the Sloth files under
[infra/observability/slo/](../../infra/observability/slo/); the table below reproduces them.
`make ops-check` compares table and files (check `slo-docs`).

## 1. Principles

1. An SLO measures what people feel: API responses, arrival of messages, reconnecting,
   push, start and duration of agent runs.
2. An SLI is a ratio of good events to all events, taken from histograms and counters
   ([signals.md](signals.md)). Thresholds are bucket boundaries.
3. Period: 30 days, rolling (Sloth default).
4. Error budget = 1 minus target. A spent budget means: reliability before new features,
   until the budget recovers (rule for milestone planning).
5. All targets are starting values. After 30 days of measurement in operation the owner checks them; a change
   is a change of the Sloth file with the reason in the commit.

## 2. SLOs

| SLO (`service/name`) | SLI: good | SLI: all | Threshold | Target | Alert | Metric of the own measurement |
|---|---|---|---|---|---|---|
| `bothy-api/requests-availability` | REST requests without 5xx | REST requests without `/api/v1/gateway` | none | 99.9 % | Page, Ticket | `sli.api_availability` |
| `bothy-sync/delivery-latency` | live deliveries of durable events up to the threshold | all live deliveries of durable events | 0.5 s | 99.5 % | Page, Ticket | `sli.delivery_latency` |
| `bothy-sync/reconnect` | connections with `ok` up to the threshold | connections without `client_error` | 2.5 s | 99.0 % | Page, Ticket | `sli.reconnect` |
| `bothy-push/provider-acceptance` | sends `accepted` | sends without `rejected_token` | none | 99.5 % | Page, Ticket | `sli.push_acceptance` |
| `bothy-agents/run-start` | runs `started` up to the threshold | runs without `cancelled` | 7.5 s | 99.0 % | Page, Ticket | `sli.run_start` |
| `bothy-agents/turn-duration` | completed runs up to the threshold | completed runs | 600 s | 95.0 % | Ticket only | `sli.turn_duration` |

The column "Metric of the own measurement" reproduces the label `bothy_sli_metric` of the SLO in the
Sloth file; it maps the SLO to its metric in `stats_minutes`
([25-visualization.md](../spec/25-visualization.md), section 3.3). The build ticket PLAN M6-11
sets the labels; until then this column is the requirement for them.

Rationale:

1. Availability without the gateway upgrade: the duration of an upgrade is the connection duration, not
   response behavior. The gateway measures `reconnect`.
2. Delivery only live: catch-up after a pause is measured by `bothy.gateway.catchup.duration`; its
   duration depends on the backlog of the client, not on the server.
3. Push measures the acceptance by the provider. Whether a device shows the notification, the server does not see;
   the fetch rate (`bothy.push.fetch`) is a hint, not an SLI.
4. `run-start` includes adapter and network: if an adapter is offline, the budget burns. That
   is intended; the instance operator also operates the adapter.
5. `turn-duration` depends on model and task. A page alert would usually not be fixable by the
   instance operator; therefore ticket only.

## 3. Burn rate alerts

Sloth generates multi-window alerts (https://github.com/slok/sloth, retrieved 2026-10-10) [verified],
following the method of the Google SRE Workbook (https://sre.google/workbook/alerting-on-slos/,
retrieved 2026-10-10) [verified]. The generated rules (Sloth v0.16.0) contain:

| Level | Short window | Long window | Burn rate |
|---|---|---|---|
| page | 5m | 1h | 14.4 |
| page | 30m | 6h | 6 |
| ticket | 2h | 1d | 3 |
| ticket | 6h | 3d | 1 |

An alert fires only when both windows are above the threshold.

## 4. Generate and check

After a change to a Sloth file, regenerate the rules with the pinned Sloth and run the gate:

```
sloth=$(uv run --project tools --locked --quiet python tools/bin/fetch_tools.py path sloth)
"$sloth" generate --no-log -i infra/observability/slo/api.yaml -o infra/observability/prometheus/rules/slo-api.yaml
make ops-check
```

The generated files are checked in and carry the header "DO NOT EDIT". `make ops-check`
regenerates them and compares byte for byte (check `sloth`; no difference allowed, and every
generated file needs its Sloth file), checks all rules with `promtool check rules` (check
`prometheus-rules`) and the table in section 2 against the Sloth files (check `slo-docs`: rows,
target with one decimal, with more only where one would round the target, threshold from the
`le` selector of the error query, alert according to `page_alert.disable`). Generated locally
on 2026-10-10 with Sloth v0.16.0 and checked with `promtool` from Prometheus v3.15.0: valid
[verified].

## 5. Visibility

The dashboard `bothy-slo-overview` shows target, attainment, remaining budget and burn rate from the
recording rules `slo:objective:ratio`, `slo:sli_error:ratio_rate30d`,
`slo:period_error_budget_remaining:ratio` and `slo:current_burn_rate:ratio`.

## 6. Own measurement in the apps

The SLO traffic light of the apps and the console does not compute with Prometheus but with the
server's own measurement (`stats_minutes`, [25-visualization.md](../spec/25-visualization.md),
section 3 and 6.4). A generator produces targets, thresholds and period from the same
Sloth files; a check keeps them equal. The traffic light carries the label "own measurement" and
does not alert; alerts stay with the burn rate rules from section 3.
