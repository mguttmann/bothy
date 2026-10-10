---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung
---

# Audit-Log hash-verkettet und getrennt von Telemetrie

## Context and Problem Statement

Sicherheitsrelevante Entscheidungen müssen nachweisbar sein: Rechte-Änderungen, Step-up,
Verwaltungsaktionen, Ablehnungen, Agenten-Aufrufe mit Backend-Richtlinie, Bestätigungen von
Freigabe-Karten. Telemetrie dient dem Betrieb, hat andere Empfänger, andere Fristen, darf Daten
verwerfen (fail-open) und wird oft bei externen Anbietern gespeichert (Baseline B11). Ein Nachweis
muss dagegen vollständig sein und Änderungen erkennen lassen.

Wo und wie wird das Audit-Log geführt, damit es vollständig, prüfbar und datensparsam ist?

## Decision Drivers

- Vollständigkeit: kein Eintrag darf still verloren gehen.
- Erkennbarkeit von Änderungen und Löschungen.
- Getrennt von Telemetrie: eigene Frist, eigene Rechte, kein externer Empfänger nötig.
- Datenschutz: keine Inhalte, Pseudonymisierung ohne Bruch der Prüfbarkeit.
- Prüfbarkeit durch den Owner ohne Spezialwerkzeug (`audit.verify`).

## Considered Options

1. Eigene Tabelle in PostgreSQL, nur anhängend, Hash-Kette über eine kanonische Form jeder Zeile,
   signierte Prüfpunkte, die außerhalb der Datenbank abgelegt werden.
2. Audit als Teil der Telemetrie bzw. der Logs (OpenTelemetry-Logs im Backend des Betreibers).
3. Nur anhängende Tabelle ohne Hash-Kette, geschützt allein durch Datenbankrechte.
4. Externes unveränderliches Ziel als Hauptspeicher (Objektspeicher mit Aufbewahrungssperre oder
   Transparenz-Log).

## Decision Outcome

Gewählte Option: 1, weil der Eintrag in derselben Transaktion wie die Änderung entsteht
(Vollständigkeit), die Kette jede Änderung an einer Zeile erkennbar macht und die signierten,
ausgelagerten Prüfpunkte auch ein vollständiges Neuschreiben der Kette erkennbar machen, ohne dass
die Instanz von einem externen Dienst abhängt.

Festlegungen:

- Tabelle `audit_events` aus [../spec/03-datenmodell.md](../spec/03-datenmodell.md): `seq` aus der
  Zähler-Zeile `audit_seq` in derselben Transaktion; `prev_hash` und `hash =
  SHA-256(prev_hash || canonical_json(Zeile ohne hash))`.
- `canonical_json` ist das JSON Canonicalization Scheme nach RFC 8785
  (https://www.rfc-editor.org/info/rfc8785, abgerufen 2026-10-10 [belegt]); damit rechnen Server und
  Prüfwerkzeug byte-gleich.
- Rechte: Die Anwendungsrolle darf nur `INSERT` und `SELECT`; `UPDATE` und `DELETE` sind per
  `REVOKE` und Trigger gesperrt. Kürzen am Fristende nur über eine Wartungsrolle mit Anker-Eintrag
  (03).
- Prüfpunkte: Alle `audit.checkpoint_interval` signiert der Worker `{instance_id, seq, hash,
  occurred_at, previous_checkpoint_hash}` mit einem eigenen Server-Schlüssel (Zweck
  `audit_checkpoint`, 03 `server_signing_keys`) und legt ihn in der Datenbank (Tabelle
  `audit_checkpoints`, 03) und am Ziel `audit.checkpoint_sink` ab. Der Owner wählt ein Ziel außerhalb
  der Reichweite der Datenbank-Verwaltung (z. B. Objektspeicher mit Aufbewahrungssperre in einem
  anderen Konto).
- `audit.verify` rechnet die Kette ab dem letzten Anker nach und vergleicht mit den Prüfpunkten
  vom Ziel, nicht aus der Datenbank.
- Inhalt: nur IDs, Aktion, Entscheidung, Grund-Code, Ressource, `details` nach Schema
  `audit_details.v1` ([../spec/19-sicherheit.md](../spec/19-sicherheit.md), SEC-AUD-5); nie Inhalte,
  Namen, E-Mails oder Geheimnisse. Einziger Freitext ist der Pflichtgrund einer Verwaltungsaktion
  (`reason`, z. B. bei I4), weil die Aktion ihn verlangt und er zur Nachvollziehbarkeit gehört.
  IP-Adressen nur als HMAC mit vernichtbarem Schlüssel je Zeitraum
  ([../spec/20-datenschutz.md](../spec/20-datenschutz.md), Abschnitt 4.5).
- Telemetrie bekommt höchstens Zähler (z. B. Ablehnungen je Aktion), nie Audit-Zeilen.
- Gleiche Ablehnungen desselben Prinzipals werden je `audit.deny_aggregate_window` zusammengefasst
  (R-AZ-04).

### Consequences

- Gut, weil kein Audit-Eintrag fehlen kann, ohne dass die Änderung selbst fehlt.
- Gut, weil Pseudonymisierung (Prinzipal-Zeile ändern, IP-Schlüssel vernichten) keine Audit-Zeile
  ändert und die Kette gültig bleibt.
- Gut, weil die Instanz ohne externen Dienst funktioniert; das Ziel der Prüfpunkte ist ersetzbar.
- Schlecht, weil die Zähler-Zeile `audit_seq` alle auditierten Transaktionen am Ende serialisiert
  (wie `event_seq`); Lasttest nötig.
- Schlecht, weil wer Datenbank und Ziel der Prüfpunkte zugleich kontrolliert, beides fälschen kann.
  Das bleibt Restrisiko (T-DB-01 im Bedrohungsmodell).

### Confirmation

- Test: Änderung einer Audit-Zeile mit der Wartungsrolle wird von `audit.verify` erkannt.
- Test: Neuschreiben der ganzen Kette wird am Vergleich mit dem Prüfpunkt erkannt.
- Test: Anwendungsrolle kann nicht `UPDATE` oder `DELETE` (Rechte-Test gegen die Migration).
- Test: Kürzen mit Anker, danach `audit.verify` erfolgreich.
- Test: Pseudonymisierung eines Kontos, danach `audit.verify` erfolgreich (20, DP-T02).
- Kanarien-Test: keine Inhalte und Geheimnisse in `details` (19, SEC-T10).

## Pros and Cons of the Options

### 1. Tabelle mit Hash-Kette und ausgelagerten Prüfpunkten

- Gut, weil transaktional vollständig und prüfbar.
- Gut, weil ohne externen Dienst lauffähig.
- Neutral, weil ein zusätzlicher Schlüssel und ein Ablageziel zu betreiben sind.
- Schlecht, weil Serialisierung am Zähler.

### 2. Audit in Telemetrie oder Logs

- Gut, weil die Infrastruktur ohnehin da ist.
- Schlecht, weil Telemetrie Daten verwerfen darf und muss (fail-open), Audit nicht.
- Schlecht, weil externe Telemetrie-Anbieter dann Nachweise mit personenbezogenen Daten halten und
  Fristen nicht zusammenpassen.
- Schlecht, weil Änderungen nicht erkennbar sind.

### 3. Tabelle ohne Kette

- Gut, weil am einfachsten.
- Schlecht, weil jede Person mit Datenbankrechten Einträge unbemerkt ändern oder löschen kann.

### 4. Externes unveränderliches Ziel als Hauptspeicher

- Gut, weil Änderungen technisch ausgeschlossen sind.
- Schlecht, weil jede auditierte Transaktion von einem externen Dienst abhinge (Ausfall, Latenz)
  oder Einträge zeitversetzt und damit nicht transaktional geschrieben würden.
- Neutral: als Ablageziel der Prüfpunkte genau richtig; das ist Teil von Option 1.

## More Information

- Datenmodell: [../spec/03-datenmodell.md](../spec/03-datenmodell.md), `audit_events`.
- Aktionen mit Audit-Pflicht: [../spec/05-aktionen.md](../spec/05-aktionen.md), Spalte "Audit".
- Kontrollen und Konfigurationswerte der Prüfpunkte: [../spec/19-sicherheit.md](../spec/19-sicherheit.md),
  Abschnitt 12.
- Kern-Vertrag: `server_signing_keys.purpose` mit `audit_checkpoint` und Tabelle
  `audit_checkpoints` stehen in [../spec/03-datenmodell.md](../spec/03-datenmodell.md),
  Abschnitt 4.12.
