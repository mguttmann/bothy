---
status: accepted
date: 2026-10-18
decision-makers: Maintainer
---

# In-process test seams of serve

## Context and Problem Statement

The canary test of SEC-T10 (M1-69) runs the real `internal/wire` graph in the test process and
must read everything the server exports: spans, OTLP log records and metrics, and it must log in
through the fake OIDC provider of M1-41, whose TLS certificate no system root signs. `wire.Deps`
already carries test-only collaborators (`EgressDenyAll`, `ConsumerHandlers`); `otel.Setup` had
in-memory exporters only behind an unexported option. The question is how a test injects
exporters and TLS roots without a production path that could ever use them by accident.

## Decision Drivers

* No network call and no collector in a unit or integration test (P-09, P-10).
* The production start stays unchanged: the zero value of every seam is the production behaviour.
* One composition root: the harness must not rebuild the graph of `wire.Serve`.

## Considered Options

* Exported, documented fields in `otel.Deps` (`Exporters`) and `wire.Deps` (`TelemetryExporters`,
  `OIDCRootCAs`), zero value means production.
* Configuration keys for an exporter target and a CA bundle file.
* A separate test build of `wire` behind a build tag.

## Decision Outcome

Chosen option: exported fields in `otel.Deps` and `wire.Deps`, because they keep one composition
root, need no configuration key that an operator could set, and their zero value is the production
behaviour. An injected exporter replaces the OTLP exporter of its signal entirely, so a test never
builds an OTLP exporter.

### Consequences

* Good, because the canary test reads the real outputs of the real graph without a collector.
* Good, because `cmd/bothy` never sets the fields; nothing changes for operators.
* Bad, because `wire.Deps` grows by two test-only fields; their doc comments say so, like the
  existing `EgressDenyAll`.

### Confirmation

`TestSetupInjectedExporters` (`internal/platform/otel`) holds the exporter seam;
`services/server/test/canary` and `TestTelemetrySinks` (`test/harness`) use both seams on every
`make test`.

## Pros and Cons of the Options

### Exported fields with a production zero value

* Good, because the seam is visible and typed.
* Bad, because a production struct names test needs.

### Configuration keys

* Good, because no Go interface changes.
* Bad, because an operator could point telemetry or OIDC trust somewhere unintended; a CA bundle
  key is a new security surface.

### Build-tagged test build

* Good, because production code stays untouched.
* Bad, because a second build of the graph can drift from the one that ships.
