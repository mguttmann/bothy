# Module `apps`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Apps and manifests, installations, scopes, outgoing webhooks with consumer and delivery, incoming webhooks.

## Port (root package `internal/apps`)

- Inbound: `Apps`, `Webhooks`, `IncomingWebhooks`.
- Emits in-tx events: none.
- Subscribes to in-tx events: `chat.ChannelArchived`.
- Implements source ports and cross-cutting ports: none.

## Dependencies

May use the ports of these modules: `instance`, `identity`, `chat`, `agents`, `messaging`, `sync`.

Reason: webhook signing key; app principal and tokens; target of the incoming webhooks; commands of apps; messages from webhooks; consumer for outgoing webhooks.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`apps`, `app_installations`, `webhook_subscriptions`, `webhook_deliveries`, `incoming_webhooks`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`app.read`, `app.install`, `app.scopes.update`, `app.suspend`, `app.resume`, `app.uninstall`, `webhook_out.read`, `webhook_out.manage`, `webhook_out.redeliver`, `webhook_out.deliver`, `webhook_in.read`, `webhook_in.create`, `webhook_in.revoke`, `webhook_in.post`.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `apps.webhook_deliver` | consumer `webhooks`, `webhook_out.redeliver` | `delivery` |

## Notes for the build

- Apps never run in the server process; they act only with a token, scopes and grants (B6).
- Outgoing webhooks only to `https` targets from `webhook.allowed_hosts` (R-DM-17); before every delivery, `webhook_out.deliver`.
- Messages from incoming webhooks have `origin = app` and never trigger agents.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [11-extensions.md](../../../../docs/spec/11-extensions.md)
