# Modul `instance`

Status: Gerüst (M1-05). Das Port-Paket existiert; Schnittstellen und Anwendungsfälle kommen mit den Tickets der Etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Instanz-Identität, Sync-Epoche, Mindest-Server-Version, Einstellungen und Branding, Feature-Flags, Server-Signierschlüssel mit JWKS und Rotation, Sequenz-Zähler.

## Code

- `internal/instance` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `domain/`, `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

## Port (Wurzelpaket `internal/instance`)

- Eingehend: `Settings`, `Flags`, `Keys` (Signer je Zweck, JWKS, Rotation), `Counters` (nur für die Phasen-Hooks von `authz`, `sync`, `audit`), `Versions` (Zählerstände ohne Sperre lesen, z. B. `permission_version` für den Rechte-Cache, `event_seq` für Catch-up); deklariert `LoginOptionsSource` (Anmelde-Treiber und Passwortweg für die Discovery), `FlagUserSource` (Menschen eines Rollouts), `FlagGroupSource` (gültige Gruppen eines Menschen für die Auswertung).
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: keine.

Grund: Fundament.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`instance`, `instance_settings`, `server_signing_keys`, `feature_flags`, `sequence_counters`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`instance.discovery.read`, `instance.read`, `instance.settings.read`, `instance.settings.update`, `instance.branding.update`, `feature_flag.read`, `feature_flag.update`, `signing_key.rotate`, `signing_key.transition`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `instance.updated`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `instance.key_rotation` | periodisch, Zustände `next` bis `retired` | `maintenance` |

## Hinweise für den Bau

- Sequenz-Zähler (`sequence_counters`) gibt das Modul nur über `Counters` an die Phasen-Hooks von `authz`, `sync` und `audit` heraus (02, 8.1 und 8.2); kein Anwendungsfall sperrt sie selbst.
- Private Schlüssel liegen nur im Secret-Provider (`private_key_ref`); die Tabelle hält öffentliche JWK und Zustand.
- Einstellungen sind Daten des Owners mit Schema je Schlüssel, keine Server-Konfiguration (02, Abschnitt 13).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [06-api.md](../../../../docs/spec/06-api.md), [16-betrieb.md](../../../../docs/spec/16-betrieb.md), [19-sicherheit.md](../../../../docs/spec/19-sicherheit.md)
