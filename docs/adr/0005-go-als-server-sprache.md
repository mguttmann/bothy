---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team Architektur
informed: alle Planungsteams
---

# Go als Server-Sprache

## Context and Problem Statement

Der Server hält viele dauerhafte WebSocket-Verbindungen, filtert jedes Ereignis je Empfänger,
schreibt in PostgreSQL und läuft auf einem Mac und in Linux-Containern (linux/amd64, linux/arm64).
OpenAPI und AsyncAPI sind normativ, Server-Stubs werden generiert (B2). Welche Sprache nutzt der
Server?

## Decision Drivers

* Nebenläufigkeit für viele Verbindungen in einem Prozess, ohne externen Broadcast-Dienst.
* Ein statisches Binary für kleine Multi-Arch-Images.
* Reife Bibliotheken für PostgreSQL, WebSocket, WebAuthn, Hintergrundjobs und OpenTelemetry.
* Codegenerierung aus OpenAPI 3.1.
* Eine Sprache, die viele Bau-Sitzungen sicher und gleichförmig schreiben (Formatierer, Vet,
  statische Typen).

## Considered Options

* Go
* Python mit FastAPI
* Elixir mit Phoenix

## Decision Outcome

Chosen option: "Go", because es Nebenläufigkeit und statische Typen in einem Binary bietet und für
jede benötigte Schicht eine gepflegte Bibliothek hat. `go.mod` setzt die ältere gepflegte Linie als
`go`-Direktive, gebaut wird mit der aktuellen Version (R-Stack).

### Consequences

* Good, because Goroutinen je Verbindung und ein Hub im Prozess das Fan-out ohne Broker tragen.
* Good, because ein Binary mit Rollen in ein kleines Image passt (B12).
* Good, because `gofmt`, `go vet` und statische Typen einheitlichen Code über viele Sitzungen hinweg
  erzwingen.
* Bad, because SDK und Hermes-Adapter in Python bleiben; das Team pflegt zwei Sprachen plus Dart.
* Bad, because Go-Versionen nur gepflegt werden, bis zwei neuere erschienen sind; Upgrades sind
  Pflicht (Renovate, ADR 0041).

### Confirmation

* Ein Gate-Check vergleicht die `go`-Direktive in `services/server/go.mod` mit der ältesten
  gepflegten Go-Version und die `toolchain`-Direktive mit der gebauten.
* Der Build erzeugt Images für linux/amd64 und linux/arm64 (18-release.md).

## Pros and Cons of the Options

### Go

* Good, because Go 1.27 und 1.26 gepflegt sind; aktuell ist go1.27.2 vom 2026-10-08. [belegt, Q1]
* Good, because der Standard-ServeMux seit Go 1.22 Methoden-Muster und Platzhalter kann. [belegt, Q2]
* Good, because coder/websocket (ISC) Kontexte durchgängig nutzt und gleichzeitige Schreibvorgänge
  erlaubt. [belegt, Q3]
* Good, because das OpenTelemetry-SDK für Go Traces und Metriken als stabil führt. [belegt, Q4]
* Good, because oapi-codegen Server für `net/http` mit Strict-Interface erzeugt. [belegt, Q7]
* Bad, because Logs im OTel-SDK für Go erst "Release candidate" sind. [belegt, Q4]

### Python mit FastAPI

* Good, because SDK und Hermes-Adapter schon Python sind.
* Bad, because der WebSocket-Broadcast laut Doku nur im Speicher eines Prozesses funktioniert; für
  mehrere Worker verweist sie auf einen externen Broadcast-Dienst. [belegt, Q5]
* Bad, because ein Python-Dienst kein statisches Binary ist; Images und Abhängigkeiten werden
  größer. [abgeleitet]

### Elixir mit Phoenix

* Good, because Phoenix Presence Prozessinformationen im Cluster ohne zentrale Stelle repliziert.
  [belegt, Q6]
* Bad, because eine weitere Sprache und Laufzeit dazukommt, für die im Team und in den Werkzeugen
  (Codegenerierung aus OpenAPI, Lizenz-Gates) weniger vorhanden ist. [abgeleitet]

## More Information

Quellen, abgerufen 2026-10-10:

* Q1 https://go.dev/doc/devel/release
* Q2 https://go.dev/blog/routing-enhancements
* Q3 https://github.com/coder/websocket (v1.8.15)
* Q4 https://opentelemetry.io/docs/languages/go/
* Q5 https://fastapi.tiangolo.com/advanced/websockets/
* Q6 https://phoenix.hexdocs.pm/presence.html
* Q7 https://github.com/oapi-codegen/oapi-codegen (v2.8.0)

Planungsgrundlage B2, R-Stack. [02-architektur.md](../spec/02-architektur.md), Abschnitte 5 und 6.
