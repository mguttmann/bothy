---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team B (API)
informed: alle Teams
---

# Spec-first: OpenAPI 3.1 und AsyncAPI 3.0 normativ, Codegenerierung

## Context and Problem Statement

Der Server (Go), die Apps (Flutter/Dart), die SDKs (Python, TypeScript) und der Hermes-Adapter
sprechen dieselbe API. Dazu kommen Gateway-Ereignisse und Webhooks mit denselben Objekten. Jede
Route muss eine Aktion aus dem Katalog tragen (B7, 05-aktionen.md Regel 3). Wie halten wir einen
Vertrag, aus dem alle Seiten typisiert erzeugt werden und der sich prüfen lässt?

## Decision Drivers

* Ein Vertrag für vier Sprachen, ohne Handarbeit an Typen (Baseline B2, R-Clients).
* Autorisierungs-Aktion und Rate-Limit-Klasse an jeder Operation, maschinell prüfbar.
* REST, Gateway und Webhooks teilen Schemas (Ereignis-Umschlag, Nachricht, Lauf).
* Werkzeuge müssen mit OpenAPI 3.1 umgehen; Mehrtyp-Unions sind für oapi-codegen ein Risiko
  (Baseline R-Stack).
* Änderungen sollen im Review als Vertragsänderung sichtbar sein.

## Considered Options

* Spec-first: handgepflegte openapi.yaml und asyncapi.yaml sind normativ, Code wird erzeugt.
* Code-first: Spec wird aus annotiertem Go-Code erzeugt.
* Zwischensprache (TypeSpec oder ähnlich), aus der OpenAPI und AsyncAPI erzeugt werden.

## Decision Outcome

Chosen option: "Spec-first", because es als einzige Option den Vertrag vor dem Code festlegt,
alle vier Generatoren direkt bedient und die Pflichtfelder `x-authz-action` und
`x-rate-limit-class` per Lint durchsetzt, ohne eine weitere Sprache einzuführen.

Festlegungen:

* openapi.yaml (OpenAPI 3.1) enthält REST, Webhooks und alle Schemas, auch die des Gateways.
  asyncapi.yaml (AsyncAPI 3.0) referenziert sie per `$ref`. Ereignis-Schemas tragen keine
  OpenAPI-Diskriminator-Objekte, weil AsyncAPI-Parser einen String erwarten (eigene Probe mit
  @asyncapi/cli 6.2.0 am 2026-10-10).
* Konservative Schemas: kein `null`, keine Typ-Arrays, kein `anyOf`, `oneOf` nur mit
  Diskriminator, wiederholte Enums als benannte Schemas (06-api.md, Abschnitt 2 und 14).
* Generatoren: oapi-codegen (Go-Server, `std-http-server` plus `strict-server`), openapi-typescript,
  openapi-python-client, openapi-generator `dart-dio`. Erzeugter Code wird eingecheckt.
* Beschreibungen in den YAML-Dateien sind Englisch, weil sie in generierten Code wandern.

### Consequences

* Good, because alle Clients aus einer Quelle entstehen; eine Feldänderung bricht den Build
  sofort an jeder betroffenen Stelle.
* Good, because Lint-Regeln die Bindung an den Aktionskatalog erzwingen, bevor Code existiert.
* Good, because Reviewer Vertragsänderungen als Diff einer Datei sehen.
* Bad, because die Spec groß wird und von Hand gepflegt werden muss; Fehler in Beispielen fängt
  nur der Lint.
* Bad, because jeder Generator eigene Lücken hat (DPoP-Schema, `application/octet-stream` in
  Dart, strenge Enums in Python); jedes SDK braucht eine Handschicht.
* Neutral, because OpenAPI-3.1-Unterstützung in oapi-codegen v2.8.0 als "initial" gilt und in
  openapi-generator als Beta; die Probe vom 2026-10-10 lief trotzdem in allen vier Generatoren
  durch.

### Confirmation

Gate G1 (Redocly-Lint mit docs/api/redocly.yaml, null Fehler und Warnungen), G2 (Abgleich mit
05-aktionen.md und 04-sync-protokoll.md), G4 (alle vier Generatoren erzeugen und kompilieren,
kein Unterschied zum eingecheckten Code) und der Routen-Test aus 05-aktionen.md Abschnitt 9.
Siehe docs/spec/06-api.md, Abschnitt 15.

## Pros and Cons of the Options

### Spec-first

* Good, because der Vertrag vor der Implementierung feststeht und Apps parallel bauen können.
* Good, because Werkzeuge (Lint, Schemathesis, Generatoren) direkt auf der Datei arbeiten.
* Bad, because Server und Spec auseinanderlaufen können; dagegen helfen generierte Stubs und G8.

### Code-first

* Good, because die Spec nie vom Server abweicht.
* Bad, because der Vertrag erst nach dem Code entsteht und Apps warten.
* Bad, because Erweiterungen wie `x-authz-action` und Webhook-Abschnitte von der Annotation des
  Frameworks abhängen; für Go gibt es kein Werkzeug, das OpenAPI 3.1, Webhooks und AsyncAPI
  gemeinsam erzeugt [vermutet].

### Zwischensprache

* Good, because kompakter als YAML und mit Wiederverwendung.
* Bad, because eine weitere Sprache, ein weiterer Compiler und eine zweite Wahrheit neben der
  erzeugten YAML; Fehlermeldungen der Generatoren zeigen auf erzeugte Dateien.
* Bad, because AsyncAPI-3-Ausgabe ist dort nicht belegt [vermutet].

## More Information

* docs/spec/06-api.md (Konventionen, Codegenerierung, Gate), docs/api/README.md (Befehle).
* Die feste Bau-Vorgabe des Owners "API-Doku entsteht aus dem Code (OpenAPI aus den Routen)" gilt
  hier als "eine Quelle": Quelle ist `docs/api/openapi.yaml`, Routen, Handler-Schnittstellen und
  Clients entstehen daraus, und `make codegen-check` hält beide gleich. Keine Bau-Sitzung erzeugt
  OpenAPI aus Handler-Code; Ziel der Vorgabe (keine Abweichung zwischen Doku und Routen) ist damit
  erfüllt.
* oapi-codegen v2.8.0 Release Notes: https://github.com/oapi-codegen/oapi-codegen/releases
  (abgerufen 2026-10-10) [belegt].
* dart-dio Generator: https://openapi-generator.tech/docs/generators/dart-dio/ (abgerufen
  2026-10-10) [belegt].
