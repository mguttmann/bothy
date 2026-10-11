# Module `stats`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Statistics for the console: usage per space, runs, tool calls, costs, error rates.

## Port (root package `internal/stats`)

- Inbound: `Stats` (queries, timeline, traffic light, backend view), `Live` (base frame per interval, filter per receiver).
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `measure.Recorder`.

## Dependencies

May use the ports of these modules: `chat`, `agents`, `messaging`, `runs`, `billing`.

Reason: reads metrics through ports.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`stats_minutes`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`stats.read`, `client_metrics.submit`.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: `stats.live`.
- WebSocket ops (client to server): `stats.subscribe`, `stats.unsubscribe` (through the hub of the gateway).

## Notes for the build

- Metrics without content; instance view only for the owner, space view with `manage` (05, `stats.read`).
- Distinction from telemetry: `stats` shows domain figures in the console, operational metrics run through OpenTelemetry (17-observability.md).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [01-product.md](../../../../docs/spec/01-product.md), [17-observability.md](../../../../docs/spec/17-observability.md)
