# API-Verträge

Die Dateien in diesem Ordner sind normativ (spec-first, ADR 0013). Server-Stubs, SDKs und das
Dart-Paket der Apps werden daraus erzeugt. Konventionen: [docs/spec/06-api.md](../spec/06-api.md).

| Datei | Inhalt |
|---|---|
| [openapi.yaml](openapi.yaml) | REST-API unter `/api/v1` (OpenAPI 3.1), Webhook-Ereignisse (`webhooks:`), alle Schemas, auch die des Gateways |
| [asyncapi.yaml](asyncapi.yaml) | Gateway-WebSocket (AsyncAPI 3.0): Ops, Ereignisse, Schließ-Codes; Körper per `$ref` auf openapi.yaml |
| [webhooks.md](webhooks.md) | Zustellung, Signatur, Wiederholung und Prüfung von Webhooks; Eingangs-Webhooks |
| [redocly.yaml](redocly.yaml) | Lint-Regeln für beide Dateien, inklusive Pflicht von `x-authz-action` und Abgleich mit dem Aktionskatalog |

## Prüfen

Alle Befehle aus dem Wurzelordner des Repos. Redocly CLI 2 braucht Node ab 22.12
(https://redocly.com/docs/cli/, abgerufen 2026-10-10 [belegt]).

```sh
# beide APIs aus redocly.yaml (Gate G1, docs/spec/06-api.md Abschnitt 15)
npx -y @redocly/cli@2.62.1 lint --config docs/api/redocly.yaml

# nur eine Datei, mit denselben Regeln
npx -y @redocly/cli@2.62.1 lint docs/api/openapi.yaml --config docs/api/redocly.yaml

# zweite Meinung zu AsyncAPI; CI=true schaltet die Nutzungsstatistik der AsyncAPI-CLI ab
CI=true npx -y @asyncapi/cli@6.2.0 validate docs/api/asyncapi.yaml
```

Erwartet: Redocly ohne Fehler und Warnungen; AsyncAPI-CLI meldet nur den Hinweis, dass es
AsyncAPI 3.1.0 gibt. Ohne `--config` nimmt Redocly nur seine Standardregeln; die Regeln aus
05-aktionen.md greifen dann nicht.

Die Versionen sind gepinnt; Renovate hebt sie an (ADR 0041). Wer eine Regel in redocly.yaml
ändert, prüft sie mit einer absichtlich falschen Kopie der Spec, ob sie anschlägt.

## Reference in the docs site

`make docs` renders three reference pages from these documents in every build: the REST
reference (every operation by tag, the webhook events, security schemes, shared responses and
headers), the schemas of `openapi.yaml` and the gateway reference (servers, channels with close
codes, operations and messages of `asyncapi.yaml`). In the site they are under API as
"REST reference", "Schemas" and "Gateway reference" (`api/reference/rest/`,
`api/reference/schemas/`, `api/reference/gateway/`); webhook delivery stays in
[webhooks.md](webhooks.md), to which the REST reference links.

The pages exist only in the build: the MkDocs hook
[tools/docs/mkdocs_api_reference.py](../../tools/docs/mkdocs_api_reference.py) generates them,
and a committed copy under `docs/api/reference/` fails the build. They are static Markdown and
need no browser script; [tools/docs/external_assets_check.py](../../tools/docs/external_assets_check.py)
fails `make docs` when a built page loads a script, stylesheet or font from another origin.

## Ändern

1. Erst die Spec ändern, dann den Code. Additiv oder Bruch entscheidet 06-api.md, Abschnitt 13.
2. Neue Operation: `operationId` in lowerCamelCase, `tags`, `x-authz-action` aus 05-aktionen.md,
   `x-rate-limit-class` der Aktion, Problem-Antworten aus `components.responses`, Beispiel im
   Medientyp. Jede POST- und PATCH-Operation bekommt den Parameter `IdempotencyKey` oder
   `x-idempotency-exempt` mit einem Grund-Code aus 06-api.md, Abschnitt 7. Ein neuer Problem-Code
   kommt zuerst in `x-problem-codes` am Schema `Problem` und in die Beschreibung der
   Antwort-Komponente, deren `x-status` sein Status ist. Fehlt eine passende Aktion, kommt sie
   zuerst in [docs/spec/actions.yaml](../spec/actions.yaml); `make catalog` erzeugt daraus die
   Tabellen in 05-aktionen.md und die Listen in redocly.yaml. Im selben Commit bekommt die
   Operation einen Eintrag unter `later` in `services/server/api/operations.yaml`
   (Freischalt-Liste, PLAN P-04).
3. Neues Ereignis: Typ und Empfangs-Aktion zuerst in [docs/spec/actions.yaml](../spec/actions.yaml),
   dann `make catalog`; Nutzlast-Schema `<Name>Payload`,
   Ereignis-Schema `<Name>Event` als `allOf` aus `EventEnvelope`, Eintrag in `Event` (dauerhaft),
   Nachricht in asyncapi.yaml, bei Bedarf Eintrag unter `webhooks:`.
4. Kein `null`, keine Typ-Arrays, kein `anyOf`, `oneOf` nur mit Diskriminator. Wiederholte
   Enum-Mengen als benanntes Schema.
5. Lint, Code neu erzeugen, Tests. Regenerate both SDK cores with `make -C sdk/python generate` and
   `make -C sdk/typescript generate` (otherwise `make api-clients` fails, G4).
   `make api-tables` erneuert die erzeugten Tabellen in
   06-api.md und webhooks.md (Markierungen `BEGIN GENERATED`); `make api-checks` prüft im Gate,
   dass sie und alle Kopien der Verträge zu ihren Quellen passen (G2, G3,
   [tools/api-checks](../../tools/api-checks/README.md)).
