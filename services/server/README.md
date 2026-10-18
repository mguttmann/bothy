# services/server

Status: Gerüst (M1-05) mit Konfiguration (M1-07) und Secret-Provider (M1-09). Das Go-Modul baut;
jedes Modul aus 02 existiert als Port-Paket, die Kernel-Pakete `ids`, `clock` und `errs`, das
Plattform-Paket `config` mit `bothy config check|print` und das Plattform-Paket
[`secrets`](internal/platform/secrets/README.md) mit `bothy secrets init|export` sind fertig, und
`make arch` hält die Modulgrenzen aus dem Register.
Since M1-11 `bothy serve` starts every role with the internal listener, health, the database
and schema check and the public listener, shuts down within `shutdown.total_timeout`, and
`bothy healthcheck` probes a loopback health endpoint.
Since M1-23 `make image` builds the multi-arch image `bothy-server` (section [Image](#image)).
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
| `bothy config check\|print` | Konfiguration prüfen (mit `--secrets` auch jeden Verweis auflösen) und redigiert ausgeben |
| `bothy import login\|logout` | login of the import tool as a `cli` device through device pairing and logout (22-import-migration.md 10.1); session file and DPoP client: [`cmd/bothy/importtool/bothyapi`](cmd/bothy/importtool/bothyapi/README.md) |
| `bothy import discord ...` | Import und Lese-Spiegel über die API (22-import-migration.md); Code unter `cmd/bothy/importtool` (D-W-124), Discord-Lese-Client: [`cmd/bothy/importtool/discord`](cmd/bothy/importtool/discord/README.md) |
| `bothy restore finalize` | nach einer Wiederherstellung: neue Sync-Epoche (16-betrieb.md) |
| `bothy secrets init\|export` | Geheimnisse anlegen und sichern (16-betrieb.md) |
| `bothy healthcheck --url` | Gesundheitsprüfung einer Loopback-URL des internen Listeners (16-betrieb.md) |
| `bothy files rescan` | Dateien erneut prüfen lassen (12-dateien.md) |
| `bothy telemetry pseudonym` | Pseudonym eines Prinzipals für die Fehlersuche |
| `bothy dev seed` | Testdaten, nur in der Umgebung `dev` |

Health: `/healthz` (der Prozess lebt) und `/readyz` (bereit) nur auf dem internen Listener
`server.internal_listen_addr`, außerhalb der öffentlichen API und ohne Aktion (Ruling L2,
[16-betrieb.md](../../docs/spec/16-betrieb.md)).

Health (W2 resolved in 02 "Rollen eines Prozesses", R-AR-13): `health` means `/healthz` and
`/readyz` on the internal listener only; metrics leave the process only over OTLP. The public
listener answers both paths with `404`. Answer format and check names:
[`internal/platform/health`](internal/platform/health/README.md). Exit codes of `bothy serve`:
`0` graceful end, `1` failed start, fatal runtime error or shutdown over
`shutdown.total_timeout`, `2` usage or configuration error, `3` forced by a second signal
([`internal/platform/lifecycle`](internal/platform/lifecycle/README.md)).

## Aufbau

```text
cmd/bothy/            Einstieg, Befehlstabelle der Unterbefehle
api/                  generiert aus docs/api/ (REST-Server, WebSocket-Typen, Routen-Metadaten)
architecture/         modules.yaml: Modulregister, Quelle für go-arch-lint und Prüfungen;
                      config-schema.json: exportiertes Konfigurationsschema (erzeugt)
migrations/           goose-SQL, eingebettet
internal/kernel/      Querschnitts-Ports
internal/platform/    Technik ohne Fachlogik
internal/wire/        Verdrahtung je Rolle, Registrierung der In-Tx-Abonnenten
internal/<modul>/     ein Modul je Verzeichnis (Tabelle unten)
test/, perf/          Testcode über Modulgrenzen und Lasttests (Komponente testsupport)
tools/archgen/        Generator: architecture/modules.yaml -> .go-arch-lint.yml (go generate)
tools/configschema/   Generator: Konfigurationsschema -> architecture/config-schema.json (go generate)
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
GO_ARCH_LINT="$(uv run --project ../../tools --locked --quiet python ../../tools/bin/fetch_tools.py path go-arch-lint)" go test -race -tags contract_validate ./...
```

Contract tests (M1-71): the tag `contract_validate` turns on G8, the validation of every response of
the public listener against `docs/api/openapi.yaml`
([`contractcheck`](internal/platform/httpserver/contractcheck/README.md); a violation ends the test
binary with exit code 3). `make contract-test` runs G7, Schemathesis against the server of this tree
([`test/contract`](test/contract/README.md)). Full-server tests build on the shared harness
[`test/harness`](test/harness/README.md). The leak tests behind threat T-AZ-02 (M1-70: AZ-P4 404
equality and timing per route, AZ-P5 property tests of lists, counters, snapshot and catch-up on
random worlds, AZ-M52) live in [`test/leak`](test/leak/README.md); a failing AZ-P5 seed is
replayed with `LEAK_SEED=<seed>`.

Load and chaos tests (M1-16, M1-72): `make perf` runs the packages under [`perf`](perf/README.md)
and [`test/chaos`](test/chaos/README.md) with the build tag `perf`, one after another, and prints the
table of every performance target with value, threshold and the profile mode of ADR 0063; it is not
part of `make check`, which only vets and lints the tagged code. The thresholds come from the table
of chapter 02, never from a literal. The production validator compiles request schemas only; the
test build with `contract_validate` keeps the responses for G8.

## Image

The image `bothy-server` (ADR 0012, 18-release.md "Server-Images") is built from
[`Dockerfile`](Dockerfile) with the repository root as build context; the ignore file
[`Dockerfile.dockerignore`](Dockerfile.dockerignore) lets only the server sources into the
context, without tests and test data.

```sh
make image        # linux/amd64 and linux/arm64 into .cache/image/bothy-server.oci.tar,
                  # host platform loaded as bothy-server:<VERSION>-local
make image-test   # structure tests of that image (services/server/test/image, tag imagetest)
make trivy-image IMAGE=bothy-server:<VERSION>-local
```

- The builder stage runs on the build platform and cross compiles `bothy` for the target
  platform without cgo (`CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, `-ldflags "-s -w"`);
  the final stage only copies, so no step runs under emulation.
- The binary is `/usr/local/bin/bothy`, owned by `0:0` with mode `0555`. The image runs as the
  numeric user `65532:65532` (`nonroot` of distroless), has no shell and no package manager, and
  declares no `VOLUME`; it runs with a read-only root file system, without capabilities and
  with `no-new-privileges` (Compose `x-hardening`, `/tmp` as tmpfs). `secrets-init` overrides the
  user with `0:0`.
- The console stage (M6-02) runs on the build platform on the same builder image, clones Flutter
  at the version of the workspace (`pubspec.yaml`) and checks its revision, and builds
  `apps/console` (`flutter build web --release --base-href /console/ --no-web-resources-cdn`)
  plus its `version.json`. The final stage copies the build to `/usr/share/bothy/console`
  (owner `0:0`, read-only) and sets `BOTHY_CONSOLE__ASSETS_DIR` to it, so the api process serves
  the console under `/console/`.
- `/var/lib/bothy/signing-keys` (`secrets.signing_key_dir`) exists empty with owner `65532:65532`
  and mode `0700`, so a named volume mounted there takes that owner.
- Health check: `/usr/local/bin/bothy healthcheck --url http://127.0.0.1:8081/readyz`, interval
  `15s`, timeout `5s`, start period `30s`, `4` retries, equal to `x-server-healthcheck` in
  `infra/compose/compose.yaml` (a test of `make image-test` compares both).
- Build arguments: `VERSION` (required, must equal [VERSION](VERSION)), `REVISION` (commit SHA;
  a build from a dirty tree still labels `HEAD`) and `SOURCE_URL` (default
  `https://github.com/OWNER/bothy`). They set the OCI labels `org.opencontainers.image.version`,
  `.revision` and `.source`; `.title` is `bothy-server`, `.licenses` is `Apache-2.0`.
- Local tags are `<VERSION>-local`, never `latest`. The release workflow pushes by digest and
  tags only after the vulnerability scan.

Base images, pinned by digest (`make lint` rejects any `FROM` without one; Renovate updates the
digests, and the golang tag follows the `toolchain` of [go.mod](go.mod)):

| Stage | Tag | Digest (image index) |
|---|---|---|
| build, console | `golang:1.27.2-trixie` | `sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d` |
| final | `gcr.io/distroless/static-debian13:nonroot` | `sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3` |

`make image` builds several platforms at once. Docker Engine with the containerd image store does
that with its default builder; otherwise create a builder once and pass it:

```sh
docker buildx create --name bothy-multiarch --driver docker-container
make image IMAGE_BUILDER=bothy-multiarch
```

Variables: `IMAGE_NAME`, `IMAGE_TAG`, `IMAGE_PLATFORMS`, `IMAGE_OCI`, `IMAGE_BUILDER`; `make
image-test IMAGE=<ref>` tests another loaded image.

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
| [`pg`](internal/platform/pg/README.md) | Pool, Unit of Work mit Sperrreihenfolge, `LISTEN`-Verbindung, Wiederholung bei Deadlock |
| [`httpserver`](internal/platform/httpserver/README.md) | Server, Middleware-Kette, `problem+json`, Nachweis-Prüfung von `authorize` (02, Abschnitt 9); listener part built in M1-11, chain in M1-18, transport protection in M1-21 (plaintext only from loopback and `server.trusted_proxies`, own TLS from 1.2 with rotation, HSTS on both listeners; 19, SEC-TLS-1 to SEC-TLS-3) |
| `ratelimit` | Token-Buckets je Klasse aus 05 |
| [`jobs`](internal/platform/jobs/README.md) | einzige Stelle mit River: Client je Rolle, Warteschlangen, Schema `jobs.schema`, Migrationen in `bothy migrate`; built in M1-14 |
| `otel` | Telemetrie-Setup, gekapselte `gen_ai.*`-Attribute (R-Telemetrie) |
| [`health`](internal/platform/health/README.md) | `/healthz` und `/readyz` auf dem internen Listener `server.internal_listen_addr` (L2); built in M1-11 |
| [`lifecycle`](internal/platform/lifecycle/README.md) | Start-Reihenfolge und Shutdown je Rolle (02, Abschnitt 15); built in M1-11 |
| [`egress`](internal/platform/egress/README.md) | einziger Client für ausgehende HTTP-Verbindungen: Ziel-Erlaubnisliste, IP-Prüfung beim Verbindungsaufbau, geprüfte Weiterleitungen, Fristen und Größengrenzen, Loopback-Client für den Health-Check (19-sicherheit.md, Abschnitt 8; gebaut in M1-22) |
| `buildinfo` | Version des Servers als Konstante gleich `VERSION` (18-release.md) |

## Regeln, die das Gate hält

- Module importieren nur Ports erlaubter Module; Fremdbibliotheken nur in zugelassenen Adaptern
  (go-arch-lint, ADR 0008).
- SQL eines Moduls nennt nur eigene Tabellen (02, 7.3).
- Jede Operation hat eine Aktion aus 05; der Server startet sonst nicht.
- Register, Kern-Vertrag (03, `actions.yaml`), Modultabellen in 02, Komponenten-Diagramm und
  Modul-READMEs stimmen überein (`make register-check`, ADR 0050).
- Erzeugter Code (`api/`, sqlc, `.go-arch-lint.yml`) entspricht seiner Quelle.
- Every reading sqlc query on a content table carries `channel_id = ANY(...)` or
  `space_id = ANY(...)` with the visible lists of `Authorizer.Scope`, or stands with a category
  and a reason on the reviewed exception list `tools/lint/sql_visibility.yaml` (`make
  sql-visibility`, AZ-P12, 07 7.4 rule 1).

## Namensgebundene Stellen

Produktname im Code nur hier: Go-Modulpfad in `go.mod`, Binary `bothy` (`cmd/bothy`), Präfix
`BOTHY_` der Umgebungsvariablen, Image-Name und der Standardwert des angezeigten Produktnamens,
die Konstante `DefaultProductName` in `internal/instance/domain` (Einstellung
`branding.product_name`, ADR 0044). Die vollständige Liste nach Art führt 02-architektur.md,
Abschnitt 6; `tools/ci/tests/test_product_name_places.py` hält sie.
