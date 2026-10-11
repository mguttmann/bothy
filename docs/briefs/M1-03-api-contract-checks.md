# M1-03: API contract checks in the gate (G2, G3)

Status: ready | Depends on: M1-02 | Milestone: M1

## Goal

After this ticket every copy between chapters 04, 05 and 06, `docs/spec/actions.yaml`,
`docs/api/openapi.yaml`, `docs/api/asyncapi.yaml` and `docs/api/webhooks.md` is checked in the
gate against its source. Rule `api-catalogue-sync` (G2) compares actions, rate-limit classes, ops,
events, receive actions, close codes, idempotency exemptions, deprecation dates and problem codes;
rule `api-tables` (G3) regenerates every `BEGIN/END GENERATED` block of chapter 06 and
`webhooks.md` and fails on any difference. The machine-readable sources these rules read are
already in the specification (planning, spec first): `x-problem-codes` on `Problem`, `x-status` on
every response component, `x-idempotency-exempt` with a reason code, `x-deprecated-at`, and the
English close-code texts in `asyncapi.yaml` (06 sections 5, 7, 13.4, R-API-23 to R-API-25). This
ticket builds the tool that holds them.

## Read first

- `docs/PLAN.md`: section "Way of working per ticket"; section "M1 Foundation" (Block A, row
  M1-03; "Exit criteria" item 6; "Test strategy").
- `docs/spec/06-api.md`: "Errors" (5, with block `problems`), "Idempotency" (7, item 3),
  "Versioning, deprecation, compatibility" (13, item 4), "Code generation" (14),
  "Contract tests and gate" (15, rows G1 to G3), "Endpoints" (16, block `endpoints`),
  "WebSocket ops and events" (17, blocks `ws_ops`, `events`), "Alignment with the core contract"
  (18), "Decisions of this chapter" (19), "Configuration values of this chapter" (20,
  `api.deprecation_min_notice`).
- `docs/spec/04-sync-protocol.md`: "Close codes" (6.4), "All other writes: `Idempotency-Key`"
  (5.2).
- `docs/spec/05-actions.md`: "Scopes for apps" (2.6), "Rate-limit classes",
  "Durable event types and receive action", "WebSocket ops and action",
  "Tests that hold this catalogue" (item 7).
- `docs/spec/actions.yaml` and `tools/catalog/action_catalog.py` with `tools/catalog/README.md`
  (from M1-02; use `load_catalog`, `generated_blocks`, `replace_generated_block`).
- `docs/api/openapi.yaml`: `components.responses` (problem code lists in the descriptions),
  `components.parameters.IdempotencyKey`, `components.schemas.Problem`, `paths`, `webhooks`,
  `tags`. `docs/api/asyncapi.yaml`: `channels.gateway.x-close-codes`, `operations`,
  `components.messages`. `docs/api/webhooks.md` section 7 (block `webhook_events`).
  `docs/api/README.md` ("Change"), `docs/api/redocly.yaml`.
- `docs/operations/configuration.md` section "Named values of all chapters" (row
  `api.deprecation_min_notice`).
- ADRs: `docs/adr/0013-spec-first-openapi-31-and-asyncapi-30-normative.md`,
  `docs/adr/0014-api-versioning-and-deprecation-policy.md`,
  `docs/adr/0015-error-format-rfc-9457-idempotency-pagination.md`,
  `docs/adr/0016-webhooks-per-standard-webhooks.md`,
  `docs/adr/0049-machine-readable-action-catalogue-as-the-single-source.md`.
- `docs/quality/gates.md`, `docs/spec/23-quality.md` "Gates", `tools/README.md`,
  `tools/pyproject.toml`, `Makefile`, `CHANGELOG.md`, `docs/DECISIONS.md`.

Operations touched: every operation in `paths` and `webhooks` is read; none is changed unless the
tool finds a real difference. `suspendApp` and `resumeApp` carry the `Idempotency-Key` parameter;
every other POST or PATCH without it carries `x-idempotency-exempt` (06 "Idempotency" item 3 and
block `idempotency_exempt`). No table of chapter 03 and no action id changes.

## Fixed requirements that apply

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.
6. Test first: every behavior gets a test that is red before the change (show the red output in
   the report), then green.
7. No en dash (U+2013) and no em dash (U+2014) in any file or commit message. Check every changed
   file with `uv run --project tools --locked --quiet python tools/hooks/text_rules.py <file>` (exit code 0; plain `grep -P` fails on macOS).
8. White label: no company, person or host names, no real domains (only example.com and
   example.org), no real IP addresses, no absolute paths into a home directory, no internal
   system names, in any file including tests and fixtures.
9. Limits are named configuration values from `docs/operations/configuration.md`; the deprecation
   check reads `api.deprecation_min_notice` from there and never hard-codes its value.
10. G2 and G3 run in the one local gate command identical to CI (06 "Contract tests and gate"
    item 3, ADR 0038); the tool uses Python with PyYAML only.
11. Every operation names exactly one known action and the class of that action; system actions
    never appear on a route or op (05 rules 3, 4, 10; test 7).
12. Every POST and PATCH accepts `Idempotency-Key` except the operations with
    `x-idempotency-exempt` (06 "Idempotency" item 3), and the gate holds that list (06 G2; 04 5.2;
    R-API-09, R-API-24).
13. A deprecated op or field carries `x-deprecated-since`, `x-deprecated-at` and `x-sunset`, with
    `x-sunset` at least `api.deprecation_min_notice` after `x-deprecated-at` (06 13.4, R-API-25,
    RFC 9745, RFC 8594).
14. Descriptions in `openapi.yaml` and `asyncapi.yaml` are English (Ruling L5, R-API-15);
    chapters are English too.
15. Additive API changes raise `info.version` MINOR, documentation-only changes PATCH (06 13.1);
    this ticket changes neither API file unless the tool finds a difference.

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
   `tools/api-checks/README.md` (English, short).
3. Specification already in place (verify, do not redo): `docs/api/openapi.yaml` has
   `x-problem-codes` on `components.schemas.Problem` (entries `{code, status, title, description,
   client_action}`, English, sorted by status, `router: true` for `method_not_allowed`), `x-status`
   on every entry of `components.responses`, and `x-idempotency-exempt` on every POST and PATCH
   without the `IdempotencyKey` parameter; `docs/api/asyncapi.yaml` has English `client_action`
   texts in `channels.gateway.x-close-codes` following 04 "Close codes"; 06 has the blocks
   `problems` and `idempotency_exempt`, the text of 5, 7.3, 13.4 and 15 (G2, G3) and the rulings
   R-API-23 to R-API-25. If the tool finds a difference between a source and its copy, fix the copy
   (or the source, if the copy is right by the chapter text) and list it in the report.
4. `docs/spec/06-api.md`: blocks `problems`, `idempotency_exempt`, `endpoints`, `ws_ops`, `events`
   regenerated by the tool; no content change expected.
5. (no further spec text change; the chapter already describes the tool as `tools/api-checks` and
   `make api-checks`).
6. `docs/api/webhooks.md`: block `webhook_events` regenerated (no content change expected).
7. `docs/api/README.md` "Change": item 2 names `x-idempotency-exempt` or the `IdempotencyKey`
   parameter and new problem codes in `x-problem-codes`; item 5 names `make api-tables` and
   `make api-checks`.
8. `Makefile`: `api-checks` (`## API contract copies equal their sources (06 G2, G3)`) in `gates`
   directly after `api-lint`; `api-tables` (`## Regenerate the generated tables of chapter 06 and
   webhooks.md`); bandit directory list of `lint` gains `tools/api-checks`; `.PHONY`.
9. `tools/pyproject.toml`: add `api-checks` to `testpaths`, `pythonpath`, coverage `source` and
   mypy `files`.
10. `docs/quality/gates.md` (row `api-checks`), `docs/spec/23-quality.md` "Gates" group "Docs"
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
| `webhook-events` | every entry in `webhooks`: the key is a catalogue event type; `x-receive-action` equals its `receive_action`; `x-authz-action` is `webhook_out.deliver`; `x-rate-limit-class` is `none`. Every durable catalogue event is either a key of `webhooks` or named in the table "Not subscribable" of `docs/api/webhooks.md` section 7 (backticked event types in its second column), never both |
| `ws-ops` | the set of asyncapi operations on the channel `gateway` equals the catalogue ops: an operation with one message matches the op whose `op` equals the message `name`; the operation with many messages is `dispatch`; asyncapi `action: receive` matches a direction ending `_to_server`, `send` matches `server_to_client`; `x-authz-action` equals the op's `action`; `x-rate-limit-class` equals the class of that action |
| `channel-ops` | the asyncapi operations on the terminal channels (`terminalStream`, `hostControl`, `hostData`) equal the catalogue `channel_ops` (M1-02): channel by name, op by message `name` (`noise_frame` for binary frames); `receive` matches `device_to_server`, `host_to_server` or `both`, `send` matches `server_to_device`, `server_to_host` or `both`; `x-authz-action` equals the row's `action`; `x-rate-limit-class` equals the class of that action. No operation exists on any other channel |
| `events` | the set of asyncapi messages with `x-event-kind` equals the catalogue events (compare as sets; order may differ); per event: `x-event-kind` equals `kind`, `x-receive-action` equals `receive_action`, `x-audience: self` exactly when `audience: self`; the `dispatch` operation lists exactly these messages |
| `close-codes` | `channels.gateway.x-close-codes` lists the same `(code, name)` pairs in the same order as the table in 04 "Close codes" (code as integer, name verbatim); `client_action` is not compared because the meaning stays in 04 and the asyncapi text has its own wording (L5) |
| `idempotency` | every POST and PATCH in `paths` has either the `IdempotencyKey` parameter (operation or path level, by `$ref` or by `name: Idempotency-Key, in: header`) or `x-idempotency-exempt` with a known reason, never both and never neither; no other method carries `x-idempotency-exempt` |
| `deprecation` | in both API files, every mapping with `deprecated: true` has `x-deprecated-since` (SemVer), `x-deprecated-at` and `x-sunset` (both `YYYY-MM-DD`), and `x-sunset - x-deprecated-at >= api.deprecation_min_notice`; any of the three `x-` keys without `deprecated: true` is a finding |
| `app-scopes` | the enum `components.schemas.AppScope` equals the set of `app_scope` values of the catalogue without `none` (compare as sets); a scope in 05 section 2.6 that the enum lacks cannot be requested by any app |
| `problem-codes` | `x-problem-codes` has unique codes, a 3-digit `status`, non-empty English fields, and is sorted by status; every `components.responses` entry has `x-status`; every reference to it in `paths` uses that status as key, or `default` when `x-status` is 500 or higher; every backticked code in the description of a response is in `x-problem-codes` with the response's `x-status`; every entry without `router: true` is named by at least one response |

Reading `api.deprecation_min_notice`: find the Markdown table in
`docs/operations/configuration.md` whose header has the columns `Key` and `Default`, take
the row whose first cell is `` `api.deprecation_min_notice` ``, strip backticks from the
`Default` cell; durations accept the units `s`, `m`, `h`, `d`. A missing or duplicate row is a
finding, never a default. Locate columns by header name, because M1-08 may add columns.

### Exemption reasons (`x-idempotency-exempt`)

The texts are those of the block `idempotency_exempt` in 06 (source of truth); the tool
keeps them as a constant mapping.

| Code | Text in block `idempotency_exempt` | Operations (state at planning) |
|---|---|---|
| `auth_flow` | login or session flow: one-time challenge or code, refresh rotation, replacement of the child session | `beginOidcLogin`, `completeOidcLogin`, `loginWithTrustedHeader`, `beginPasskeyLogin`, `completePasskeyLogin`, `beginEnrollment`, `completeEnrollment`, `redeemEnrollmentCode`, `beginPairing`, `completePairing`, `loginWithPassword`, `refreshToken`, `logout`, `createStepUpChallenge`, `completeStepUpWithPasskey`, `completeStepUpWithDeviceKey`, `beginIdentityLink`, `completeIdentityLink`, `beginPasskeyRegistration`, `completePasskeyRegistration`, `createExtensionSession` |
| `read_only` | reads only, changes no durable state | `previewGrant`, `autocompleteCommand`, `validateManifest`, `verifyAuditChain`, `createFileTicket` |
| `agent_runtime` | agent runtime: monotonic or compare-and-swap | `sendHeartbeat`, `ackAgentEvents`, `patchRun`, `startRun`, `resumeRun`, `awaitRunInput`, `finishRun`, `answerAutocomplete` |
| `client_id` | client ID in the body (table above) | `createMessage`, `createInteraction`, `postIncomingWebhook` |
| `origin_key` | origin key of the import (`[22-import-migration.md](22-import-migration.md)`) | `applyImportBatch`, `createImportFile` |
| `telemetry` | anonymous measurements; a retry counts a sample twice | `submitClientMetrics` |

The `origin_key` text is written into 06 as a Markdown link relative to `docs/spec/` (shown as code
here because this brief lives in another folder).

`suspendApp` and `resumeApp` change state with audit and fit no reason; they carry the
`IdempotencyKey` parameter. If the base branch has a POST or PATCH operation with neither, the
check fails; fix the operation spec first (header, or a reason only if one clearly applies) and
list the decision in the report. The texts live as a constant mapping in `api_checks.py`
because they render text of chapter 06; comments and docstrings stay English.

### Rule `api-tables` (G3), blocks

The tool owns exactly these blocks; any other `BEGIN GENERATED` name in the two files is a
finding. Use `action_catalog.generated_blocks` and `replace_generated_block`. Rows render as
`"| " + " | ".join(cells) + " |"` (an empty cell becomes two spaces, as in the current blocks).

| File | Block | Source and rows |
|---|---|---|
| 06 | `problems` | `x-problem-codes` in list order: `` `code` ``, status, `description`, `client_action` |
| 06 | `idempotency_exempt` | header `\| operationId \| Method \| Path \| Reason \|`; operations with `x-idempotency-exempt`, ordered by the reason order of the table above, then by order in `paths`; path as `` `/api/v1<path>` ``; reason as text |
| 06 | `endpoints` | for each tag in `tags` order that has operations: `#### <tag>`, blank line, header `\| Method \| Path \| operationId \| Action \| Purpose (from openapi.yaml) \|`, separator, rows `\| <METHOD> \| \`/api/v1<path>\` \| \`<operationId>\` \| \`<x-authz-action>\` \| <summary> \|` in `paths` order; groups separated by one blank line |
| 06 | `ws_ops` | header `\| Op \| Direction \| Action \| Body schema \| Reply \|`; one row per asyncapi operation of the channel `gateway` in file order (the terminal channels are documented in 24 and 05, not in this block): message `name`, `Client to server` for `receive` and `Server to client` for `send`, `x-authz-action`, last segment of the `$ref` of `payload.properties.d`, reply message name or empty; the operation with many messages renders the fixed row `` \| `dispatch` \| Server to client \| Receive action of the event type \| `<Type>Event` \| \| `` |
| 06 | `events` | header `\| Event type \| Kind \| Receive action \| Audience \| Schema \| Webhook \|`; messages in the order of the `dispatch` operation: `name`, `durable` or `ephemeral`, `x-receive-action`, `x-audience` or empty, payload schema name, `yes` if the type is a key of `webhooks`, else empty |
| `webhooks.md` | `webhook_events` | header `\| Event type \| Receive action \| Receiver and scope in addition to \`events:subscribe\` \| Schema \|`; entries of `webhooks` in file order: type, `x-receive-action`, scope cell, schema of the request body. Scope cell: event with `audience: self` and `receivers: agent` gives `agent subscription only (agent is the receiver)`; other `self` events give `own only (app or agent is the receiver)`; otherwise the `app_scope` of the receive action in backticks, or `none (metadata, 05 section 2.6)` for `none` |

Prototype runs on the planning state reproduced `problems`, `idempotency_exempt`, `endpoints`,
`ws_ops`, `events` and `webhook_events` byte for byte with these rules; a difference after
implementation is a bug in the renderer, not a reason to edit the block by hand.

### Rulings for `docs/DECISIONS.md`

Rulings 1 to 3 and 5 are already recorded in 06 (R-API-23 to R-API-25); `docs/DECISIONS.md` points
to them. New here:

1. Close codes are compared by code and name only; asyncapi texts are English. Reason: L5; the
   meaning stays in 04.
2. Problem codes the router emits without any operation documenting them carry `router: true`.
   Reason: the check "every code is named by a response" would otherwise fail on
   `method_not_allowed`.

## Tests (write them first)

`tools/api-checks/tests/test_api_checks.py`. Each negative test copies the real inputs
(`docs/spec/actions.yaml`, `docs/spec/04-sync-protocol.md`, `docs/spec/05-actions.md`,
`docs/spec/06-api.md`, `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `docs/api/webhooks.md`,
`docs/operations/configuration.md`) into `tmp_path`, applies one mutation (a small helper edits
the YAML through PyYAML and writes it back, or edits text), runs the rule and asserts a finding
with the named sub-check:

- `test_repository_is_clean` (both rules, real repository, no findings).
- `rest-action`: `test_unknown_action_fails`, `test_system_action_on_route_fails`,
  `test_rate_limit_class_mismatch_fails`.
- `webhook-events`: `test_webhook_receive_action_mismatch_fails`,
  `test_webhook_for_unknown_event_fails`, `test_durable_event_neither_webhook_nor_listed_fails`,
  `test_durable_event_both_webhook_and_listed_fails`.
- `ws-ops`: `test_ws_op_action_mismatch_fails`, `test_ws_op_missing_in_asyncapi_fails`,
  `test_ws_direction_mismatch_fails`.
- `channel-ops`: `test_channel_op_action_mismatch_fails`, `test_channel_op_missing_in_asyncapi_fails`,
  `test_operation_on_unknown_channel_fails`.
- `events`: `test_event_receive_action_mismatch_fails`, `test_event_audience_mismatch_fails`,
  `test_event_kind_mismatch_fails`, `test_event_missing_from_dispatch_fails`.
- `close-codes`: `test_close_code_name_mismatch_fails`, `test_close_code_missing_fails`.
- `idempotency`: `test_post_without_key_or_exemption_fails`, `test_unknown_exemption_reason_fails`,
  `test_key_and_exemption_together_fails`, `test_exemption_on_get_fails`.
- `deprecation`: `test_deprecated_without_sunset_fails`, `test_sunset_too_early_fails`,
  `test_sunset_exactly_min_notice_passes`, `test_sunset_without_deprecated_fails`,
  `test_missing_config_value_fails`.
- `app-scopes`: `test_app_scope_missing_from_enum_fails`, `test_enum_scope_unknown_to_catalogue_fails`.
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
- [ ] `x-problem-codes`, `x-status`, `x-idempotency-exempt` and `x-deprecated-at` are checked;
      every difference the tool found is fixed and listed in the report.
- [ ] asyncapi close codes equal 04 in code and name.
- [ ] `docs/api/README.md`, `docs/quality/gates.md`, chapter 23, `tools/README.md`,
      `tools/api-checks/README.md`, `CHANGELOG.md`, `docs/DECISIONS.md` updated.
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
