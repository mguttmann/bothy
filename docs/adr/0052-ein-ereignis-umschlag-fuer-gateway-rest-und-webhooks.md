---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team B (API), Team D (Agenten-Plattform), Team A (Architektur)
informed: Team E (Apps), Team F (Betrieb)
---

# Ein Ereignis-Umschlag für Gateway, REST-Nachholen und Webhooks

## Context and Problem Statement

Ereignisse erreichen ihre Empfänger auf vier Wegen: als `dispatch` auf dem WebSocket-Gateway, über
`GET /api/v1/events` (REST-Nachholen), über den REST-Verbraucher der Agenten und als ausgehender
Webhook, auch als Transport für Agenten ([04-sync-protokoll.md](../spec/04-sync-protokoll.md),
Abschnitte 2, 7 und 15). Agenten brauchen zusätzlich Angaben, die nur für eine Zustellung gelten:
eine frisch ausgestellte Aufruf-Marke und das Kennzeichen einer erneuten Zustellung. Hat jedes
Ereignis auf jedem Weg dieselbe Form, und wo stehen die Zustellangaben?

## Decision Drivers

- SDKs, Adapter und Apps dekodieren Ereignisse mit einem Typ, gleich auf welchem Weg sie kommen
  (ADR 0026, R-API-06 in [06-api.md](../spec/06-api.md)).
- Ein Empfänger, der vom Gateway auf Webhooks wechselt, ändert seine Verarbeitung nicht.
- Aufruf-Marken entstehen je Zustellung neu und dürfen nie gespeichert werden (I3, ADR 0023).
- Felder heißen überall gleich, in snake_case (06, Abschnitt 2).
- Webhooks folgen Standard Webhooks für Kopfzeilen und Signatur (ADR 0016).

## Considered Options

- Ein Umschlag für alle Wege; Zustellangaben als eigene Felder im Umschlag, nie im Log
- Eigene Form je Weg (Gateway-Rahmen, REST-Seite, Webhook-Körper nach der Empfehlung von Standard
  Webhooks mit `type`, `timestamp`, `data`)
- CloudEvents 1.0 im JSON-Format als Umschlag
- Ein Umschlag für das Ereignis plus eine getrennte Hülle für Zustellangaben

## Decision Outcome

Gewählte Option: "Ein Umschlag für alle Wege, Zustellangaben als Felder im Umschlag", weil er eine
Form, ein Schema und eine Dekodierung für alle Empfänger gibt. Die Felder stehen normativ in
[04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitt 2.2 (R-SY-17).

1. Felder des Umschlags: `id`, `seq` (nur dauerhafte), `type`, `schema_version`, `occurred_at`,
   `actor`, `scope`, `resource`, `audience`, `payload`, `redacted`, `truncated`, `correlation_id`,
   `trace_context` (nur an Agenten-Adapter), `client_msg_id`, `permission_version` (nur
   dauerhafte). Kurzlebige Ereignisse nutzen denselben Umschlag ohne `seq`.
2. Zustellfelder: `invocation_token` (Aufruf-Marke für Ereignisse, die einen Aufruf tragen) und
   `redelivered` (erneute Zustellung nach `agent.ack_timeout`). Sie entstehen je Zustellung und
   stehen nie in `event_log` ([03-datenmodell.md](../spec/03-datenmodell.md), `event_log`). Für
   Webhooks kommt `sync_epoch` dazu (ADR 0059).
3. Auf dem Gateway ist der Umschlag der Körper von `{"op": "dispatch", "d": ...}`; in
   `GET /api/v1/events` ein Element von `events`; im Webhook der ganze Körper, Byte für Byte
   signiert. `webhook-id` ist die Ereignis-ID ([webhooks.md](../api/webhooks.md), Abschnitt 2).
4. Schema: `EventEnvelope` in `docs/api/openapi.yaml`, je Typ `allOf` aus Umschlag und Typ-Teil;
   die AsyncAPI verweist nur darauf (06, Abschnitt 11).

### Consequences

- Gut, weil ein Typ je Ereignis in allen generierten Clients reicht und Webhook-Empfänger dieselben
  Fixtures wie Gateway-Clients nutzen.
- Gut, weil Zustellfelder klar markiert sind und die Speicherregel einfach bleibt: Das Log enthält
  nie eine Marke.
- Schlecht, weil der Webhook-Körper von der Empfehlung von Standard Webhooks abweicht (`occurred_at`
  statt `timestamp`, kein `data`). Die Empfehlung ist keine Pflicht [Q1, belegt]; Kopfzeilen und
  Signatur folgen dem Standard.
- Neutral, weil das Schema Felder enthält, die für manche Empfänger nie gesetzt sind
  (Zustellfelder an Menschen); dort fehlen sie (ADR 0053).
- Neutral, weil eine Änderung am Umschlag alle Wege trifft; Änderungen sind darum nur additiv
  (04, Abschnitt 6.1).

### Confirmation

- Gate G9 ([06-api.md](../spec/06-api.md), Abschnitt 15): jeder gesendete Gateway-Rahmen passt zu
  seinem Schema.
- Gate G10: Webhook-Konformität mit Testvektoren.
- Vertragstest: Dasselbe Ereignis über Gateway, `GET /api/v1/events` und Webhook ergibt denselben
  Umschlag; Unterschiede nur in Zustellfeldern.
- Test: `event_log` hat keine Spalte und keine Nutzlast mit `invocation_token` oder `redelivered`;
  eine Zustellung an einen Menschen enthält keines der beiden Felder.
- Bau-Ticket M1-16 in [PLAN.md](../PLAN.md) (Umschläge ohne JSON-`null`, `correlation_id` gleich
  Trace-ID).

## Pros and Cons of the Options

### Ein Umschlag mit Zustellfeldern

- Gut, weil eine Form überall.
- Schlecht, weil die Grenze zwischen gespeicherten und zustellbezogenen Feldern nur durch Regel und
  Test gehalten wird, nicht durch die Form.

### Eigene Form je Weg

- Gut, weil jeder Weg seiner Konvention folgen kann.
- Schlecht, weil drei Schemas, drei Dekodierer und drei Testreihen entstehen und ein Wechsel des
  Weges Code ändert.

### CloudEvents 1.0

- Gut, weil ein verbreiteter Standard mit Pflichtfeldern `id`, `source`, `specversion`, `type`
  [Q2, belegt].
- Schlecht, weil Namen von Kontext-Attributen nur Kleinbuchstaben und Ziffern enthalten dürfen
  [Q2, belegt]; `permission_version`, `client_msg_id` und `correlation_id` müssten anders heißen
  als in API und Datenmodell.
- Schlecht, weil `seq`, `scope`, `resource` und `audience` Erweiterungen wären, die kein
  Standard-Werkzeug auswertet; der Nutzen des Standards bliebe gering.

### Getrennte Hülle für Zustellangaben

- Gut, weil die Form selbst zeigt, was gespeichert ist.
- Schlecht, weil Agenten-Verbraucher eine zweite Form bekämen und der Webhook-Körper nicht mehr
  das Ereignis wäre; Signatur und Doppelerkennung über `webhook-id` würden komplizierter.

## More Information

- Quellen (abgerufen 2026-10-10): Q1 Standard Webhooks, Abschnitt "Payload structure",
  https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md
  [belegt]; Q2 CloudEvents 1.0, Abschnitte "Naming Conventions" und "REQUIRED Attributes",
  https://github.com/cloudevents/spec/blob/main/cloudevents/spec.md [belegt].
- Verwandt: ADR 0009 (Ereignis-Log), ADR 0013 (Spec-first), ADR 0016 (Webhooks), ADR 0023
  (Aufruf-Marke), ADR 0053 (kein JSON-`null`).
