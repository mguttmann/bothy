# Dashboards

Status: draft M0. Six Grafana dashboards (seven since M7-25, the seventh uses the panel plugin of
section 5) as files under
[infra/observability/grafana/dashboards/](../../infra/observability/grafana/dashboards/), loaded through
file provisioning (https://grafana.com/docs/grafana/latest/administration/provisioning/,
retrieved 2026-10-10) [verified].

## 1. Overview

| File | UID | Title | Content |
|---|---|---|---|
| `server-overview.json` | `bothy-server-overview` | Bothy / Server overview | REST success and latency, processes per role, gateway connections, messages by origin, authorization, rate limits, error logs |
| `delivery-sync.json` | `bothy-delivery-sync` | Bothy / Delivery and sync | Delivery latency, fan-out lag, events per type, waiting at the sequence counter, connecting, catch-up, `resync_required`, close codes, worker consumers (lag and age of the oldest pending event per `bothy_consumer_name`) |
| `agent-runs.json` | `bothy-agent-runs` | Bothy / Agent runs | Active runs, start latency, turn duration, failure reasons, steps, tokens, costs per currency (estimate), consumer backlog, heartbeat, rejections (I2), mode mismatches (heartbeat and I3) |
| `push.json` | `bothy-push` | Bothy / Push | Acceptance by the provider, fetch rate, sends by result, send duration, skips |
| `database.json` | `bothy-database` | Bothy / Database and backup | Connections, size, deadlocks, commits and rollbacks, query duration, pool, largest tables, age of the backups, WAL archive, restore test, free space |
| `slo-overview.json` | `bothy-slo-overview` | Bothy / SLO overview | Targets, attainment over 30 days, remaining error budget, burn rate, active alerts |
| `bothy-live.json` | `bothy-live` | Bothy / Live | five panels of the plugin `bothy-live-panel` (section 5): live topology per agent, backend view, delivery and sync, SLO traffic light, timeline; variable `env` only |

All dashboards have the variable `env` (from `target_info`, label
`deployment_environment_name`); `agent-runs` also has `agent`. Data sources with the fixed UIDs
`prometheus`, `loki`, `tempo`, `alertmanager` from
[datasources.yaml](../../infra/observability/grafana/provisioning/datasources/datasources.yaml).

## 2. Checked

Locally on 2026-10-10 [verified]:

1. All PromQL expressions of the dashboards (after replacing the Grafana variables) with
   `promtool check rules` from Prometheus v3.15.0: valid.
2. Grafana 13.2.3 started with the provisioning of this repository: four data sources and six
   dashboards loaded, `provisioned = true`, no errors in the log.
3. Metric names after the Prometheus translation (see [signals.md](signals.md) 1).

Not checked: presentation with real data. The portability test does this as soon as the server
delivers telemetry.

## 3. Maintenance

1. Dashboards are code. The provider sets `allowUiUpdates: false` and `disableDeletion: true`.
2. Changing: edit in a local Grafana, export the JSON ("Export for sharing externally"
   off), replace the file, normalize the formatting with `jq`, commit.
3. Every new query uses only metrics from [signals.md](signals.md); the naming test
   ([17-observability.md](../spec/17-observability.md) 10) checks this.
4. Grafonnet is not used: the library calls itself experimental and not intended for
   production use (https://github.com/grafana/grafonnet, retrieved 2026-10-10)
   [verified].
5. Runtime texts (titles, descriptions) are English (R-OB-09).

## 4. Other backends

The dashboards use PromQL and LogQL. With New Relic, Datadog, Azure Monitor or Sentry, their own
views apply; this file provides the list of questions that every backend must answer
(section 1, column Content). A ticket per backend can rebuild them there; the product
ships only the Grafana version.

## 5. Panel plugin "Bothy Live"

With M7 the own panel plugin `bothy-live-panel` from `plugins/grafana-bothy-live` is added
([25-visualization.md](../spec/25-visualization.md), section 8):

1. It shows the views of the apps (live topology per agent, backend view, delivery and sync,
   SLO traffic light), as far as metrics support them, in the colors and fonts of the design tokens.
   Data comes only through the data sources of this repository (`prometheus`, `tempo`).
2. A seventh dashboard `bothy-live.json` (UID `bothy-live`, title "Bothy / Live") uses the panel;
   it follows the rules from section 3. The six dashboards from section 1 stay unchanged.
3. Queries are in `plugins/grafana-bothy-live/queries.yaml`; the naming test
   ([17-observability.md](../spec/17-observability.md) 10) checks them like the dashboards.
4. Until V1 the plugin is unsigned and is provisioned locally; Grafana loads it only because
   `allow_loading_unsigned_plugins` names exactly this ID (ruling T11). Signing later after
   the owner's decision.
5. Embedding dashboards in the web console is optional and off by default; in apps it never
   exists ([25-visualization.md](../spec/25-visualization.md), section 9).

As built (M7-25): `bothy-live.json` has one panel per view (option `view`), each with the
targets of its view in `plugins/grafana-bothy-live/queries.yaml`; a unit test of the plugin
holds them equal, and `make signal-check` checks `queries.yaml` and the dashboard against the
catalogue. The provider of `infra/observability/grafana/provisioning/dashboards/` loads it with
the other six, but Grafana renders its panels only once the plugin build is installed and
allowed unsigned; that provisioning in the Compose stack is M7-26. TV mode is Grafana's kiosk
mode: `/d/bothy-live?kiosk` (verified for Grafana 13.2,
[25-visualization.md](../spec/25-visualization.md) section 8.2).

As built (M7-26): the product bundle carries the plugin build, `compose.yaml` mounts it and
allows exactly `bothy-live-panel` unsigned, `make ops-check` holds that setting, and the
portability test proves VZ-T13 through Grafana's API
([25-visualization.md](../spec/25-visualization.md) section 8.3 "As built"; operator steps in
[install-compose.md](../operations/install-compose.md) section 5.4).
