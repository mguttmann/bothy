# M1-03: API contract checks in the gate (G2, G3)

Status: ready | Depends on: M1-02 | Etappe: M1

## Goal

After this ticket every copy between chapters 04, 05 and 06, `docs/spec/actions.yaml`,
`docs/api/openapi.yaml`, `docs/api/asyncapi.yaml` and `docs/api/webhooks.md` is checked in the
gate against its source. Rule `api-catalogue-sync` (G2) compares actions, rate-limit classes, ops,
events, receive actions, close codes, idempotency exemptions, deprecation dates and problem codes;
rule `api-tables` (G3) regenerates every `BEGIN/END GENERATED` block of chapter 06 and
`webhooks.md` and fails on any difference. To make that possible, the facts that today live only
in prose get a machine-readable home in `openapi.yaml`: the problem codes with their status, the
reasons for operations without `Idempotency-Key`, and the deprecation date.

## Read first

- `docs/PLAN.md`: section "Arbeitsweise je Ticket"; section "M1 Fundament" (Block A, row M1-03;
  "Exit-Kriterien" item 6; "Teststrategie").
- `docs/spec/06-api.md`: "Fehler" (5, with block `problems`), "Idempotenz" (7, item 3),
  "Versionierung, Deprecation, Kompatibilität" (13, item 4), "Codegenerierung" (14),
  "Vertragstests und Gate" (15, rows G1 to G3), "Endpunkte" (16, block `endpoints`), "WebSocket-Ops
  und Ereignisse" (17, blocks `ws_ops`, `events`), "Abgleich mit dem Kern-Vertrag" (18),
  "Entscheidungen dieses Kapitels" (19), "Konfigurationswerte dieses Kapitels" (20,
  `api.deprecation_min_notice`).
- `docs/spec/04-sync-protokoll.md`: "Schließ-Codes" (6.4), "Alle anderen Schreibvorgänge:
  `Idempotency-Key`" (5.2).
- `docs/spec/05-aktionen.md`: "Scopes für Apps" (2.6), "Rate-Limit-Klassen", "Dauerhafte
  Ereignistypen und Empfangs-Aktion", "WebSocket-Ops und Aktion", "Tests, die diesen Katalog
  halten" (item 7).
- `docs/spec/actions.yaml` and `tools/catalog/action_catalog.py` with `tools/catalog/README.md`
  (from M1-02; use `load_catalog`, `generated_blocks`, `replace_generated_block`).
- `docs/api/openapi.yaml`: `components.responses` (problem code lists in the descriptions),
  `components.parameters.IdempotencyKey`, `components.schemas.Problem`, `paths`, `webhooks`,
  `tags`. `docs/api/asyncapi.yaml`: `channels.gateway.x-close-codes`, `operations`,
  `components.messages`. `docs/api/webhooks.md` section 7 (block `webhook_events`).
  `docs/api/README.md` ("Ändern"), `docs/api/redocly.yaml`.
- `docs/operations/configuration.md` section "Benannte Werte aller Kapitel" (row
  `api.deprecation_min_notice`).
- ADRs: `docs/adr/0013-spec-first-openapi-31-und-asyncapi-30-normativ.md`,
  `docs/adr/0014-api-versionierung-und-deprecation-policy.md`,
  `docs/adr/0015-fehlerformat-rfc-9457-idempotenz-paginierung.md`,
  `docs/adr/0016-webhooks-nach-standard-webhooks.md`, ADR 0049 (from M1-02).
- `docs/quality/gates.md`, `docs/spec/23-qualitaet.md` "Gates", `tools/README.md`,
  `tools/pyproject.toml`, `Makefile`, `CHANGELOG.md`, `docs/DECISIONS.md`.

Operations touched: every operation in `paths` and `webhooks` is read; `suspendApp` and
`resumeApp` gain the `Idempotency-Key` parameter; every POST or PATCH without that parameter gains
`x-idempotency-exempt` (list in "Design notes"). No table of chapter 03 and no action id changes.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching documentation
   is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour gets a test that is red before the change (show the red output in
   the report), then green.
7. No en dash (U+2013) and no em dash (U+2014) in any file or commit message. Check every changed
   file with `grep -nP "[\x{2013}\x{2014}]" <file>` (must print nothing).
8. White label: no company, person or host names, no real domains (only example.com and
   example.org), no real IP addresses, no absolute paths into a home directory, no internal
   system names, in any file including tests and fixtures.
9. Limits are named configuration values from `docs/operations/configuration.md`; the deprecation
   check reads `api.deprecation_min_notice` from there and never hard-codes its value.
10. G2 and G3 run in the one local gate command identical to CI (06 "Vertragstests und Gate"
    item 3, ADR 0038); the tool uses Python with PyYAML only.
11. Every operation names exactly one known action and the class of that action; system actions
    never appear on a route or op (05 rules 3, 4, 10; test 7).
12. Every POST and PATCH accepts `Idempotency-Key` except the cases of 06 "Idempotenz" item 3,
    and the gate holds that list (06 G2; 04 5.2; R-API-09).
13. A deprecated op or field carries `x-deprecated-since`, a deprecation date and `x-sunset`, with
    `Sunset` at least `api.deprecation_min_notice` after `Deprecation` (06 13.4, RFC 9745,
    RFC 8594).
14. Descriptions in `openapi.yaml` and `asyncapi.yaml` are English (Ruling L5, R-API-15);
    chapters stay German.
15. Additive API changes raise `info.version` MINOR, documentation-only changes PATCH (06 13.1).

## Scope

1. `tools/api-checks/api_checks.py` (new; Python 3.12, standard library plus PyYAML; imports
   `action_catalog` from `tools/catalog`). CLI:
   - `api_checks.py check [--root <repo root>] [--rule api-catalogue-sync|api-tables]` (default:
     both rules; exit 0 clean, 1 findings).
   - `api_checks.py generate [--root <repo root>]` (rewrites the generated blocks; refuses with
     exit 1 if a source file is invalid).
   Public interface with docstrings: `def check_catalogue_sync(root: Path) -> list[str]`,
   `def check_tables(root: Path) -> list[str]`, `def render_blocks(root: Path) -> dict[Path,
   dict[str, str]]`, `def generate(root: Path) -> list[Path]`, `def main(argv) -> int`. Every
   finding line starts with the rule id and the sub-check, for example
   `api-catalogue-sync: rest-action: <operationId>: ...`.
2. `tools/api-checks/tests/test_api_checks.py` (mutation fixtures, see "Tests").
   `tools/api-checks/README.md` (German, short).
3. `docs/api/openapi.yaml`:
   - `components.schemas.Problem` gains `x-problem-codes`: a list of entries `{code, status,
     title, description, client_action}` (English), one per row of today's 06 table "Bekannte
     Codes", same order (sorted by status, stable), with optional `router: true` for codes the
     router emits without any operation documenting them (`method_not_allowed`). Translate
     today's German cells faithfully; `title` is the short RFC 9457 title (for example
     `Validation failed`).
   - Every entry of `components.responses` gains `x-status` (the HTTP status it stands for:
     `BadRequest` 400, `Unauthorized` 401, `Forbidden` 403, `NotFound` 404, `Conflict` 409, `Gone`
     410, `PreconditionFailed` 412, `PayloadTooLarge` 413, `UnsupportedMediaType` 415,
     `RangeNotSatisfiable` 416, `UnprocessableEntity` 422, `PreconditionRequired` 428,
     `TooManyRequests` 429, `Error` 500, `ServiceUnavailable` 503). Reason: `Error` is referenced
     only as `default` and `ServiceUnavailable` nowhere, so the status of their codes cannot be
     read from `paths`.
   - Every POST and PATCH in `paths` without the `IdempotencyKey` parameter gets
     `x-idempotency-exempt: <reason>` (list in "Design notes"); `suspendApp` and `resumeApp` get
     `- $ref: '#/components/parameters/IdempotencyKey'` instead.
   - `info.version` raised by one MINOR step (new optional header on two operations).
4. `docs/api/asyncapi.yaml`: rewrite every `client_action` of `channels.gateway.x-close-codes` in
   English, faithful to 04 "Schließ-Codes" (this also fixes the drift of code `4016`, whose text
   differs from 04 today); `info.version` raised by one PATCH step.
5. `docs/spec/06-api.md`:
   - 5 "Fehler": the sentence before the block says the block is generated from `x-problem-codes`
     in `openapi.yaml`; block `problems` gets the header `| Code | Status | Bedeutung (aus
     openapi.yaml) | Client-Reaktion (aus openapi.yaml) |`.
   - 7 "Idempotenz" item 3: replace the prose list by: "Ohne `Idempotency-Key` bleiben genau die
     POST- und PATCH-Operationen mit `x-idempotency-exempt` in openapi.yaml; der Grund steht als
     Code daran. Das Gate prüft die Liste (Abschnitt 15, G2):" followed by the new block
     `idempotency_exempt`.
   - 13 "Versionierung, Deprecation, Kompatibilität" item 4: add `x-deprecated-at` (date of the
     deprecation, RFC 3339 full-date, source of the `Deprecation` header) next to
     `x-deprecated-since` (spec version) and `x-sunset` (date).
   - 15: rows G2 and G3 describe the real rules and the tool (`tools/api-checks`,
     `make api-checks`); G2 no longer mentions the redocly list (that is `catalog-check` of
     M1-02); G3 names sections 5, 7, 16 and 17 and `webhooks.md` (18.1 is no generated table).
   - 19: new rulings `R-API-23` (problem codes have one source, `x-problem-codes`, and every
     response component names its status in `x-status`), `R-API-24` (exemptions from
     `Idempotency-Key` are machine-readable with a reason code; `suspendApp` and `resumeApp` take
     the key), `R-API-25` (`x-deprecated-at` carries the deprecation date because
     `x-deprecated-since` is a spec version, not a date).
   - Blocks `endpoints`, `ws_ops`, `events` are regenerated (no content change expected).
6. `docs/api/webhooks.md`: block `webhook_events` regenerated (no content change expected).
7. `docs/api/README.md` "Ändern": item 2 names `x-idempotency-exempt` or the `IdempotencyKey`
   parameter and new problem codes in `x-problem-codes`; item 5 names `make api-tables` and
   `make api-checks`.
8. `Makefile`: `api-checks` (`## API contract copies equal their sources (06 G2, G3)`) in `gates`
   directly after `api-lint`; `api-tables` (`## Regenerate the generated tables of chapter 06 and
   webhooks.md`); bandit directory list of `lint` gains `tools/api-checks`; `.PHONY`.
9. `tools/pyproject.toml`: add `api-checks` to `testpaths`, `pythonpath`, coverage `source` and
   mypy `files`.
10. `docs/quality/gates.md` (row `api-checks`), `docs/spec/23-qualitaet.md` "Gates" group "Doku"
    (`api-checks`), `tools/README.md` (row), `CHANGELOG.md` `[Unreleased]`, `docs/DECISIONS.md`
    (rulings below).

## Out of scope

- The action catalogue, the redocly enums and the tables of chapter 05: M1-02 (`catalog-check`).
- G4 generated clients and G5 tolerance tests: M2 and M4 with the SDKs. G6 break check: M7.
- G7 and G8 Schemathesis and response validation: M1-71. G9 gateway frames: M1-67. G10 webhook
  conformance: M2.
- Server code that uses `x-problem-codes`, `x-status`, `x-idempotency-exempt` or
  `x-deprecated-at`: M1-17 and M1-18. This ticket only defines and checks them.
- The operation allow-list `services/server/api/operations.yaml`: M1-17.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-03-api-contract-checks`.
- Conventional Commits with DCO sign-off (`git commit -s`), for example `build(api-checks): ...`
  and `docs(api): ...`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `docs/DECISIONS.md` and `CHANGELOG.md` as
  named. Do not edit `docs/spec/actions.yaml` or chapter 05; if the catalogue is wrong, stop with
  `Verdict: BLOCKED` and name the difference (P-11).
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
  Temporary directories only via `mktemp -d` or pytest `tmp_path`, removed at the end.
- Never read or print secrets. The launcher provides the private deny list; do not set
  `DENYLIST_FLAGS=--allow-missing` or `SONAR_SKIP=1`.
- No network calls except package downloads for pinned tools.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

### Rule `api-catalogue-sync` (G2), sub-checks

Load the catalogue with `action_catalog.load_catalog(root / "docs/spec/actions.yaml")`.

| Sub-check | Holds |
|---|---|
| `rest-action` | every operation in `paths`: `x-authz-action` is a known, non-system action; `x-rate-limit-class` equals that action's `rl_class` (`none` stays `none`) |
| `webhook-events` | every entry in `webhooks`: the key is a catalogue event type; `x-receive-action` equals its `receive_action`; `x-authz-action` is `webhook_out.deliver`; `x-rate-limit-class` is `none` |
| `ws-ops` | the set of asyncapi operations equals the catalogue ops: an operation with one message matches the op whose `op` equals the message `name`; the operation with many messages is `dispatch`; asyncapi `action: receive` matches a direction ending `_to_server`, `send` matches `server_to_client`; `x-authz-action` equals the op's `action`; `x-rate-limit-class` equals the class of that action |
| `events` | the set of asyncapi messages with `x-event-kind` equals the catalogue events (compare as sets; order may differ); per event: `x-event-kind` equals `kind`, `x-receive-action` equals `receive_action`, `x-audience: self` exactly when `audience: self`; the `dispatch` operation lists exactly these messages |
| `close-codes` | `channels.gateway.x-close-codes` lists the same `(code, name)` pairs in the same order as the table in 04 "Schließ-Codes" (code as integer, name verbatim); `client_action` is not compared because 04 is German and the asyncapi text English (L5) |
| `idempotency` | every POST and PATCH in `paths` has either the `IdempotencyKey` parameter (operation or path level, by `$ref` or by `name: Idempotency-Key, in: header`) or `x-idempotency-exempt` with a known reason, never both and never neither; no other method carries `x-idempotency-exempt` |
| `deprecation` | in both API files, every mapping with `deprecated: true` has `x-deprecated-since` (SemVer), `x-deprecated-at` and `x-sunset` (both `YYYY-MM-DD`), and `x-sunset - x-deprecated-at >= api.deprecation_min_notice`; any of the three `x-` keys without `deprecated: true` is a finding |
| `problem-codes` | `x-problem-codes` has unique codes, a 3-digit `status`, non-empty English fields, and is sorted by status; every `components.responses` entry has `x-status`; every reference to it in `paths` uses that status as key, or `default` when `x-status` is 500 or higher; every backticked code in the description of a response is in `x-problem-codes` with the response's `x-status`; every entry without `router: true` is named by at least one response |

Reading `api.deprecation_min_notice`: find the Markdown table in
`docs/operations/configuration.md` whose header has the columns `Schlüssel` and `Standard`, take
the row whose first cell is `` `api.deprecation_min_notice` ``, strip backticks from the
`Standard` cell; durations accept the units `s`, `m`, `h`, `d`. A missing or duplicate row is a
finding, never a default. Locate columns by header name, because M1-08 may add columns.

### Exemption reasons (`x-idempotency-exempt`)

| Code | German text in block `idempotency_exempt` | Operations (state at planning) |
|---|---|---|
| `auth_flow` | Anmeldeablauf: einmalige Challenge oder Code, Refresh-Rotation | `beginOidcLogin`, `completeOidcLogin`, `loginWithTrustedHeader`, `beginPasskeyLogin`, `completePasskeyLogin`, `beginEnrollment`, `completeEnrollment`, `beginPairing`, `completePairing`, `loginWithPassword`, `refreshToken`, `logout`, `createStepUpChallenge`, `completeStepUpWithPasskey`, `completeStepUpWithDeviceKey`, `beginIdentityLink`, `completeIdentityLink`, `beginPasskeyRegistration`, `completePasskeyRegistration` |
| `read_only` | liest nur, ändert keinen dauerhaften Zustand | `previewGrant`, `autocompleteCommand`, `validateManifest`, `verifyAuditChain`, `createFileTicket` |
| `agent_runtime` | Agenten-Laufzeit: monoton oder Compare-and-swap | `sendHeartbeat`, `ackAgentEvents`, `patchRun`, `startRun`, `resumeRun`, `awaitRunInput`, `finishRun`, `answerAutocomplete` |
| `client_id` | Client-ID im Körper (Tabelle oben) | `createMessage`, `createInteraction`, `postIncomingWebhook` |
| `origin_key` | Herkunftsschlüssel des Imports ([22-import-migration.md](22-import-migration.md)) | `applyImportBatch`, `createImportFile` |
| `telemetry` | anonyme Messwerte; eine Wiederholung zählt eine Stichprobe doppelt | `submitClientMetrics` |

`suspendApp` and `resumeApp` change state with audit and fit no reason; they get the
`IdempotencyKey` parameter. If the base branch has other POST or PATCH operations without the
header, give them the header unless one reason clearly applies, and list each decision in the
report. The German texts live as a constant mapping in `api_checks.py` because they render a
German chapter; comments and docstrings stay English.

### Rule `api-tables` (G3), blocks

The tool owns exactly these blocks; any other `BEGIN GENERATED` name in the two files is a
finding. Use `action_catalog.generated_blocks` and `replace_generated_block`. Rows render as
`"| " + " | ".join(cells) + " |"` (an empty cell becomes two spaces, as in the current blocks).

| File | Block | Source and rows |
|---|---|---|
| 06 | `problems` | `x-problem-codes` in list order: `` `code` ``, status, `description`, `client_action` |
| 06 | `idempotency_exempt` | header `\| operationId \| Methode \| Pfad \| Grund \|`; operations with `x-idempotency-exempt`, ordered by the reason order of the table above, then by order in `paths`; path as `` `/api/v1<path>` ``; reason as German text |
| 06 | `endpoints` | for each tag in `tags` order that has operations: `#### <tag>`, blank line, header `\| Methode \| Pfad \| operationId \| Aktion \| Zweck (aus openapi.yaml) \|`, separator, rows `\| <METHOD> \| \`/api/v1<path>\` \| \`<operationId>\` \| \`<x-authz-action>\` \| <summary> \|` in `paths` order; groups separated by one blank line |
| 06 | `ws_ops` | header `\| Op \| Richtung \| Aktion \| Körper-Schema \| Antwort \|`; one row per asyncapi operation in file order: message `name`, `Client an Server` for `receive` and `Server an Client` for `send`, `x-authz-action`, last segment of the `$ref` of `payload.properties.d`, reply message name or empty; the operation with many messages renders the fixed row `` \| `dispatch` \| Server an Client \| Empfangs-Aktion des Ereignistyps \| `<Typ>Event` \| \| `` |
| 06 | `events` | header `\| Ereignistyp \| Art \| Empfangs-Aktion \| Zielgruppe \| Schema \| Webhook \|`; messages in the order of the `dispatch` operation: `name`, `dauerhaft` or `kurzlebig`, `x-receive-action`, `x-audience` or empty, payload schema name, `ja` if the type is a key of `webhooks`, else empty |
| `webhooks.md` | `webhook_events` | header `\| Ereignistyp \| Empfangs-Aktion \| Empfänger und Scope zusätzlich zu \`events:subscribe\` \| Schema \|`; entries of `webhooks` in file order: type, `x-receive-action`, scope cell, schema of the request body. Scope cell: event with `audience: self` and `receivers: agent` gives `nur Agenten-Abo (Agent ist Empfänger)`; other `self` events give `nur eigene (App oder Agent ist Empfänger)`; otherwise the `app_scope` of the receive action in backticks, or `keiner (Metadaten, 05 Abschnitt 2.6)` for `none` |

Prototype runs on the planning state reproduced `endpoints`, `ws_ops`, `events` and
`webhook_events` byte for byte with these rules; a difference after implementation is a bug in
the renderer, not a reason to edit the block by hand.

### Rulings for `docs/DECISIONS.md`

1. Problem codes get one machine-readable source (`x-problem-codes` on `Problem`); the 06 table
   is generated from it in English like the endpoint summaries. Reason: a German table cannot be
   generated from an English contract (L5), and the server needs titles and descriptions for
   `getProblemType` and problem bodies.
2. Exemptions from `Idempotency-Key` carry a reason code; `suspendApp` and `resumeApp` take the
   key. Reason: the prose list of 06 7.3 did not cover all operations, so the G2 check had no
   source.
3. `x-deprecated-at` added. Reason: `x-deprecated-since` is a spec version, and the sunset rule
   and the `Deprecation` header need a date.
4. Close codes are compared by code and name only; asyncapi texts are English. Reason: L5; the
   meaning stays in 04.
5. Every response component carries `x-status`. Reason: two components are not referenced under
   a numeric status, and problem codes need a status from one place.

## Tests (write them first)

`tools/api-checks/tests/test_api_checks.py`. Each negative test copies the real inputs
(`docs/spec/actions.yaml`, `docs/spec/04-sync-protokoll.md`, `docs/spec/05-aktionen.md`,
`docs/spec/06-api.md`, `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `docs/api/webhooks.md`,
`docs/operations/configuration.md`) into `tmp_path`, applies one mutation (a small helper edits
the YAML through PyYAML and writes it back, or edits text), runs the rule and asserts a finding
with the named sub-check:

- `test_repository_is_clean` (both rules, real repository, no findings).
- `rest-action`: `test_unknown_action_fails`, `test_system_action_on_route_fails`,
  `test_rate_limit_class_mismatch_fails`.
- `webhook-events`: `test_webhook_receive_action_mismatch_fails`,
  `test_webhook_for_unknown_event_fails`.
- `ws-ops`: `test_ws_op_action_mismatch_fails`, `test_ws_op_missing_in_asyncapi_fails`,
  `test_ws_direction_mismatch_fails`.
- `events`: `test_event_receive_action_mismatch_fails`, `test_event_audience_mismatch_fails`,
  `test_event_kind_mismatch_fails`, `test_event_missing_from_dispatch_fails`.
- `close-codes`: `test_close_code_name_mismatch_fails`, `test_close_code_missing_fails`.
- `idempotency`: `test_post_without_key_or_exemption_fails`, `test_unknown_exemption_reason_fails`,
  `test_key_and_exemption_together_fails`, `test_exemption_on_get_fails`.
- `deprecation`: `test_deprecated_without_sunset_fails`, `test_sunset_too_early_fails`,
  `test_sunset_exactly_min_notice_passes`, `test_sunset_without_deprecated_fails`,
  `test_missing_config_value_fails`.
- `problem-codes`: `test_code_in_response_missing_from_source_fails`,
  `test_problem_status_mismatch_fails`, `test_unused_problem_code_fails`,
  `test_router_code_may_be_unused`, `test_response_without_x_status_fails`,
  `test_response_referenced_under_other_status_fails`.
- `api-tables`: `test_edited_row_fails` (one test per block: `problems`, `idempotency_exempt`,
  `endpoints`, `ws_ops`, `events`, `webhook_events`), `test_unknown_block_name_fails`,
  `test_generate_is_idempotent`.
- `test_cli_exit_codes`.

Commands that must pass:

```
uv run --project tools --locked pytest tools/api-checks/tests
make api-tables && git diff --exit-code   # regeneration gives no diff
make api-checks
make api-lint
make check
```

## Acceptance criteria

- [ ] `pytest tools/api-checks/tests`: for every sub-check of `api-catalogue-sync` and for every
      block of `api-tables` one mutated fixture is red; the repository is green.
- [ ] `make api-checks` and `make api-lint` are green; `make api-tables` gives no diff.
- [ ] `x-problem-codes`, `x-status`, `x-idempotency-exempt` and `x-deprecated-at` are defined in
      06, used in `openapi.yaml` and checked; `suspendApp` and `resumeApp` take `Idempotency-Key`.
- [ ] asyncapi close-code texts are English and match 04 in meaning; versions of both API files
      raised as stated.
- [ ] 06 sections 5, 7, 13, 15 and 19 updated; `docs/api/README.md`, `docs/quality/gates.md`,
      chapter 23, `tools/README.md`, `tools/api-checks/README.md`, `CHANGELOG.md`,
      `docs/DECISIONS.md` updated.
- [ ] `make check` green; no file of the diff contains U+2013 or U+2014.

## Definition of done

Code, tests and docs as above: docstrings on every public function of `api_checks.py`; README of
the tool; no new ADR (the decisions refine ADRs 0014 and 0015 and are recorded as rulings in 06
and `docs/DECISIONS.md`); CHANGELOG entry under `[Unreleased]`; `make check` green. Then
`make review` and an independent code review over the diff, then the red team gate
(`docs/quality/red-team-gate.md`) over the diff with verdict recorded. Fix every finding with a
test and re-check the delta. Merge locally into the base branch only if the review is clean, the
red team says `JA` and `make check` is green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red runs), the
list of operations that got `x-idempotency-exempt` or the header with the reason, what is not
proven (for example: no server reads the new `x-` fields before M1-17 and M1-18), and follow-ups
that could not be done in scope, each with its reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<merged sha>` or `exit=1 <reason>`.
