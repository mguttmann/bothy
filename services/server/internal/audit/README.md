# Modul `audit`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Audit-Log mit Hash-Kette, signierte Prüfpunkte, Abfrage, Prüfung, Kürzung mit Anker.

## Port (Wurzelpaket `internal/audit`)

- Eingehend: `Log` (Abfrage, Prüfung).
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `auditing.Recorder`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`.

Grund: Zähler `audit_seq`.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`audit_events`, `audit_checkpoints`, `audit_ip_keys`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`audit.read`, `audit.verify`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: keine.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `audit.chain_verify` | periodisch | `maintenance` |
| `audit.retention_trim` | periodisch, eigene Wartungsrolle der Datenbank | `maintenance` |

## Hinweise für den Bau

- Die Anwendungsrolle der Datenbank darf nur `INSERT` und `SELECT`; Kürzung läuft über eine eigene Wartungsrolle und schreibt einen Anker (03, 4.12).
- `Recorder` schreibt in der laufenden Transaktion (Phase 3.3 der Unit of Work) oder getrennt, wenn eine Ablehnung nach einem Rollback ins Audit muss.
- Gleiche Ablehnungen werden je `audit.deny_aggregate_window` zusammengefasst (05, R-AZ-04).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [19-sicherheit.md](../../../../docs/spec/19-sicherheit.md), [20-datenschutz.md](../../../../docs/spec/20-datenschutz.md)
