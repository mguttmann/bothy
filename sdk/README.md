# sdk

Status: geplant. Dieser Ordner enthält noch keinen Code.

## Zweck

SDKs für Agenten-Adapter, Apps und Werkzeuge, die die öffentliche API nutzen. Je Sprache ein
generierter Kern aus der Spezifikation plus eine Handschicht für Gateway, Aufruf-Marke, Läufe,
Komponenten, Befehle, Idempotenz, Wiederholungen, Webhook-Signaturen und Testdoppel (ADR 0026).

| Ordner | Sprache | Stand |
|---|---|---|
| `python/` | Python | geplant |
| `typescript/` | TypeScript | geplant |

Clients der Apps (Dart) entstehen ebenfalls aus der Spezifikation, gehören aber zu den Apps
(docs/spec/15-client-architektur.md).

## Vertrag

- Design: docs/spec/11-erweiterungen.md Abschnitt 11.
- Agenten-Vertrag, den die Handschicht umsetzt: docs/spec/09-agenten.md.
- Quelle der Wahrheit: `docs/api/openapi.yaml` und `docs/api/asyncapi.yaml`.

## Versionierung

SemVer je Sprache, eigenes Changelog je Ordner. Regeln: docs/spec/11-erweiterungen.md
Abschnitt 12; Deprecation nach docs/spec/06-api.md.

## Kompatibilitätsmatrix

Noch keine veröffentlichte Version. Jede Zeile bekommt einen Test im Gate, der die Vertragstests
der SDK-Version gegen die genannte Server-Version laufen lässt.

| SDK | Version | API | Gateway-Protokoll | Server mindestens | Status |
|---|---|---|---|---|---|
| Python | noch keine | | | | |
| TypeScript | noch keine | | | | |

Status-Werte: `supported`, `deprecated`, `end-of-life`.

## Paketnamen

Die Namen bei PyPI und npm hängen am endgültigen Produktnamen und sind noch nicht reserviert
(Frage an den Owner).
