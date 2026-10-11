---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team B (API)
informed: alle Teams
---

# API-Versionierung und Deprecation-Policy

## Context and Problem Statement

Apps kommen über Stores und Auto-Update, Server betreibt jeder Owner selbst. Neue Apps treffen
also alte Server und umgekehrt. Plugins und Adapter Dritter nutzen dieselbe API. Wie versionieren
wir REST, Gateway und Ereignisse, und wie lange bleibt Altes bestehen?

## Decision Drivers

* Unabhängige Updates von Apps, Servern, Adaptern und Plugins.
* Einfache Regel für generierte Clients und für Proxys und Logs.
* Brüche selten, angekündigt und maschinell erkennbar.
* Keine festen Fristen im Fließtext; Fristen sind benannte Werte.

## Considered Options

* Hauptversion im Pfad (`/api/v1`), innerhalb nur additive Änderungen.
* Version per Kopfzeile oder Medientyp (`Accept: application/vnd...+json;version=2`).
* Datumsversionen je Client (Server übersetzt zwischen Ständen).

## Decision Outcome

Chosen option: "Hauptversion im Pfad", because sie für alle Generatoren, Proxys und Logs sichtbar
ist und zusammen mit strikter Additivität die unabhängigen Updates trägt, ohne Übersetzungsschichten
im Server.

Regeln (Einzelheiten in docs/spec/06-api.md, Abschnitt 13):

* `info.version` beider Verträge ist SemVer; MINOR für additive Änderungen; `0.x` bis zur ersten
  Veröffentlichung.
* Additiv: neue Endpunkte, Ops, Ereignistypen, Antwortfelder, optionale Anfragefelder, neue
  Enum-Werte in Antworten, neue Problem-Codes. Clients ignorieren Unbekanntes.
* Bruch: entfernen, umbenennen, Typ ändern, Pflicht verschärfen, Statuscode, Paginierung oder
  Aktion einer Op ändern. Nur in einer neuen Hauptversion.
* Deprecation: `deprecated: true` mit `x-deprecated-since` und `x-sunset` in der Spec; Server sendet
  `Deprecation` (RFC 9745), `Sunset` (RFC 8594) und `Link rel="deprecation"`. `Sunset` liegt
  mindestens `api.deprecation_min_notice` nach `Deprecation`.
* Server-Releases mit `v2` bedienen `v1` weiter für mindestens `api.major_version_overlap`.
* Gateway: Protokollversion in `hello` und `identify`, `4007` bei fehlender Unterstützung;
  Ereignis-Nutzlasten nur additiv, `schema_version` zählt mit.
* Sicherheitslücken dürfen Verhalten sofort ändern; das steht im Sicherheitshinweis.

### Consequences

* Good, because ein Client an einer Stelle erkennt, welche Hauptversion er spricht.
* Good, because neue Funktionen ohne Absprache zwischen App- und Server-Releases kommen.
* Bad, because ein Bruch an einer Stelle eine ganze neue Hauptversion braucht; deshalb fügen wir
  lieber neue Endpunkte hinzu, als bestehende zu ändern.
* Bad, because "neue Enum-Werte sind additiv" nur gilt, wenn jeder Client sie toleriert; für
  Python braucht das eine Handschicht (Probe vom 2026-10-10, openapi-python-client 0.29.1 wirft
  bei unbekannten Werten).

### Confirmation

Gate G5 (Toleranz-Test je SDK mit unbekanntem Feld, Enum-Wert und Ereignistyp) und G6
(Bruch-Prüfung gegen die zuletzt veröffentlichte Spec). Eine Lint-Regel prüft später, dass
`x-sunset` mindestens `api.deprecation_min_notice` nach dem Datum von `x-deprecated-since` liegt
(Ticket B-1). Werkzeug für G6 ist oasdiff, sobald Ticket B-6 seine OpenAPI-3.1-Tauglichkeit belegt.

## Pros and Cons of the Options

### Hauptversion im Pfad

* Good, because sichtbar und einfach zu routen.
* Good, because generierte Clients bekommen die Version über die Server-URL.
* Bad, because die Granularität grob ist.

### Version per Kopfzeile oder Medientyp

* Good, because URLs bleiben gleich.
* Bad, because unsichtbar in Logs und für viele Generatoren nicht abbildbar.
* Bad, because Caches und Proxys müssen `Vary` beachten.

### Datumsversionen je Client

* Good, because sehr feine Entwicklung ohne große Sprünge.
* Bad, because der Server muss jede alte Fassung in Übersetzungen tragen; für ein selbst gehostetes
  Projekt ohne zentrale Betriebsmannschaft zu teuer.

## More Information

* RFC 9745 (Deprecation): https://www.rfc-editor.org/rfc/rfc9745.html (abgerufen 2026-10-10)
  [belegt].
* RFC 8594 (Sunset): https://www.rfc-editor.org/rfc/rfc8594.html; Reihenfolge Sunset nicht vor
  Deprecation laut RFC 9745 [belegt].
* oasdiff: https://github.com/oasdiff/oasdiff (abgerufen 2026-10-10); OpenAPI-3.1-Stand
  [vermutet].
