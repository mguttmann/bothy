# Modul `instance`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Instanz-Identität, Sync-Epoche, Mindest-Server-Version, Einstellungen und Branding, Feature-Flags, Server-Signierschlüssel mit JWKS und Rotation, Sequenz-Zähler.

## Port (Wurzelpaket `internal/instance`)

- Eingehend: `Settings`, `Flags`, `Keys` (Signer je Zweck, JWKS, Rotation), `Counters` (nur für die Phasen-Hooks von `authz`, `sync`, `audit`).
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports keines anderen Moduls nutzen.

Grund: Fundament.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`instance`, `instance_settings`, `server_signing_keys`, `feature_flags`, `sequence_counters`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`instance.discovery.read`, `instance.read`, `instance.settings.read`, `instance.settings.update`, `instance.branding.update`, `instance.bootstrap`, `feature_flag.read`, `feature_flag.update`, `signing_key.rotate`.

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
