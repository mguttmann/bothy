# Telemetry backends

Status: draft M0. Every backend is an overlay of the collector ([collector.md](collector.md));
a switch is configuration, not code. All sources retrieved on 2026-10-10.

## 1. Overview

| Backend | Overlay | Signals | Authentication | Temporality | Status of the component | Open items |
|---|---|---|---|---|---|---|
| Local Grafana stack | `local` | all | none (internal network) | cumulative | `otlp_http`, `otlp_grpc` (core) | none |
| Grafana Cloud | `grafana` | all | Basic auth header | cumulative | `otlp_http` | exact format of the header and gRPC not verified |
| New Relic | `newrelic` | all | header `api-key` | delta | `otlp_http` | none |
| Datadog | `datadog` | all | API key | converted internally | exporter beta | statistics only from the sample |
| Azure Monitor | `azure-monitor` | all | Microsoft Entra (service principal or managed identity) | delta | `azure_auth` beta | GA or preview; exponential histograms |
| Sentry | `sentry` | traces, logs | header `x-sentry-auth` | no metrics | OTLP "open beta" | status of the OTLP ingestion |

## 2. Local Grafana stack

Compose profile `observability` with Prometheus, Loki, Tempo, Grafana and Alertmanager.

| Part | Assumption | Evidence | Configuration |
|---|---|---|---|
| Prometheus | OTLP receiver with `--web.enable-otlp-receiver`, path `/api/v1/otlp/v1/metrics`; delta only experimental | https://prometheus.io/docs/guides/opentelemetry/ [verified] | [prometheus.yaml](../../infra/observability/prometheus/prometheus.yaml), checked with `promtool check config` |
| Loki | `otlphttp` exporter to `http://<loki>/otlp`; `allow_structured_metadata: true` | https://grafana.com/docs/loki/latest/send-data/otel/ [verified] | [loki.yaml](../../infra/observability/loki/loki.yaml), checked with `loki -verify-config` (3.7.8) |
| Tempo | OTLP gRPC and HTTP; Tempo 3 without the block `compactor:` | Example and manifest in the Tempo repository, tag v3.1.0 [verified] | [tempo.yaml](../../infra/observability/tempo/tempo.yaml), checked with `tempo -config.verify=true` (3.1.0) |
| Grafana | File provisioning | https://grafana.com/docs/grafana/latest/administration/provisioning/ [verified] | Provisioning loaded with Grafana 13.2.3 |

Compose sets the admin password through the Grafana resolution `$__file{...}`. The variant
`GF_SECURITY_ADMIN_PASSWORD__FILE` had no effect in the local test with the Grafana binary 13.2.3
(login with `admin:admin` still worked) [verified]; whether the Docker image resolves it through
its start script is not checked [assumed]. `$__file{}` works in both cases.

The local stack is meant for one instance. It has no high availability and no
tenant separation.

## 3. Grafana Cloud

One OTLP gateway `https://otlp-gateway-<REGION>.grafana.net/otlp` for all three signals;
authentication by Basic auth from the stack tile
(https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/) [verified]. The overlay reads
the whole header value `Basic <base64(instance-id:token)>` from a file so that it works without
an extension. The dashboards of this repository run there if the data source gets the UID
`prometheus` or the UID is replaced on import.

## 4. New Relic

OTLP endpoints US `https://otlp.nr-data.net` and EU `https://otlp.eu01.nr-data.net`, header
`api-key` with the license key, http/protobuf and delta temporality recommended; limits include
string attributes up to 4095 characters and a payload under 1 MB
(https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/) [verified]. There
is no dedicated New Relic exporter in the contrib repository; the route is OTLP [verified]. The
overlay converts with `cumulative_to_delta` and truncates attributes.

## 5. Datadog

The Datadog exporter (contrib, beta for traces, metrics, logs) no longer computes APM statistics
itself; the Datadog connector is intended for that. Against "413 Request Entity Too Large",
batching in the `sending_queue` of the exporter helps
(https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/datadogexporter)
[verified]. Datadog also names its own DDOT collector as the recommended route
(https://docs.datadoghq.com/opentelemetry/setup/) [verified]; the product stays with the
standard collector so that all backends have the same base with the same privacy filters.

## 6. Azure Monitor

Native OTLP ingestion through the collector, Azure Monitor Agent or AKS add-on; the collector needs
the extension `azure_auth`, authentication only through Microsoft Entra with the role "Monitoring Metrics
Publisher" on the Data Collection Rule; endpoints depend on DCE and DCR
(https://learn.microsoft.com/en-us/azure/azure-monitor/containers/opentelemetry-protocol-ingestion)
[verified]. Application Insights expects delta temporality and exponential histograms [verified].

Consequences:

1. The overlay converts sums to delta. The explicit histograms of the server stay
   explicit; whether Azure displays them that way is open [assumed]. Exponential histograms would
   break the SLO rules for Prometheus (there, thresholds are bucket boundaries). Ticket: check against
   a real Azure environment.
2. Whether the native ingestion is generally available or a preview is contradictory in the sources
   [verified: contradictory]. The owner decides before use.
3. On Azure VMs, `managed_identity` instead of a service principal is the better route (no secret);
   the overlay shows the service principal because it works everywhere.
4. The extension `azure_auth` had a vulnerability in inbound checking in versions v0.124.0 to
   v0.150.0; outbound use was not affected
   (https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/azureauthextension)
   [verified]. The overlay uses it only outbound, and the pinned version is above that range.

## 7. Sentry

Sentry accepts OTLP traces and logs, no metrics, status "open beta"; one endpoint per signal and
the header `x-sentry-auth: sentry sentry_key=<public-key>` (https://docs.sentry.io/concepts/otlp/)
[verified]. The contrib exporter `sentry` is alpha [verified]; the overlay uses OTLP.

For app crashes, Sentry is the only target that covers all platforms of the apps
(`sentry_flutter`: Android, iOS, Linux, macOS, Web, Windows;
https://docs.sentry.io/platforms/dart/guides/flutter/) [verified]. The apps send directly, not
through the collector ([privacy.md](privacy.md) 7).

## 8. Telemetry of the agent runtimes

Claude Code can send its own OpenTelemetry data (metrics, events, traces as beta); content is
off by default; the variables must be in the process environment
(https://code.claude.com/docs/en/monitoring-usage) [verified]. An adapter may point them at the
collector of the instance. Then the following applies: metadata only; `OTEL_LOG_USER_PROMPTS`,
`OTEL_LOG_ASSISTANT_RESPONSES`, `OTEL_LOG_TOOL_DETAILS`, `OTEL_LOG_TOOL_CONTENT` and
`OTEL_LOG_RAW_API_BODIES` stay off. By default Claude Code sets attributes such as `user.email`,
`user.account_uuid`, `organization.id` and `session.id` [verified, same source]. The allowlist
filters spans and logs; for metrics, `transform/scrub_metrics` deletes these keys on the resource
and the data point (checked locally on 2026-10-10 with otelcol-contrib 0.162.0) [verified]. The
`claude_code.*` names are not in the catalogue; whoever wants to evaluate them adds them there first
([09-agents.md](../spec/09-agents.md)).
