# services/server

Status: geplant (M0). Noch kein Code. Normativ ist
[docs/spec/02-architektur.md](../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

Der Server ist ein modularer Monolith in Go (ADR 0004, 0005): ein Binary `bothy`, eine
PostgreSQL-Datenbank (ADR 0006), ein Objektspeicher. Agenten, Plugins und Apps laufen nie in diesem
Prozess; sie nutzen die öffentliche API (B6, B10).

## Rollen und Befehle

| Befehl | Zweck |
|---|---|
| `bothy serve --role=api` | REST, Gateway-WebSocket, Log-Tail und Fan-out |
| `bothy serve --role=worker` | River-Jobs, Ereignis-Verbraucher (Push, Webhooks), Dateiverarbeitung, Wartung |
| `bothy serve --role=all` | beides in einem Prozess (Standard) |
| `bothy migrate up\|status\|validate` | Migrationen (goose, rivermigrate); nie implizit beim Start |
| `bothy setup ...` | Einrichtung, erster Owner (08-identitaet.md) |
| `bothy config check\|print` | Konfiguration prüfen und redigiert ausgeben |

## Aufbau

```text
cmd/bothy/            Einstieg, Unterbefehle
api/                  generiert aus docs/api/ (REST-Server, WebSocket-Typen, Routen-Metadaten)
architecture/         modules.yaml: Modulregister, Quelle für go-arch-lint und Prüfungen
migrations/           goose-SQL, eingebettet
internal/kernel/      Querschnitts-Ports
internal/platform/    Technik ohne Fachlogik
internal/wire/        Verdrahtung je Rolle, Registrierung der In-Tx-Abonnenten
internal/<modul>/     ein Modul je Verzeichnis (Tabelle unten)
sqlc.yaml             ein Eintrag je Modul
.go-arch-lint.yml     erzeugt aus architecture/modules.yaml
```

Jedes Modul hat dieselben Schichten: Port im Wurzelpaket, `domain/`, `app/`, `adapters/<art>/`
(02-architektur.md, Abschnitt 5).

## Module

| Modul | Kurz |
|---|---|
| [instance](internal/instance/README.md) | Einstellungen, Schalter, Signierschlüssel, Zähler |
| [audit](internal/audit/README.md) | Audit-Log mit Hash-Kette |
| [authz](internal/authz/README.md) | `authorize`, Gruppen, Freigaben, I1 |
| [billing](internal/billing/README.md) | Kosten und Preise |
| [identity](internal/identity/README.md) | Prinzipale, Geräte, Sitzungen, Tokens |
| [chat](internal/chat/README.md) | Bereiche, Kanäle, Threads |
| [agents](internal/agents/README.md) | Agenten, Backend-Richtlinie, Sitzungen, Befehle |
| [files](internal/files/README.md) | Dateien, Scan, Vorschau |
| [messaging](internal/messaging/README.md) | Nachrichten, Reaktionen, Anheften |
| [runs](internal/runs/README.md) | Auslösung, Läufe, Live-Box, Komponenten |
| [sync](internal/sync/README.md) | Ereignis-Log, Nachholen, persönliche Sync-Daten |
| [notify](internal/notify/README.md) | Benachrichtigung, Push |
| [gateway](internal/gateway/README.md) | WebSocket, Fan-out |
| [search](internal/search/README.md) | Volltextsuche |
| [stats](internal/stats/README.md) | Statistik für die Konsole |
| [apps](internal/apps/README.md) | Apps, Webhooks |
| [importer](internal/importer/README.md) | Import |
| [privacy](internal/privacy/README.md) | DSGVO-Aufträge |
| [status](internal/status/README.md) | Statusseite |

## `internal/kernel`

Querschnitts-Ports, die jedes Modul nutzen darf. `kernel` hängt von nichts im Projekt ab.

| Paket | Inhalt | Implementiert von |
|---|---|---|
| `ids`, `clock`, `errs` | UUIDv7, Zeitquelle, Fehlerarten mit Code | kernel |
| `actor` | Prinzipal einer Anfrage, `Authenticator` | `identity` |
| `authorize` | Aktionen (aus 05 erzeugt), Ressource, `Decision`, `Authorizer`, In-Tx-Ereignis `AccessChanged` | `authz` |
| `txn` | Unit of Work, Phasen vor dem Commit, In-Tx-Ereignisbus | `platform/pg` |
| `eventlog` | `Appender` (dauerhaft), `EphemeralPublisher` (kurzlebig) | `sync`, `gateway` |
| `auditing` | `Recorder` | `audit` |
| `idempotency` | `Store` für `Idempotency-Key` | `sync` |
| `personaldata` | `Participant` für Export und Löschung | Module mit personenbezogenen Tabellen |

## `internal/platform`

Technik ohne Fachlogik; nur `adapters/`, `wire` und `cmd` dürfen sie importieren.

| Paket | Inhalt |
|---|---|
| `config` | Laden und Prüfen der Konfiguration (02, Abschnitt 13) |
| `secrets` | Secret-Provider (ADR 0011) |
| `pg` | Pool, Unit of Work mit Sperrreihenfolge, `LISTEN`-Verbindung, Wiederholung bei Deadlock |
| `httpserver` | Server, Middleware-Kette, `problem+json`, Nachweis-Prüfung von `authorize` (02, Abschnitt 9) |
| `ratelimit` | Token-Buckets je Klasse aus 05 |
| `jobs` | einzige Stelle mit River: Client je Rolle, Warteschlangen, Schema `jobs.schema` |
| `otel` | Telemetrie-Setup, gekapselte `gen_ai.*`-Attribute (R-Telemetrie) |
| `health` | Ops-Listener mit `live` und `ready` |
| `lifecycle` | Start-Reihenfolge und Shutdown je Rolle (02, Abschnitt 15) |

## Regeln, die das Gate hält

- Module importieren nur Ports erlaubter Module; Fremdbibliotheken nur in zugelassenen Adaptern
  (go-arch-lint, ADR 0008).
- SQL eines Moduls nennt nur eigene Tabellen (02, 7.3).
- Jede Operation hat eine Aktion aus 05; der Server startet sonst nicht.
- Erzeugter Code (`api/`, sqlc, `.go-arch-lint.yml`) entspricht seiner Quelle.

## Namensgebundene Stellen

Produktname im Code nur hier: Go-Modulpfad in `go.mod`, Binary `bothy` (`cmd/bothy`), Präfix
`BOTHY_` der Umgebungsvariablen, Image-Name. Die Liste führt 18-release.md.
