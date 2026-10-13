# M4-02: Dart API client as a gate

Status: ready | Depends on: M4-01, M1-17 | Milestone: M4

## Goal

After this ticket `packages/api_client` holds the Dart client of the public REST API, generated from `docs/api/openapi.yaml` with openapi-generator 7.26.0 `dart-dio` (06 14) and checked in, plus a small hand-written layer that every later client package uses: one way to build the client for an instance, the headers `Idempotency-Key` and `traceparent`, retry for reads only, the mapping of RFC 9457 problems to typed errors with their `code`, and tolerant decoding of event pages. The gate holds it: `make -C packages/api_client generate-check` regenerates the client into a temporary folder, formats and builds it, compares it byte for byte with the checked-in state and analyses it; `make api-clients` calls it, so a spec change without regeneration fails `make check` (G4 for Dart). The tolerance test G5 proves for Dart that an unknown field, an unknown enum value and an unknown event type are skipped without an error. Why: the apps and the console must build against the tested contract instead of hand-written models (15 8, R-Clients), and an additive change of the spec (06 13) must never break an installed app.

## Read first

- `docs/PLAN.md`: section 3; section 5 (row "Contract": G1 to G6 in `make check`); section 8 "Exit criteria" item 4 and the row M2-23 (the same gate for TypeScript and Python); section 10 (Scope, Test strategy, the rows M4-02, M4-09, M4-13, M4-17, which use this package); section 19 P-04, P-09, P-10, P-11.
- `docs/spec/06-api.md`: 1 "Overview", 2 "Basic rules" (row "Correlation"), 3.1 "DPoP" (only to know what M4-09 adds), 5 "Errors", 6 "Pagination", 7 "Idempotency", 9 "Rate limits" (`Retry-After`), 13 "Versioning, deprecation, compatibility" (the table and item 3: Dart with `enumUnknownDefaultCase=true`), 14 "Code generation" (row "Dart package of the apps", items 1 to 4), 15 "Contract tests and gate" (G4, G5).
- `docs/spec/15-client-architecture.md`: 3, 3.1 (rule 5: `api_client` is generated and checked in), 8 "Network and login" (REST bullet), 17 (`traceparent` on every request, ADR 0057 point 4).
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" (no JSON `null`, unknown fields), 7.4 "REST variant" (`GET /api/v1/events`), 9 item 3.
- `docs/spec/23-quality.md`: 3 (row "Generated client": `generate-check`), "Documentation in code" (generated code excluded).
- `docs/api/openapi.yaml` (whole file; `components.schemas.Event` and its discriminator, `Problem`, `EventPage`), `docs/api/redocly.yaml`.
- ADRs: `0013` (spec first), `0026` (SDK with a hand layer), `0052` (one envelope), `0053` (no JSON `null`).
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115; the rulings of M2-23 (`D-M2-23-*`) if present; section 10.
- Briefs: `docs/briefs/M2-23-generated-clients.md` (Design notes 8 and 9: gate scope of generated code, the `api-clients` contract), `docs/briefs/M1-17-api-server-stubs.md` (the server's generation from the same file, the enable list that concerns the server only), `docs/briefs/M4-01-workspace-arch.md` (generated patterns, workspace member rules).
- Code on the base branch: root `Makefile` (`api-clients`), `tools/ci/codegen_check.py` and its tests, `tools/ci/components.py` (`API_PREFIX` selects every client), `packages/api_client/` skeleton of M4-01, `tools/arch-check/rules.yaml`.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

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

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`. This ticket adds none; the read retry uses the server's `Retry-After` and the backoff keys of the caller (Design note 4), never its own number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. This ticket does not change `docs/api/`; if the generator cannot process the file, stop (Boundaries).

Product requirements implemented here:

11. Generator openapi-generator 7.26.0, generator `dart-dio` with `built_value`, settings `enumUnknownDefaultCase=true` and `legacyDiscriminatorBehavior=false` (06 14); pinned, run reproducibly.
12. Generated code is checked in; the gate generates it again and fails on a difference (G4, 06 14 item 1, 15 3.1 rule 5).
13. G5 for Dart: one test with a fixture: an unknown field, an unknown enum value and an unknown event type are skipped without an error (06 15).
14. A hand-written layer over the generated core (06 14 item 2): `Idempotency-Key` on writes from the caller's outbox ID, `traceparent` on every request, retry only for reads, mapping of problem codes; DPoP and nonce retry come from `auth` (M4-09) through the interceptor slot this layer provides.
15. No JSON `null` is sent (ADR 0053): request bodies omit absent fields.
16. `api_client` imports no `package:flutter` (15 3.1 rule 1).

## Scope

Create in `packages/api_client/`:

- `Makefile` with the targets `generate` and `generate-check` (Design note 2); `.openapi-generator-ignore`; `tool/generate.sh` or the recipe inline (Design note 2).
- The generated sources (paths chosen by the generator under `lib/` and the generated `lib/src/` tree), formatted, with their `*.g.dart` serializers from `build_runner`.
- The hand layer `lib/src/hand/`: `client_factory.dart`, `headers.dart`, `read_retry.dart`, `problem.dart`, `events.dart`; exported from `lib/api_client.dart` next to the generated API (Design notes 3 to 6).
- Tests: `test/tolerance_g5_test.dart` with `test/fixtures/g5/*.json`, `test/problem_test.dart`, `test/headers_test.dart`, `test/read_retry_test.dart`, `test/events_test.dart`, `test/null_free_test.dart`.
- `tools/ci/tests/test_api_client_generate.py` (Tests).

Change:

- `packages/api_client/pubspec.yaml` (dependencies the generated code needs: `dio`, `built_value`, `built_collection`, `one_of`, `one_of_serializer`, `http_parser`, `uuid` for the hand layer; dev: `build_runner`, `built_value_generator`, `test`), `README.md`, `analysis_options.yaml` (generated paths excluded per M4-01).
- Root `Makefile`: `api-clients` keeps calling `make -C packages/api_client generate-check` (it is already in the list); only if M2-23 has not yet changed the target and it does not fail on a manifest without `generate-check`, leave it as it is.
- Root `analysis_options.yaml` (exclude list), `tools/ci/lcov_filter.py` with `tools/ci/tests/test_lcov_filter.py`, and `sonar-project.properties` (exclusions): only to add the exact paths the generator writes when they do not match the generated patterns of M4-01 (the `dart-dio` generator writes `lib/src/api/` and `lib/src/model/`), each with a test case (Design note 1).
- `docs/spec/06-api.md` 14 (row "Dart package of the apps": the pinned image, the ignore file, where the hand layer lives), `docs/quality/gates.md` (row `api-clients`: Dart), `docs/quality/toolchain.md` (the generator image with digest).
- `docs/DECISIONS.md` section 10: rows `D-M4-02-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- DPoP proofs, nonce adoption, refresh and step-up interceptors: M4-09 (it plugs into the slot of Design note 3).
- The outbox and its retry of writes: M4-15. Gateway frames: M4-13 (asyncapi has no generator in the gate, 06 14 item 3).
- Upload of file parts and `application/octet-stream` bodies: M4-22 builds them on the hand layer.
- Changes to `docs/api/` and to the server.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-02-dart-api-client`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- Docker must be available for the generator (Design note 2); without it the ticket stops with `Verdict: BLOCKED docker missing`.
- If the pinned generator fails on `docs/api/openapi.yaml` (OpenAPI 3.1 is beta in the generator, 06 14), first try the generator options and a pre-filter that only rewrites 3.1 constructs into equivalent 3.0 forms in a temporary copy (for example `type: [x]` is not allowed anyway; `examples`); if the result still fails or would change the meaning of an operation, stop with `Verdict: BLOCKED generator: <error>` and the failing construct, instead of editing the spec.

## Design notes

1. **Package layout.** The generator writes into a temporary folder; the recipe copies only the generated Dart sources (`lib/` without the paths the ignore file protects) into the package. Hand-written files that the generator must never touch are listed in `.openapi-generator-ignore` and in the copy step: `pubspec.yaml`, `README.md`, `analysis_options.yaml`, `Makefile`, `.openapi-generator-ignore`, `lib/api_client.dart`, `lib/src/hand/**`, `test/**`. The generated library entry (`lib/<pubName>.dart` of the generator) is named `lib/src/generated_api.dart` by the option `pubLibrary` or by a rename in the recipe; `lib/api_client.dart` exports it and the hand layer. Generator properties: `pubName=api_client`, `pubLibrary=api_client.generated`, `serializationLibrary=built_value`, `enumUnknownDefaultCase=true`, `legacyDiscriminatorBehavior=false`, `useEnumExtension` left at default. Generated paths match the patterns of M4-01 (`lib/src/generated/**` or `*.g.dart`); if the generator's layout does not fit them, add the exact generated paths to the shared exclude list of M4-01 in the root `analysis_options.yaml`, `tools/ci/lcov_filter.py` and `sonar-project.properties` in the same change, and test it there.

2. **Generation and the gate.** Ruling for docs/DECISIONS.md: the generator runs from the image `openapitools/openapi-generator-cli:v7.26.0` pinned by its multi-architecture digest (`docker buildx imagetools inspect` at build time; record the digest in `docs/quality/toolchain.md` and in the `Makefile`), with `--user` of the caller, the repository mounted read-only and a temporary output folder mounted writable, and with `--network none`; reason: the generator needs a Java runtime that the repository does not pin otherwise, and the digest makes the run reproducible. `make -C packages/api_client generate`: run the generator, copy the sources (note 1), `dart run build_runner build --delete-conflicting-outputs` in the package, `dart format` over the generated files. `make -C packages/api_client generate-check`: the same into a temporary copy of the package (`mktemp -d`, removed on exit), then a byte comparison of every generated file (missing, extra or changed files fail with their paths and the hint `make -C packages/api_client generate`), then `dart analyze --fatal-warnings` limited to errors in generated files (the generator emits warnings there, 06 14) and without findings in the hand layer. The gate never writes into the working tree. If `tools/ci/codegen_check.py` offers a reusable comparison, use it instead of a second implementation.

3. **Client factory.** `ApiClient createApiClient({required Uri baseUrl, List<Interceptor> authInterceptors = const [], required TraceContext Function() traceContext, HttpClientAdapter? adapter, Duration? connectTimeout})`: builds one `Dio` with `baseUrl` = `baseUrl` plus `/api/v1` (06 R-API-01), `responseType` JSON, the interceptors in a fixed order (headers of note 4, then `authInterceptors` from M4-09, then problem mapping of note 5, then the read retry of note 4 outermost), and returns the generated API root with that `Dio`. The adapter parameter lets tests and the fake server (M4-17) replace the network. No global state; one instance per server address.

4. **Headers and read retry.** `Idempotency-Key`: a request option `RequestOptions.extra['idempotency_key']` set by the caller (the outbox of M4-15 passes its stored UUIDv7); the interceptor writes it as the header for `POST`, `PATCH`, `PUT` and `DELETE`, and never invents one (a write without a key is the caller's choice for operations marked `x-idempotency-exempt`). `traceparent`: from the `traceContext` callback (W3C trace context, a new span ID per request, version `00`, sampled flag from the callback); the server only links it (06 2 "Correlation"). The response header `Correlation-Id` is exposed on the typed error and on a `ResponseMeta` for logs of the caller (the value is not secret). Read retry: only `GET` and `HEAD`, only for a network error, a timeout, `429` and `503`; waits `Retry-After` when the response has it, otherwise a full-jitter backoff whose base and cap the caller passes (`ReadRetryPolicy(base, cap, maxAttempts)`; the apps pass `client.outbox_retry_base` and `client.outbox_retry_cap` through the app configuration of M4-08); never for `4xx` other than `429`; `503 setup_required` is final. Writes are never retried here (15 8: the outbox retries writes).

5. **Problems.** `ApiProblem implements Exception` with `status`, `code` (the `code` member of RFC 9457, 06 5), `title`, `detail`, `correlationId`, `reason`, `hint`, `errors` (list of `pointer`, `code`, `detail`), `current` (raw JSON object for `version_conflict` and `base_version_conflict`), `retryAfter`; built from `application/problem+json` and from any non-2xx response without a problem body (`code` `http_<status>`). An unknown `code` stays a string; the apps translate known codes and show unknown ones with the code (01 9). Never log or put the request body into the exception message.

6. **Tolerant events.** The generated `Event` union may throw on an unknown discriminator value. `EventsPageDecoder.decode(Map<String, Object?> json) -> DecodedEventsPage` decodes `events` element by element: an element whose `type` the generated serializers do not know is kept as `UnknownEvent(id, seq, type, schemaVersion, raw)` and never throws; an element of a known type with unknown fields decodes with the fields ignored; an unknown enum value decodes to the generated unknown default case. `next_cursor`, `head_seq` and `sync_epoch` are copied. The sync engine (M4-14) applies only known events and advances the cursor over unknown ones. The same decoder is used for a single gateway `dispatch` envelope (`decodeEnvelope`). Ruling for docs/DECISIONS.md: tolerance lives in the hand layer, because the generated union cannot skip an unknown member.

7. **No `null` on the wire.** A test serializes every generated request model that the apps use in M4 (create message, update message, create interaction, update settings) with only required fields set and asserts that the JSON contains no `null` value; if the generator emits `null` for absent fields, configure the serializer (`built_value` `@BuiltValueField(serialize: false)` is not generated; instead use a `Serializers` plugin in the hand layer that strips `null` members before sending) and record the ruling.

8. **Rulings for docs/DECISIONS.md**, one row each (`D-M4-02-<n>`): the pinned generator image (note 2); the copy step and ignore list (note 1); tolerant events in the hand layer (note 6); the `null` stripping, if needed (note 7); read retry only for reads with the caller's backoff keys (note 4).

## Tests (write them first)

Dart, `packages/api_client/test/` (run by `flutter test --coverage`; the network is a fake `HttpClientAdapter` that records requests and returns canned responses):

- `tolerance_g5_test.dart` (G5): `unknown field is ignored` (a `Message` response with an extra member `future_field` decodes, the known fields are equal); `unknown enum value decodes to the unknown default case` (a `trigger_state` of `"rocket"` gives the generated unknown value, no exception); `unknown event type is skipped` (an events page with three events, the middle one of type `future.happened`: two known events decoded, one `UnknownEvent` with its `seq`, cursor values copied); `unknown event schema version is read` (a known type with `schema_version` 99 and an extra payload member decodes).
- `problem_test.dart`: a `403 forbidden` problem with `reason`; `400 validation_failed` with two `errors`; `412 version_conflict` with `current`; `429 rate_limited` with `Retry-After: 7`; a `502` without a body gives `code` `http_502`; the exception text contains no request body (a canary in the body).
- `headers_test.dart`: `POST` with `idempotency_key` carries `Idempotency-Key`; `GET` never does; a `POST` without the option carries none; every request carries a valid `traceparent` (regular expression of the W3C format) with a new span ID per request; `Correlation-Id` of the response is exposed.
- `read_retry_test.dart`: `GET` retried after a socket error and after `503` with `Retry-After: 1` (fake clock), not after `404`, not after `503 setup_required`; `POST` never retried; at most `maxAttempts`.
- `events_test.dart`: `decodeEnvelope` of a `message.created` dispatch, of an unknown type, of an envelope with `truncated: true` (flag kept).
- `null_free_test.dart` (Design note 7).

Repository tools, `tools/ci/tests/test_api_client_generate.py` (marked to run when Docker and `flutter` are present; otherwise it prints why it is skipped, and the report says where it ran):

- `test_generate_check_passes_on_clean_tree`;
- `test_spec_change_without_regeneration_fails` (a temporary copy of the repository where `docs/api/openapi.yaml` gains an optional response field on one schema: `make -C packages/api_client generate-check` exits non-zero and names a changed file);
- `test_hand_edit_of_generated_file_fails` (one character changed in a generated file);
- `test_generator_image_pinned_by_digest` (the `Makefile` names the image with `@sha256:` and `--network none`).

Commands that must pass, from the repository root:

- `make -C packages/api_client generate-check`
- `cd packages/api_client && flutter test --coverage`
- `cd tools && uv run --locked --quiet pytest ci/tests/test_api_client_generate.py ci/tests/test_components.py`
- `make api-clients`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`
- `make check`

## Acceptance criteria

- [ ] `packages/api_client` is generated from `docs/api/openapi.yaml` with the pinned `dart-dio` generator and checked in; `make api-clients` regenerates, compares and analyses it.
- [ ] A spec change without regeneration fails `make api-clients`; a hand edit of a generated file fails it.
- [ ] G5 for Dart: unknown field, unknown enum value and unknown event type are skipped without an error.
- [ ] The hand layer sets `Idempotency-Key` from the caller, `traceparent` on every request, retries only reads, maps problems with their `code`, sends no `null`, and offers the interceptor slot for M4-09.
- [ ] No `package:flutter` in `api_client` (`make arch`); generated code excluded from doc lint and coverage, hand layer covered at least `quality.coverage.dart_min_percent`.
- [ ] 06 14, gates.md and toolchain.md updated; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol of the hand layer (purpose, parameters, return value, errors, example where useful); comments explain why.
- `packages/api_client/README.md` (English): what is generated, how to regenerate, the hand layer, what M4-09 plugs in.
- `docs/spec/06-api.md` 14, `docs/quality/gates.md`, `docs/quality/toolchain.md` updated; rulings of Design note 8 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The generator image digest, the Dart and dependency versions, the number of generated files and their total size, every warning the analyzer reports in generated code.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
