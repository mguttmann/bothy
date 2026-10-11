# Modul `apps`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Apps und Manifeste, Installationen, Scopes, ausgehende Webhooks mit Verbraucher und Zustellung, Eingangs-Webhooks.

## Port (Wurzelpaket `internal/apps`)

- Eingehend: `Apps`, `Webhooks`, `IncomingWebhooks`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: `chat.ChannelArchived`.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `identity`, `chat`, `agents`, `messaging`, `sync`.

Grund: Webhook-Signierschlüssel; App-Prinzipal und Tokens; Ziel der Eingangs-Webhooks; Befehle von Apps; Nachrichten aus Webhooks; Verbraucher für ausgehende Webhooks.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`apps`, `app_installations`, `webhook_subscriptions`, `webhook_deliveries`, `incoming_webhooks`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`app.read`, `app.install`, `app.scopes.update`, `app.suspend`, `app.resume`, `app.uninstall`, `webhook_out.read`, `webhook_out.manage`, `webhook_out.redeliver`, `webhook_out.deliver`, `webhook_in.read`, `webhook_in.create`, `webhook_in.revoke`, `webhook_in.post`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: keine.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `apps.webhook_deliver` | Verbraucher `webhooks`, `webhook_out.redeliver` | `delivery` |

## Hinweise für den Bau

- Apps laufen nie im Server-Prozess; sie handeln nur mit Token, Scopes und Freigaben (B6).
- Ausgehende Webhooks nur an `https`-Ziele aus `webhook.allowed_hosts` (R-DM-17); vor jeder Zustellung `webhook_out.deliver`.
- Nachrichten aus Eingangs-Webhooks haben `origin = app` und lösen nie Agenten aus.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [11-erweiterungen.md](../../../../docs/spec/11-erweiterungen.md)
