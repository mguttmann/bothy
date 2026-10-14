# services/server/api

Status: erzeugt aus dem Vertrag (M1-17). Die REST-Schicht des Servers entsteht aus
[docs/api/openapi.yaml](../../../docs/api/openapi.yaml) (spec-first, ADR 0013); der Server erzeugt
und registriert nur die Operationen, die die Freischalt-Liste [operations.yaml](operations.yaml)
nennt (PLAN P-04). Normativ sind [06-api.md](../../../docs/spec/06-api.md), Abschnitt
"Codegenerierung", und [05-aktionen.md](../../../docs/spec/05-aktionen.md), Regel 3.

## Aufbau

| Pfad | Inhalt | Von Hand? |
|---|---|---|
| `operations.yaml` | Freischalt-Liste: `enabled` und `later` mit Etappe | ja |
| `generate.go` | die beiden `go:generate`-Zeilen (echte Oberfläche und Fixture) | ja |
| `rest/oapi-codegen.yaml` | Einstellungen von oapi-codegen (`std-http-server`, `strict-server`, `models`, `skip-prune`) | ja |
| `rest/models.gen.go` | Go-Typen aller Schemas, Parameter und Request-Bodies (Teil vor `ServerInterface`) | erzeugt |
| `rest/rest.gen.go` | Server der freigeschalteten Operationen (ab `ServerInterface`) | erzeugt |
| `rest/objects.gen.go` | Request and response objects of the strict server (from the first `*RequestObject` up to `StrictServerInterface`, INT-W17/4) | erzeugt |
| `rest/openapi.gen.yaml` | Byte-gleiche Kopie von `docs/api/openapi.yaml`; `rest.Spec` (`rest/spec.go`) bettet sie für den Anfrage-Validator der HTTP-Kette ein (M1-18) | erzeugt |
| `routes/routes.go` | Typen, `Table`, `Recorder`, `Verify`, `Build`, Nachschlagen | ja |
| `routes/routes.gen.go`, `catalog.gen.go`, `build.gen.go` | Operationstabelle, Aktionskatalog, Verbindung zum erzeugten Server | erzeugt |
| `routes/gen/` | Generator mit Tests und `testdata/` (Fixture und Negativ-Fixtures) | ja |
| `routes/internal/fixture/` | Fixture mit drei Operationen, vom selben Generator erzeugt | erzeugt, außer `fixture.go` |
| `routes/routestest/` | Fixture-Oberfläche mit Stub für Tests außerhalb von `api/routes` | ja, nur für `_test.go` |

Komponente im Modulregister: `api` mit den Vendor-Gruppen `oapi` (Laufzeit von oapi-codegen),
`openapi` (kin-openapi, Lader des Generators) und `yaml` (Generator). Werkzeug: oapi-codegen v2.8.0, als `tool` in [go.mod](../go.mod) gepinnt,
Laufzeit `github.com/oapi-codegen/runtime` v1.7.0.

## Neu erzeugen

```sh
cd services/server
go generate ./api/...      # oder: go generate ./... (alles, was make codegen-check erzeugt)
make -C ../.. codegen-check
```

`make codegen-check` erzeugt alles neu und scheitert, wenn sich dabei eine Datei ändert (R-API-18).
Nach einer Änderung an `openapi.yaml`, `actions.yaml` oder `operations.yaml` gehören die neu
erzeugten Dateien in denselben Commit.

Der Generator ([routes/gen](routes/gen/main.go)) liest Spec, Katalog und Liste, meldet jeden Befund
mit Datei und Operation und schreibt dann nichts. Sonst schreibt er eine Kopie der Spec, in der
`paths` nur die Methoden freigeschalteter Operationen behält (Pfade ohne Methode fallen weg),
`webhooks` fehlt und `components` vollständig bleibt, in einen Ordner aus `os.MkdirTemp`, lässt
oapi-codegen darauf laufen und entfernt den Ordner auf jedem Weg. Warum eine vorgefilterte Kopie
statt `include-operation-ids`: siehe [DECISIONS.md](../../../docs/DECISIONS.md), M1-17/1.

Flags: `-spec`, `-catalog`, `-list`, `-codegen-config`, `-rest-dir`, `-routes-dir`, `-rest-pkg`
(Importpfad des Pakets in `-rest-dir`) und `-core-pkg` (Importpfad von `api/routes`; leer, wenn
`-routes-dir` selbst `api/routes` ist). Exitcodes: 0, 1 bei Befunden oder Fehlern, 2 bei falschem
Aufruf.

## Freischalt-Liste

```yaml
version: 1
enabled: []          # operationIds mit Handler; aufsteigend sortiert
later:               # jede andere operationId unter paths; sortiert nach operation_id
  - operation_id: addOwner
    etappe: M1
    ticket: M1-31    # optional
```

Der Generator lehnt ab und nennt jeden Eintrag: `version` ungleich 1, unbekannte Schlüssel, eine
Operation unter `paths` in keinem Abschnitt, ein Eintrag in beiden Abschnitten, doppelt in einem
oder nicht in `openapi.yaml`, ein Abschnitt nicht aufsteigend nach Byte-Reihenfolge, `etappe`
nicht `M1` bis `M8` oder `V1` bis `V3`, `ticket` nicht in der Form `<Etappe>-<zwei Ziffern>` oder
mit anderer Etappe. Operationen unter `webhooks:` stehen nicht in der Liste: Der Server sendet sie,
er bedient sie nicht. Wie `later` gefüllt wurde: DECISIONS.md, M1-17/4.

Über alle Operationen unter `paths`, freigeschaltet oder nicht, prüft der Generator außerdem:
`x-authz-action` und `x-rate-limit-class` vorhanden, Aktion im Katalog, Klasse gleich der Klasse
der Aktion und nicht `none`, keine Aktion mit Recht `system` (05, Regel 10), genau ein Server
`/api/v1` (W6), keine Health-Operation (L2), und Formen, die die Routen-Tabelle nicht beschreiben
könnte (DECISIONS.md, M1-17/13). Eine freigeschaltete Operation darf keine eigenen
Sicherheitsanforderungen haben (M1-17/19). Jedes Objekt-Schema, das ein Anfragekörper einer Operation
unter `paths` erreicht, muss unbekannte Mitglieder ablehnen (`additionalProperties: false`, bei `allOf`
`unevaluatedProperties: false`; ausgenommen `JsonObject` und Maps), sonst nennt der Generator das
Schema (W7, DECISIONS.md M1-18/12). Prüfungen und Metadaten stammen aus dem Dokument, das
der Lader von oapi-codegen (kin-openapi) liest, damit Tabelle und erzeugter Server dasselbe sehen
(M1-17/18).

## Eine Operation in openapi.yaml ergänzen

Jede neue Operation unter `paths` bekommt im selben Commit einen Eintrag unter `later` mit der
Etappe (und dem Ticket, das sie baut); sonst scheitert der Generator in `make codegen-check`.

## Eine Operation freischalten (Handler-Ticket)

1. Die operationId aus `later` nach `enabled` verschieben (sortiert) und `go generate ./api/...`.
   `rest.gen.go` bekommt die Methode im `StrictServerInterface`, `routes.gen.go` die Zeile der
   Tabelle; `routes.Server` ist ab der ersten Operation `rest.StrictServerInterface`.
2. Im Modul `internal/<modul>/adapters/httpapi` die Methode des Interfaces umsetzen und den Handler
   des Moduls in `wire.Handlers` einbetten (02 "Schichten im Modul", Regel 3). Fehlt eine Methode
   oder setzen zwei Module dieselbe um, baut das Paket nicht. Der Compiler prüft nur die
   Methoden: Der Start (`wire.Serve`, M1-11) baut jeden Modul-Handler mit seinen Abhängigkeiten
   und übergibt den gefüllten Verbund an `wire.MountRoutes`.
3. Tests, die die Zahl der Operationen festhalten (`TestRealTable_Enabled` in `api/routes`), mit
   ändern; `make codegen-check` und `make arch` grün.

## Start-Prüfung

`wire.MountRoutes(role, mux, handlers, cat, log)` ist Schritt 7 des Starts (02 "Start"): Für die
Rollen `api` und `all` ruft es `routes.Build(handlers, routes.Options{}, cat)`; since M1-25 `cat`
is the closed catalogue of `authz` (`authz/app.Catalog`), and the start step checks the route
bindings against it first (`internal/authz/README.md`). `routes.SpecCatalog()` remains for the
tests of this package. `Build` lässt den erzeugten
std-http- und Strict-Server auf einem `routes.Recorder` registrieren und vergleicht mit `Verify`
jedes registrierte Muster mit der Tabelle, jede Aktion mit dem Katalog und jede Klasse mit der
Klasse der Aktion (05 Regel 3, AZ-P1). Bei einem Befund protokolliert `MountRoutes`
`route check failed` auf Stufe error mit allen Befunden, hängt nichts ein und gibt den Fehler
zurück. Sonst hängt es jede Route unter `Route.Operation.Pattern` am öffentlichen Mux ein. Den
Befehl `bothy serve` und den öffentlichen Listener baut M1-11; der Vertrag mit diesem Ticket:
`bothy serve` ruft den Schritt vor dem Öffnen des öffentlichen Listeners auf und endet bei einem
Fehler mit Exitcode 1, bevor es bereit meldet (bis dahin ist der Abbruch auf der Ebene des
Schritts getestet, `TestServe_AbortsOnUnknownAction`). Der Katalog ist ein Parameter; M1-25
passes the closed catalogue of `authz`.

`routes.Options` trägt die Fehler-Handler des erzeugten Servers; `nil` heißt der Standard von
oapi-codegen, der den Fehlertext mit 400 oder 500 schreibt. M1-18 setzt die Problem-Schreiber ein.

## Metadaten je Route

`routes.Operation` nennt je freigeschalteter Operation `ID`, `Method`, `Path` (mit Basis-URL),
`Pattern` (genau wie der erzeugte Code es registriert), `Action`, `RateLimitClass`, `Public`
(`security: []`), `IdempotencyKey` (Kopfzeile `Idempotency-Key`, in der Spec
`#/components/parameters/IdempotencyKey`), `IfMatch` (Kopfzeile `If-Match`: `IfMatchRequired`
bei `required: true`, sonst `IfMatchOptional`), `RequestBody` und `BodyRequired` sowie
`ETagStatuses` (2xx-Status, deren Antwort `ETag` nennt) und `Download` (a 2xx response streams a
body that is not JSON, neither `application/json` nor a `+json` type, M1-50: the route class of
downloads with the deadline `http.transfer_timeout`). Nachschlagen über `routes.ByPattern` (der Wert von
`http.Request.Pattern`) und `routes.ByID`; die HTTP-Kette (M1-18 bis M1-20) liest daraus.

## Grenzen

- Erzeugter Code zählt nicht in die Go-Abdeckung (`*.gen.go`, DECISIONS.md M1-17/10); der
  Golden-Test des Generators und `make codegen-check` halten ihn.
- Der Generator teilt die Ausgabe von oapi-codegen vor `ServerInterface` in `models.gen.go` und
  `rest.gen.go` (`splitRest`, DECISIONS.md INT-W14/4), weil eine Datei `repo.max_file_kb`
  überschritt. `rest.gen.go` wächst weiter mit jeder freigeschalteten Operation;
  `TestRealRest_BelowMaxFileKB` und der Hook `check-added-large-files` halten die Grenze je Datei.
- Since INT-W17/4 the request and response objects of the strict server go to `objects.gen.go`,
  because `rest.gen.go` again exceeded `repo.max_file_kb` with M1-54 and M1-55.
