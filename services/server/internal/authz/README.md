# Modul `authz`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

`authorize`, Rechte-Cache, sichtbare Ziele, Gruppen, Freigaben, Rechte-Version, I1-Prüfung, "Wer hat Zugriff", "Als Person ansehen".

## Port (Wurzelpaket `internal/authz`)

- Eingehend: `Admin` (Gruppen, Freigaben, Erklärung, Ansicht als Person); deklariert `PrincipalSource`, `StructureSource`, `AgentPolicySource`.
- Gibt In-Tx-Ereignisse aus: kernel `authorize.AccessChanged`.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `authorize.Authorizer`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`.

Grund: Zähler `permission_version`; alle anderen Daten über Quellen-Ports.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`groups`, `group_memberships`, `grants`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`access.explain`, `access.preview`, `access.view_as`, `group.read`, `group.create`, `group.update`, `group.delete`, `group.member.add`, `group.member.remove`, `group.member.expire`, `grant.read`, `grant.create`, `grant.revoke`, `grant.expire`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `grant.changed`, `group.changed`, `access.changed`, `access.expiring`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `authz.expiry_reminder` | periodisch, `access.expiring` vor Ablauf | `maintenance` |
| `authz.expiry_sweep` | periodisch, `access.changed` nach Ablauf (Systemaktionen `grant.expire`, `group.member.expire`) | `maintenance` |

## Hinweise für den Bau

- Einzige Stelle mit Rechte-Logik (B7). Andere Module rufen `authorize` über `kernel/authorize` und verknüpfen nie selbst mit `grants`.
- Daten anderer Module kommen über die Quellen-Ports `PrincipalSource`, `StructureSource`, `AgentPolicySource`; `authz` importiert keine Fachmodule.
- I1-Prüfung und `access.changed` laufen in Phase 3.1 der Unit of Work (02, 8.1).
- Der Rechte-Cache folgt `access.changed` über `Observe` in Log-Reihenfolge (02, 11.3).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [07-rechte.md](../../../../docs/spec/07-rechte.md)
