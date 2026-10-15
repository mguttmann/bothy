---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team B (API)
informed: alle Teams
---

# Webhooks nach Standard Webhooks

## Context and Problem Statement

Apps laufen nie im Server-Prozess (B6). Wer kein Gateway hält, erfährt Ereignisse über
ausgehende Webhooks. Empfänger müssen Echtheit, Frische und Doppel prüfen können; der Server darf
dabei keine fremden Geheimnisse im Klartext halten (03, Grundsatz 9). Welches Format und welche
Signatur nutzen wir?

## Decision Drivers

* Empfänger prüfen mit vorhandenen Bibliotheken statt eigenem Code.
* Kein gespeichertes Geheimnis je Abo in der Datenbank.
* Schlüsselwechsel ohne Ausfall.
* Gleiche Ereignis-Form wie Gateway und `GET /events`.

## Considered Options

* Standard Webhooks: `webhook-id`, `webhook-timestamp`, `webhook-signature`; Signatur `v1a`
  (Ed25519) immer, `v1` (HMAC) je Abo zuschaltbar.
* Eigenes HMAC-Kopfzeilenformat nach dem Vorbild großer Plattformen
  (`X-Signature-256: sha256=...`).
* Körper als JWS (signiertes JSON).

## Decision Outcome

Chosen option: "Standard Webhooks mit v1a immer und v1 zuschaltbar", because es ein offener,
dokumentierter Standard mit Referenzbibliotheken ist, asymmetrische Signaturen ausdrücklich
vorsieht und mehrere Signaturen für Schlüsselwechsel erlaubt.

Festlegungen (docs/api/webhooks.md):

* `webhook-id` ist die Ereignis-ID; signiert wird `{id}.{timestamp}.{Körper}`.
* `v1a` mit dem Server-Schlüssel des Zwecks `webhook_out`; öffentliche Schlüssel unter
  `GET /api/v1/instance/webhook-keys`.
* `v1` mit einem abgeleiteten Geheimnis (`HMAC` aus Hauptschlüssel des Secret-Providers,
  Abo-ID und Generation); nichts wird gespeichert, angezeigt wird es einmal.
* Körper ist der Ereignis-Umschlag; `occurred_at` statt der empfohlenen Bezeichnung `timestamp`.
* Wiederholung mit Backoff bis `webhook.retry_max_age`; `410` schaltet ab; jeder Versuch läuft
  durch `authorize`.

### Consequences

* Good, because Empfänger für HMAC verbreitete Bibliotheken nutzen können und für Ed25519 nur die
  Standardbibliothek ihrer Sprache brauchen.
* Good, because ein abgeflossener Datenbank-Auszug weder Signaturschlüssel noch HMAC-Geheimnisse
  enthält.
* Bad, because zwei Verfahren zu testen sind (G10).
* Bad, because die Referenzbibliotheken `v1a` nach unserem Stand nicht beschreiben [vermutet];
  Empfänger mit Ed25519 prüfen von Hand nach webhooks.md.
* Neutral, because die Körperform von der Empfehlung abweicht; der Standard verlangt sie nicht.

### Confirmation

Gate G10: Testvektoren (Schlüssel, Zeitstempel, Körper, erwartete Signaturen) im Repo; eine
Zustellung im Integrationstest wird mit der Referenzbibliothek (`v1`) und mit Ed25519 aus der
Standardbibliothek (`v1a`) geprüft. Lint (G1) hält die Ereignis-Schemas unter `webhooks:`, G2 die
Empfangs-Aktionen gegen 05-aktionen.md.
Built in M2-26: the vectors are [docs/api/webhook-test-vectors.json](../api/webhook-test-vectors.json)
(webhooks.md section 2.2), reproduced byte for byte and verified with `crypto/ed25519` and the
Standard Webhooks reference library by `TestWebhookTestVectors`; a real delivery through River is
verified the same way by `TestWebhookDeliveryG10` (`services/server/internal/apps/app`).

## Pros and Cons of the Options

### Standard Webhooks

* Good, because offen (Apache-2.0), mit Bibliotheken in vielen Sprachen.
* Good, because Rotation über mehrere Signaturen in einer Kopfzeile.
* Bad, because junger Standard; die Asymmetrie ist in den Bibliotheken schwächer belegt.

### Eigenes HMAC-Format

* Good, because sehr einfach.
* Bad, because je Abo ein gespeichertes Geheimnis; kein Standard, jede Plattform anders.

### Körper als JWS

* Good, because asymmetrisch, Schlüssel per JWKS.
* Bad, because der Körper ist kein lesbares JSON mehr; Werkzeuge der Empfänger erwarten JSON;
  Replay-Schutz muss eigens gebaut werden.

## More Information

* Spezifikation: https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md
  (abgerufen 2026-10-10) [belegt].
* Bibliotheken: https://github.com/standard-webhooks/standard-webhooks (abgerufen 2026-10-10);
  Unterstützung von `v1a` dort nicht beschrieben [vermutet].
