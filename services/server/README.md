# services/server

Status: Gerüst (M1-05) mit Konfiguration (M1-07). Das Go-Modul baut; jedes Modul aus 02 existiert
als Port-Paket, die Kernel-Pakete `ids`, `clock` und `errs` und das Plattform-Paket `config` mit
`bothy config check|print` sind fertig, und `make arch` hält die Modulgrenzen aus dem Register.
Normativ ist [docs/spec/02-architektur.md](../../docs/spec/02-architektur.md); bei Abweichung gilt
das Kapitel.

Modulpfad bis V1: `github.com/OWNER/bothy/services/server` (PLAN P-05). `go.mod` nennt die ältere
gepflegte Go-Linie als `go`-Direktive und die gebaute Version als `toolchain` (ADR 0005). Version:
[VERSION](VERSION), Änderungen: [CHANGELOG.md](CHANGELOG.md).

Der Server ist ein modularer Monolith in Go (ADR 0004, 0005): ein Binary `bothy`, eine
PostgreSQL-Datenbank (ADR 0006), ein Objektspeicher. Agenten, Plugins und Apps laufen nie in diesem
Prozess; sie nutzen die öffentliche API (B6, B10).

## Rollen und Befehle

Das Register aller Unterbefehle steht in 02-architektur.md, Abschnitt 3.3 (Ruling L1); hier nur
der Überblick.

| Befehl | Zweck |
|---|---|
| `bothy serve --role=api` | REST, Gateway-WebSocket, Log-Tail und Fan-out |
| `bothy serve --role=worker` | River-Jobs, Ereignis-Verbraucher (Push, Webhooks), Dateiverarbeitung, Wartung |
| `bothy serve --role=all` | beides in einem Prozess (Standard) |
| `bothy migrate up\|status\|validate` | Migrationen (goose, rivermigrate); nie implizit beim Start |
| `bothy setup bootstrap-owner\|issue-enrollment-code` | Einrichtung, erster Owner, Einladungs-Code (08-identitaet.md) |
| `bothy config check\|print` | Konfiguration prüfen und redigiert ausgeben |
| `bothy import discord ...` | Import und Lese-Spiegel über die API (22-import-migration.md) |
| `bothy restore finalize` | nach einer Wiederherstellung: neue Sync-Epoche (16-betrieb.md) |
| `bothy secrets init\|export` | Geheimnisse anlegen und sichern (16-betrieb.md) |
| `bothy healthcheck --url` | Gesundheitsprüfung einer Loopback-URL des internen Listeners (16-betrieb.md) |
| `bothy files rescan` | Dateien erneut prüfen lassen (12-dateien.md) |
| `bothy telemetry pseudonym` | Pseudonym eines Prinzipals für die Fehlersuche |
| `bothy dev seed` | Testdaten, nur in der Umgebung `dev` |

Health: `/healthz` (der Prozess lebt) und `/readyz` (bereit) nur auf dem internen Listener
`server.internal_listen_addr`, außerhalb der öffentlichen API und ohne Aktion (Ruling L2,
[16-betrieb.md](../../docs/spec/16-betrieb.md)).

## Aufbau

```text
cmd/bothy/            Einstieg, Befehlstabelle der Unterbefehle
api/                  generiert aus docs/api/ (REST-Server, WebSocket-Typen, Routen-Metadaten)
architecture/         modules.yaml: Modulregister, Quelle für go-arch-lint und Prüfungen
migrations/           goose-SQL, eingebettet
internal/kernel/      Querschnitts-Ports
internal/platform/    Technik ohne Fachlogik
internal/wire/        Verdrahtung je Rolle, Registrierung der In-Tx-Abonnenten
internal/<modul>/     ein Modul je Verzeichnis (Tabelle unten)
test/, perf/          Testcode über Modulgrenzen und Lasttests (Komponente testsupport)
tools/archgen/        Generator: architecture/modules.yaml -> .go-arch-lint.yml (go generate)
sqlc.yaml             ein Eintrag je Modul
.go-arch-lint.yml     erzeugt aus architecture/modules.yaml, nie von Hand bearbeitet
.golangci.yml         golangci-lint v2 des Moduls
```

Das Register `architecture/modules.yaml` ist die einzige Quelle der Modulgrenzen (ADR 0050). Ändert
sich ein Eintrag, erzeugt `go generate ./tools/archgen` die Konfiguration neu; `make codegen-check`
meldet eine veraltete Datei, `make arch` jedes Paket ohne Komponente.

Jedes Modul hat dieselben Schichten: Port im Wurzelpaket, `domain/`, `app/`, `adapters/<art>/`
(02-architektur.md, Abschnitt 5).

## Tests

`make test` im Repository-Stamm testet das Modul und setzt dabei `GO_ARCH_LINT`. Die Tests von
`tools/archgen` prüfen die erzeugte Konfiguration mit dem gepinnten go-arch-lint und scheitern ohne
diese Variable. Von Hand, aus `services/server`:

```sh
GO_ARCH_LINT="$(uv run --project ../../tools --locked --quiet python ../../tools/bin/fetch_tools.py path go-arch-lint)" go test -race ./...
```

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
| [terminal](internal/terminal/README.md) | Rechner, Kopplung, Terminal-Sitzungen, Relais (ab M2) |

## `internal/kernel`

Querschnitts-Ports, die jedes Modul nutzen darf. `kernel` hängt von nichts im Projekt ab.

| Paket | Inhalt | Implementiert von |
|---|---|---|
| `ids`, `clock`, `errs` | UUIDv7, Zeitquelle, Fehlerarten mit Code | kernel |
| `textcheck` | Prüfung von Anzeige-Texten (SEC-IN-4) | kernel |
| `reqmeta` | `correlation_id` und Client-IP im Kontext | kernel; gesetzt von `platform/httpserver` |
| `jose` | Kompakte JWS und JWK mit der Standardbibliothek (ADR 0067) | kernel |
| `designkeys` | erlaubte Schlüssel des Design-Systems, erzeugt aus den Tokens | Generator |
| `actor` | Prinzipal einer Anfrage, `Authenticator` | `identity` |
| `authorize` | Aktionen (aus 05 erzeugt), Ressource, `Decision`, `Authorizer`, In-Tx-Ereignis `AccessChanged` | `authz` |
| `txn` | Unit of Work, Phasen vor dem Commit, In-Tx-Ereignisbus | `platform/pg` |
| `eventlog` | `Appender` (dauerhaft), `Redactor` (schwärzt die Nutzlasten einer gelöschten Nachricht), `EphemeralPublisher` (kurzlebig) | `sync`, `gateway` |
| `auditing` | `Recorder` | `audit` |
| `idempotency` | `Store` für `Idempotency-Key` | `sync` |
| `personaldata` | `Participant` für Export und Löschung | Module mit personenbezogenen Tabellen |
| `measure` | `Recorder` der eigenen Messung, nur Zähler im Speicher | `stats` |

`ids`, `clock`, `errs`, `textcheck`, `reqmeta`, `jose` und `designkeys` stehen im Register unter
`kernel.core` und dürfen von jedem Paket genutzt werden, auch von `domain`; die übrigen stehen unter
`kernel.ports` und sind für `domain` gesperrt (02, Abschnitt 5). Maßgeblich ist das Register.

## `internal/platform`

Technik ohne Fachlogik; nur `adapters/`, `wire` und `cmd` dürfen sie importieren.

| Paket | Inhalt |
|---|---|
| [`config`](internal/platform/config/README.md) | Schema, Laden und Prüfen der Konfiguration, `Bind` an die `Config`-Strukturen (02, Abschnitt 13; gebaut in M1-07) |
| `secrets` | Secret-Provider (ADR 0011) |
| `pg` | Pool, Unit of Work mit Sperrreihenfolge, `LISTEN`-Verbindung, Wiederholung bei Deadlock |
| `httpserver` | Server, Middleware-Kette, `problem+json`, Nachweis-Prüfung von `authorize` (02, Abschnitt 9) |
| `ratelimit` | Token-Buckets je Klasse aus 05 |
| `jobs` | einzige Stelle mit River: Client je Rolle, Warteschlangen, Schema `jobs.schema` |
| `otel` | Telemetrie-Setup, gekapselte `gen_ai.*`-Attribute (R-Telemetrie) |
| `health` | `/healthz` und `/readyz` auf dem internen Listener `server.internal_listen_addr` (L2) |
| `lifecycle` | Start-Reihenfolge und Shutdown je Rolle (02, Abschnitt 15) |
| [`egress`](internal/platform/egress/README.md) | einziger Client für ausgehende HTTP-Verbindungen: Ziel-Erlaubnisliste, IP-Prüfung beim Verbindungsaufbau, geprüfte Weiterleitungen, Fristen und Größengrenzen, Loopback-Client für den Health-Check (19-sicherheit.md, Abschnitt 8; gebaut in M1-22) |
| `buildinfo` | Version des Servers als Konstante gleich `VERSION` (18-release.md) |

## Regeln, die das Gate hält

- Module importieren nur Ports erlaubter Module; Fremdbibliotheken nur in zugelassenen Adaptern
  (go-arch-lint, ADR 0008).
- SQL eines Moduls nennt nur eigene Tabellen (02, 7.3).
- Jede Operation hat eine Aktion aus 05; der Server startet sonst nicht.
- Erzeugter Code (`api/`, sqlc, `.go-arch-lint.yml`) entspricht seiner Quelle.

## Namensgebundene Stellen

Produktname im Code nur hier: Go-Modulpfad in `go.mod`, Binary `bothy` (`cmd/bothy`), Präfix
`BOTHY_` der Umgebungsvariablen, Image-Name. Die Liste führt 18-release.md.
