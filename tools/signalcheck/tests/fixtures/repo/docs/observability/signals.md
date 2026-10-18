<!-- SPDX-License-Identifier: Apache-2.0 -->
# Signal catalogue (test fixture)

## 1. Names and units

Fixture of the gate signal-check.

## 3. Resource attributes

| Attribute | Value | Source |
|---|---|---|
| `service.name` | `bothy-server` | process |
| `deployment.environment.name` | `dev`, `prod` | configuration |

Prometheus exposes these attributes as the labels of the series `target_info`.

## 4. Metrics

### 4.1 REST and gateway

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `http.server.request.duration` | Histogram | `s` | `http.route`, `http.response.status_code` | otelhttp | `http_server_request_duration_seconds` |
| `bothy.gateway.close` | Counter | `{connection}` | `bothy.close.code` | api | `bothy_gateway_close_total` |
| `bothy.event.fanout.lag` | Gauge | `s` | none | api | `bothy_event_fanout_lag_seconds` |

### 4.6a App metrics

The apps send these; every one carries `bothy.client.platform` (`ios`, `web`).

| Name in the app | Type | Unit | Further attributes | Server name and Prometheus |
|---|---|---|---|---|
| `client.frames` | Counter | `{frame}` | none | `bothy.client.frames`, `bothy_client_frames_total` |

### 4.7 Histogram bounds

| Histograms | Bounds |
|---|---|
| durations (`http.server.request.duration`) | 0.1, 0.5, 1, 2.5 |

## 5. Traces

Not read by the gate.
