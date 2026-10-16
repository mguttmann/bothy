---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team F (Betrieb), Team A (Architektur), Team B (API)
informed: Team E (Apps), Team D (Agenten-Plattform)
---

# Sync-Epoche nach Wiederherstellung

## Context and Problem Statement

Clients, Agenten-Adapter und Worker-Verbraucher holen nach mit "alles ab Nummer X" (ADR 0009). Eine
Wiederherstellung aus der Sicherung setzt die globale Sequenz auf den Stand der Sicherung zurück.
Nach den ersten Schreibvorgängen vergibt der Server Nummern, die Clients schon aus der verlorenen
Zeitlinie kennen. Ein Client mit Cursor 1000 fragt dann "alles nach 1000" und überspringt still neue
Ereignisse mit kleineren oder gleichen Nummern; sein Cache enthält zudem Daten, die es auf dem
Server nicht mehr gibt. Die Prüfung `cursor > head_seq` (R-SY-04) erkennt das nur, bis die neue
Zeitlinie den Cursor überholt. Wie erkennen alle Empfänger eine neue Zeitlinie sicher?

## Decision Drivers

- Kein stilles Überspringen von Ereignissen, auch nachdem die neue Zeitlinie alte Cursor
  überholt hat.
- Clients verwerfen nach einer Wiederherstellung ihren Server-Zustand, behalten aber ihr
  Ausgangsfach (04, Abschnitt 13).
- Ein Umzug ohne Datenverlust darf kein Neuladen aller Clients auslösen (16, Abschnitt 11).
- Server-seitige Cursor (Agenten, Webhooks, Worker) holen nichts aus der alten Zeitlinie nach.
- Ein Mechanismus für Gateway, REST und Webhooks (ADR 0052).

## Considered Options

- Epoche `instance.sync_epoch` (UUIDv7), neu nach jeder Wiederherstellung; Clients senden sie mit
  dem Cursor; Abweichung ergibt `resync_required` mit Grund `epoch_changed`
- Nur die Prüfung `cursor > head_seq` (R-SY-04)
- Nummern nach der Wiederherstellung springen lassen (Zähler weit über jeden je vergebenen Wert)
- Der Server merkt sich den Cursor jedes Geräts und erzwingt ein Neuladen je Gerät

## Decision Outcome

Gewählte Option: "Epoche `instance.sync_epoch`", weil sie eine neue Zeitlinie unabhängig von den
Nummern erkennt und nur bei einer echten Wiederherstellung wechselt. Normativ in
[03-datenmodell.md](../spec/03-datenmodell.md), Abschnitt 6.7 (R-DM-26),
[04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitte 6.3 und 7.3 bis 7.5 (R-SY-18), und
[16-betrieb.md](../spec/16-betrieb.md), Abschnitt 8.4 (R-OP-11).

1. Die Einrichtung erzeugt die erste Epoche. `bothy restore finalize` setzt nach jeder
   Wiederherstellung in einer Transaktion: neue Epoche, `agent_consumer_cursors.acked_seq`,
   `webhook_subscriptions.last_delivered_seq` und `event_consumer_cursors.last_seq` auf den neuen
   `head_seq`, Audit-Eintrag.
2. `hello`, `ready`, `resumed`, die Momentaufnahme und `GET /api/v1/events` nennen die Epoche. Der
   Client speichert sie mit dem Cursor und sendet beide bei `identify`, `resume` und im
   REST-Nachholen.
3. Passt die Epoche nicht, antwortet der Server `resync_required` mit Grund `epoch_changed`, auch
   wenn `cursor <= head_seq`. Der Client verwirft den Server-Zustand im Cache, behält das
   Ausgangsfach, lädt die Momentaufnahme und setzt Cursor und Epoche neu.
4. Ausgehende Webhooks tragen die Epoche als Zustellfeld `sync_epoch` im Umschlag (additiv nach 04,
   Abschnitt 6.1). Grund: Webhook-Empfänger ordnen nach `seq` ([webhooks.md](../api/webhooks.md),
   Abschnitt 3) und würden kleinere Nummern der neuen Zeitlinie sonst als alt verwerfen. Ein
   Empfänger, der eine neue Epoche sieht, setzt seinen Stand zurück.
5. R-SY-04 bleibt als zweites Netz für Clients ohne gespeicherte Epoche.
6. Ein Umzug ohne Datenverlust behält die Epoche; Clients setzen mit ihrem Cursor fort.

### Consequences

- Gut, weil kein Client nach einer Wiederherstellung still Ereignisse überspringt oder gelöschte
  Daten weiter anzeigt.
- Gut, weil das Ausgangsfach erhalten bleibt: Nachrichten, die ein Client noch nicht bestätigt
  bekam, sendet er erneut, und sie entstehen in der neuen Zeitlinie wieder.
- Gut, weil Agenten und Webhooks nichts aus der alten Zeitlinie nachholen; die Cursor stehen auf dem
  neuen Kopf.
- Schlecht, weil jede Wiederherstellung alle Clients neu laden lässt (Momentaufnahme, Threads bei
  Bedarf); das kostet Last direkt nach dem Restore.
- Schlecht, weil die Epoche nur wechselt, wenn `bothy restore finalize` läuft. Startet ein
  Betreiber `api` vor diesem Schritt, gilt bis zum Überholen nur R-SY-04. Das Runbook
  ([backup-restore.md](../operations/backup-restore.md), Abschnitt 5) setzt den Schritt vor den
  Start; eine technische Sperre fehlt noch (More Information).
- Schlecht, weil Nachrichten, die ein Client schon bestätigt bekam und aus dem Ausgangsfach
  entfernt hat, mit der alten Zeitlinie verloren sind (Grenze des RPO, backup-restore.md
  Abschnitt 7).

### Confirmation

- Bau-Ticket M7-04 in [PLAN.md](../PLAN.md): Ein Client mit Cursor aus der alten Zeitlinie bekommt
  `resync_required` mit `epoch_changed`, auch wenn `head_seq` den Cursor überholt hat; ein Umzug
  ohne Restore ändert die Epoche nicht.
- Test: Nach `bothy restore finalize` stehen alle Agenten-, Webhook- und Worker-Cursor auf
  `head_seq`, und ein Audit-Eintrag existiert; bricht der Befehl ab, ist nichts geändert.
- Test: Eine Webhook-Zustellung nach dem Restore trägt die neue Epoche.
- Wiederherstellungstest (ADR 0058) und Portabilitätstest (16, Abschnitt 11.1): Manifest nennt
  `sync_epoch`.
- Test `TestListEventsResync` (M1-63, `services/server/internal/sync/adapters/httpapi`), case
  `epoch_changed`: `GET /api/v1/events` with an `epoch` other than `instance.sync_epoch` answers
  `410 resync_required` with `reason = epoch_changed`, also when the cursor is above `head_seq`.

## Pros and Cons of the Options

### Epoche

- Gut, weil eine einzige Zahl je Zeitlinie, unabhängig von Nummern und Anbieter.
- Schlecht, weil Clients ein weiteres Feld speichern und senden müssen.

### Nur R-SY-04

- Gut, weil ohne neues Feld.
- Schlecht, weil die Prüfung nur greift, solange die neue Zeitlinie den Cursor nicht überholt hat;
  danach überspringt der Client still.

### Nummern springen lassen

- Gut, weil neue Nummern immer größer als alte Cursor wären.
- Schlecht, weil der Server die höchste je vergebene Nummer nach einem Verlust nicht kennt; sie lag
  in der verlorenen Zeitlinie.
- Schlecht, weil Clients trotzdem Daten aus der verlorenen Zeitlinie im Cache behielten; ein
  Neuladen ist ohnehin nötig.

### Cursor je Gerät im Server

- Gut, weil der Server gezielt neu laden ließe.
- Schlecht, weil der Server dafür einen Zustand je Gerät dauerhaft halten müsste, der nach einer
  Wiederherstellung selbst veraltet ist.

## More Information

- Offen: eine Startsperre, die eine nicht abgeschlossene Wiederherstellung erkennt. Kandidat ist der
  Vergleich von `pg_control_system().system_identifier` und `pg_control_checkpoint().timeline_id`
  mit Werten, die `bothy restore finalize` speichert (Funktionen:
  https://www.postgresql.org/docs/18/functions-info.html, abgerufen 2026-10-10 [belegt]). Ob
  verwaltete Dienste diese Werte der Anwendungsrolle zeigen und wie sich ein Failover auswirkt, ist
  nicht geprüft [vermutet]; das Bau-Ticket M7-04 klärt es.
- Verwandt: ADR 0009 (Ereignis-Log), ADR 0051 (Worker-Verbraucher), ADR 0052 (Umschlag),
  ADR 0058 (Sicherung).
