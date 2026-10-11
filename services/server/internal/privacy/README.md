# Modul `privacy`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

DSGVO-Aufträge, Export und Löschung über die Teilnehmer aller Module.

## Port (Wurzelpaket `internal/privacy`)

- Eingehend: `Requests`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: sammelt alle kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `files`.

Grund: betroffene Person; Exportpaket speichern.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`gdpr_requests`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`gdpr.request.create`, `gdpr.request.read`, `gdpr.request.decide`, `gdpr.export.download`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `gdpr.request_updated`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `privacy.export`, `privacy.erase` | Freigabe eines Auftrags | `privacy` |

## Hinweise für den Bau

- Sammelt alle Implementierungen von `kernel/personaldata.Participant`; ein neues Modul mit personenbezogenen Tabellen muss einen Teilnehmer registrieren.
- Exportpakete liegen als Datei über den Port von `files`; Frist `due_at` laut 20-datenschutz.md.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [20-datenschutz.md](../../../../docs/spec/20-datenschutz.md)
