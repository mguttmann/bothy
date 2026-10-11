# Audit-Log und Telemetrie

Status: Entwurf M0. Abgrenzung nach B11. Audit-Log: [03-datenmodell.md](../spec/03-datenmodell.md)
4.12, Aktionen mit Audit: [05-aktionen.md](../spec/05-aktionen.md). Sicherheitskapitel 19 und
ADR 0021 legen Inhalt und Prüfung des Audit-Logs fest.

## 1. Gegenüberstellung

| Merkmal | Audit-Log (`audit_events`) | Telemetrie (Traces, Metriken, Logs) |
|---|---|---|
| Zweck | Nachweis: wer hat was entschieden oder verändert | Betrieb: läuft es, wo hakt es |
| Inhalt | Aktion, Entscheidung, Grund, Ressource, Akteur (Klar-ID), Gerät, Sitzung | Dauern, Zähler, Fehlerarten, IDs von Objekten, Pseudonyme von Menschen |
| Vollständigkeit | jedes Ereignis der Pflichtliste; Ablehnungen je Fenster zusammengefasst | gesampelt, darf bei Rückstau verworfen werden |
| Schreiben | in derselben Transaktion wie die Änderung; scheitert das Audit, scheitert die Änderung | asynchron, fail-open |
| Integrität | Hash-Kette, nur `INSERT` und `SELECT` für die Anwendung | keine |
| Ort | PostgreSQL der Instanz | Collector, dann Backend nach Wahl |
| Zugriff | Owner mit Step-up (`audit.read`, `audit.verify`) | Betreiber des Backends |
| Frist | eigene Klasse `audit` (Kapitel 20) | kurz, je Backend ([privacy.md](privacy.md) 5) |
| Sicherung | mit der Datenbank, Kette im Wiederherstellungstest geprüft | keine |
| Export | nur durch den Owner über `exportAuditEvents`; geprüft wird die Kette mit `verifyAuditChain` (M1-50) | per Overlay |

## 2. Regeln

1. Kein Audit-Eintrag entsteht aus Telemetrie, keine Telemetrie aus dem Audit-Log. Beide schreibt
   der Code an derselben Stelle getrennt.
2. Verbindung nur über `correlation_id`: Sie ist die Trace-ID der Anfrage
   ([signals.md](signals.md) 5.1). Ein Prüfer kann vom Audit-Eintrag zum Trace springen, solange
   der Trace noch aufbewahrt und aufgezeichnet ist.
3. Ablehnungen von `authorize` stehen in beiden: im Audit-Log als Nachweis (zusammengefasst nach
   `audit.deny_aggregate_window`), in der Telemetrie als Zähler `bothy.authz.decisions` ohne
   Akteur.
4. Der Collector bekommt nie den Inhalt von `audit_events.details`.
5. Das Audit-Log enthält Klar-IDs von Menschen, die Telemetrie nur Pseudonyme. Wer Telemetrie
   betreibt, kann damit nicht nachvollziehen, wer etwas getan hat; dafür ist das Audit-Log da.
6. Die Prüfung der Hash-Kette läuft periodisch im `worker` (`bothy.audit.verify.*`) und in jedem
   Wiederherstellungstest; ein Fehler ist ein Alarm `page`.

## 3. Beispiel: Freigabe widerrufen

| Ort | Was entsteht |
|---|---|
| `audit_events` | `action = grant.revoke`, `decision = allow`, Akteur, Gerät, Sitzung, `resource_id` der Freigabe, `correlation_id` |
| `event_log` | `access.changed` an Betroffene, ggf. `run.cancel_requested`, gleiche `correlation_id` |
| Trace | Span `POST /api/v1/grants/{id}/revoke` mit `bothy.action = grant.revoke`, `bothy.decision = allow`, Pseudonym des Akteurs |
| Metrik | `bothy.authz.decisions{bothy.action="grant.revoke", bothy.decision="allow"}` plus eins |
