# M1-17: Server interface generated from openapi.yaml and enable list (ADR 0013)

Status: ready | Depends on: M1-02, M1-05 | Etappe: M1

## Goal

After this ticket the Go server has a generated REST layer. oapi-codegen produces Go types for every schema in `docs/api/openapi.yaml` (REST bodies, events, gateway frames, problems) and the std-http plus strict-server interface for exactly the operations that `services/server/api/operations.yaml` lists as enabled (plan decision P-04). A generated route table gives every enabled operation its authorization action, rate-limit class and request metadata, and `bothy serve` refuses to start when a registered route has no known action (AZ-P1). Every later handler ticket plugs into this contract, and the chain tickets M1-18 to M1-20 read their per-route metadata from it.

## Read first

- `docs/PLAN.md`: section "M1 Fundament" (rows M1-17 to M1-20 and M1-25, table "Bekannte Widersprüche" rows W2 and W6), section "Arbeitsweise je Ticket", section "Entscheidungen dieses Plans" (P-04, P-05, P-09 to P-12).
- `docs/adr/0013-spec-first-openapi-31-und-asyncapi-30-normativ.md`, `docs/adr/0008-hexagonale-module-und-gepruefte-abhaengigkeitsregeln.md`, ADR 0049 (M1-02) and ADR 0050 (M1-05).
- `docs/spec/02-architektur.md`: "Code-Layout", "Schichten im Modul" (rule 3: module handlers embed the strict interface), "Automatische Prüfung der Regeln" (all subsections), "Request-Pfad".
- `docs/spec/06-api.md`: "Überblick" (prefix `/api/v1`, no health), "Grundregeln", "Autorisierung in der API" (points 1 and 7), "Codegenerierung", "Vertragstests und Gate", "Entscheidungen dieses Kapitels" (R-API-05, R-API-16, R-API-18).
- `docs/spec/05-aktionen.md`: "Verbindliche Regeln" (rules 3, 5 and 10), "Rate-Limit-Klassen", "Tests, die diesen Katalog halten" (tests 1 and 7).
- `docs/spec/07-rechte.md`: test table, rows AZ-P1 to AZ-P3.
- `docs/spec/actions.yaml` and `tools/catalog/` (M1-02): the machine-readable action catalogue (action id, right, rate-limit class). Read its documented schema before writing the reader.
- `docs/api/openapi.yaml`: `servers`, `security`, `components.parameters` (`IdempotencyKey`, `IfMatchRequired`, `IfMatchOptional`), `components.headers.ETag`, `components.schemas` (`Event`, `EventPage`, `EventEnvelope`, `Gateway*`, `Problem`); `docs/api/README.md`.
- `docs/quality/gates.md`, `docs/quality/toolchain.md`, `CONTRIBUTING.md`, `services/server/README.md`, `services/server/architecture/modules.yaml`, `services/server/sonar-project.properties`, `.golangci.yml`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: write the failing test, show it red, then make it green. No en or em dashes in any text. White label: no company, person, host or instance names in code, tests, fixtures or docs. Configuration values only through named keys from `docs/operations/configuration.md`.
7. Spec-first (B2, ADR 0013): the Go server stubs come from oapi-codegen v2.8.0 with `std-http-server`, `strict-server`, `models` and `output-options.skip-prune: true`, runtime `github.com/oapi-codegen/runtime` v1.7.0 (06 "Codegenerierung"). Generated code is checked in; the gate regenerates it and fails on any difference (R-API-18, `make codegen-check`).
8. P-04: the generated server registers only built operations; every other operationId of `openapi.yaml` stands in the `later` section of the enable list with its etappe.
9. 05 rule 3 and AZ-P1: every registered route has a known `x-authz-action` and the rate-limit class of that action; the server checks this at start, and a test counts the registered routes.
10. 05 rule 10 and test 7: no operation names an action whose right is `system`.
11. W2 (Ruling L2): `openapi.yaml` contains no health operations. W6: the REST base URL is `/api/v1` (06 "Überblick").
12. Owner rule 3 asks for API documentation generated from code. Under ADR 0013 the direction is spec to code with the same single source: routes are generated from `openapi.yaml`, and AZ-P1 plus `make codegen-check` prove that code and spec are equal. Do not generate OpenAPI from Go code.

## Scope

Paths are relative to `services/server/` unless they start with `docs/`.

1. `api/operations.yaml` (new): the enable list (format in Design notes). At this ticket `enabled` is empty, because no handler exists yet; `later` lists every operationId of `docs/api/openapi.yaml`.
2. `api/routes/gen/` (new, `package main`, with tests and `testdata/`): the generator. Inputs as flags: `docs/api/openapi.yaml`, `docs/spec/actions.yaml`, `api/operations.yaml`. It validates the inputs (rules in Design notes), writes a filtered copy of the spec into a `mktemp` directory (`os.MkdirTemp`), runs `go tool oapi-codegen` with `api/rest/oapi-codegen.yaml` on it, writes the route files below, and removes its temporary directory on every exit path.
3. `api/rest/oapi-codegen.yaml` and `api/rest/rest.gen.go` (generated, package `rest`).
4. `api/routes/routes.go` (hand-written: types, lookup, `Verify`, `Recorder`, `Build`), `api/routes/routes.gen.go` (generated operation table), `api/routes/catalog.gen.go` (generated action catalogue: id, right, class), `api/routes/build.gen.go` (generated glue that calls the `rest` constructors; see Design notes for the empty case).
5. A fixture pipeline: `api/routes/gen/testdata/fixture/{openapi.yaml,actions.yaml,operations.yaml}` with three operations (one public `GET` with a path parameter, one `POST` with `IdempotencyKey` and a JSON body, one `PATCH` with `IfMatchRequired` and an `ETag` on `200`), generated by the same generator into `api/routes/internal/fixture/rest` and `api/routes/internal/fixture/routes`. Checked in and covered by `make codegen-check`.
6. `api/generate.go`: the `//go:generate` lines for the real and the fixture pipeline. Make sure the generation step that `make codegen-check` runs (M1-01) includes them; if it runs `go generate ./...` in `services/server`, nothing else is needed.
7. `go.mod` and `go.sum`: tool directive for `github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen` pinned at v2.8.0 (`go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0`), `github.com/oapi-codegen/runtime` v1.7.0, one YAML library for the generator (`go.yaml.in/yaml/v3` or `gopkg.in/yaml.v3`; license must be in `quality/licenses/allowed.txt`).
8. `internal/wire`: build the routes for roles `api` and `all`, mount them on the public listener of M1-11, abort start on a failed check (Design notes "Start check").
9. `architecture/modules.yaml`: register the new packages and their vendors so that `make arch` passes without loosening any rule (`depOnAnyVendor` stays false).
10. Docs: `api/README.md` (new); `docs/spec/06-api.md` "Codegenerierung" (one point: the server generates only operations enabled in `services/server/api/operations.yaml`, through a pre-filtered spec, P-04); `docs/DECISIONS.md` (entries in Design notes); `services/server/CHANGELOG.md` (Unreleased).

## Out of scope

- HTTP chain, problem bodies, request validation, client IP, CORS and the proof of the authorize check: M1-18. This ticket passes `nil` error handlers to the generated code (oapi-codegen defaults); M1-18 replaces them.
- Rate limiting (M1-19) and idempotency (M1-20).
- Action constants in `internal/kernel/authorize`, the closed catalogue in `authz`, the registers of gateway ops and event types, AZ-P2 and AZ-P3: M1-25. M1-25 replaces the catalogue that `internal/wire` passes to `Build`.
- Enabling any operation: each handler ticket moves its operationIds from `later` to `enabled` and implements them in its module's `adapters/httpapi`.
- Go types for AsyncAPI frames beyond the schemas that `openapi.yaml` already holds (06 "Codegenerierung" point 3).
- Changes to `docs/spec/actions.yaml` or to the core contract (00, 03, 04, 05).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-17-api-server-stubs`.
- Conventional Commits with the component as scope (for example `feat(api): generate server interface from openapi.yaml`) and DCO sign-off (`git commit -s`). Squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp`; remove them when done.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Tests bind only `127.0.0.1` and port 0 (P-10).
- Do not lower a threshold, disable a gate or delete a test.
- A gap in the core contract (chapters 00, 03, 04, 05) that this ticket needs ends the ticket with `Verdict: BLOCKED` (P-11).

## Design notes

### Enable list `api/operations.yaml`

```yaml
# Enable list of REST operations (plan decision P-04).
version: 1
enabled: []          # operationIds that have a handler; sorted ascending
later:               # every other operationId of docs/api/openapi.yaml; sorted by operation_id
  - operation_id: addOwner
    etappe: M1
    ticket: M1-31    # optional
```

The generator fails, naming every offending entry, when: `version` is not 1; an operationId under `paths` of `openapi.yaml` is in neither section; an entry is in both sections, twice in one section, or not in `openapi.yaml`; a section is not sorted ascending (byte order); `etappe` is not one of `M1` to `M8` or `V1` to `V3`; `ticket` does not match `^(M[1-8]|V[1-3])-[0-9]{2}$` or its prefix differs from `etappe`. Operations under `webhooks:` are not part of the list (the server sends them, it does not serve them).

Filling `later` in this ticket, per operationId: (1) if a ticket row in a table of `docs/PLAN.md` names the operationId in backticks, take the first such row in document order: its ID is `ticket`, its prefix is `etappe`; (2) otherwise take the table below; (3) an operationId still without etappe ends the ticket with `Verdict: BLOCKED` and the list.

| operationIds | etappe | ticket |
|---|---|---|
| `beginPairing`, `completePairing`, `approvePairing` | M1 | M1-38 |
| `getPushPreview` | M3 | |
| `getUpload` | M3 | M3-03 |
| `createFileTicket` | M3 | M3-05 |
| `deleteFile` | M3 | M3-08 |
| `searchThreadTitles` | M3 | M3-14 |
| `listMyNotificationPreferences`, `putNotificationPreference`, `deleteNotificationPreference`, `listMyQuietHours`, `createQuietHoursRule`, `putQuietHoursRule`, `deleteQuietHoursRule` | M3 | M3-18 |
| `submitClientMetrics` | M4 | M4-31 |
| `setAgentToolClass`, `getAgentTriggerers`, `suspendApp`, `resumeApp` | M2 | |
| `getRunContext` | M2 | M2-13 |
| `submitCostReconciliation` | M2 | M2-28 |
| `listAgentWebhookSubscriptions`, `createAgentWebhookSubscription`, `listWebhookSubscriptions`, `createWebhookSubscription`, `updateWebhookSubscription`, `deleteWebhookSubscription`, `listWebhookDeliveries`, `redeliverWebhook` | M2 | M2-26 |
| `getStatus`, `getPublicStatus`, `createStatusComponent`, `updateStatusComponent`, `deleteStatusComponent`, `createStatusIncident`, `addStatusIncidentUpdate` | M7 | M7-09 |
| `listGdprRequests`, `createGdprRequest`, `getGdprRequest`, `decideGdprRequest`, `downloadGdprExport` | M7 | |
| `setChannelReadOnly` | M8 | M8-05 |
| `listImports`, `createImport`, `getImport`, `listImportItems`, `startImport`, `pauseImport`, `cancelImport`, `completeImport`, `setImportLead`, `applyImportBatch`, `createImportFile`, `setImportCursor` | M8 | M8-03 |

Write the list once with a throwaway script in a `mktemp` directory or by hand; do not commit the script. The generator only validates the list, it never rewrites it.

### Generator checks on the spec

In addition to the list rules, the generator fails when an operation under `paths` has no `x-authz-action` or no `x-rate-limit-class`, names an action that `docs/spec/actions.yaml` does not contain, names a class different from the class of its action in the catalogue, or names an action whose right is `system`. These checks run over all operations, not only enabled ones, so a broken spec fails early. (M1-03 checks the YAML pairing in G2; this check guards the generated table and must agree with it.)

### Filtering (P-04)

The generator writes a copy of `openapi.yaml` in which `paths` keeps only the methods of enabled operations (path items left without methods are dropped), `webhooks` is removed and `components` stays complete. Edit the YAML node tree so key order and comments of the copy stay stable. oapi-codegen runs on that copy with `skip-prune: true`, so every schema, including events and gateway frames, becomes a Go type.

Use the pre-filtered copy for every enable list, including the empty one, because one mechanism must cover both cases. P-04 asks this ticket to check `output-options.include-operation-ids`: probe it once against the fixture with a non-empty and with an empty list, and record the observed behaviour in `docs/DECISIONS.md` as the reason for the pre-filter.

oapi-codegen config `api/rest/oapi-codegen.yaml`: `package: rest`, `generate: {std-http-server: true, strict-server: true, models: true}`, `output: rest.gen.go`, `output-options: {skip-prune: true}`. Do not enable `embedded-spec`: it pulls in a second OpenAPI library, and M1-18 owns the runtime validation decision.

Generated files carry the standard `// Code generated ... DO NOT EDIT.` header and end in `.gen.go`. Make sure the Sonar exclusion and the golangci-lint configuration from M1-05 cover them; adjust the glob if M1-05 used another one.

### Route metadata (`api/routes`)

```go
// Operation describes one enabled REST operation. All values come from docs/api/openapi.yaml.
type Operation struct {
	ID             string      // operationId, for example getDiscovery
	Method         string      // upper case HTTP method
	Path           string      // template including the base URL, for example /api/v1/threads/{thread_id}
	Pattern        string      // net/http pattern exactly as the generated code registers it
	Action         string      // x-authz-action
	RateLimitClass string      // x-rate-limit-class
	Public         bool        // operation-level security: []
	IdempotencyKey bool        // declares #/components/parameters/IdempotencyKey
	IfMatch        IfMatchMode // IfMatchNone, IfMatchOptional, IfMatchRequired
	RequestBody    BodyKind    // BodyNone, BodyJSON, BodyMultipart, BodyOctetStream
	BodyRequired   bool        // requestBody.required
	ETagStatuses   []int       // 2xx statuses whose response declares the ETag header
}
```

Hand-written API, each symbol with a doc comment: `const BaseURL = "/api/v1"`; `Enabled() []Operation` (a copy, sorted by `ID`); `ByPattern(pattern string) (Operation, bool)`; `ByID(id string) (Operation, bool)`; `type Catalog interface { Class(action string) (class string, ok bool) }`; `SpecCatalog() Catalog` (backed by `catalog.gen.go`); `type Recorder` implementing the `ServeMux` interface of the generated std-http server (it records pattern and handler; its `ServeHTTP` panics, because a recorder never serves); `type Route struct { Operation Operation; Handler http.Handler }`; `type Options` carrying the std-http `ErrorHandlerFunc` and the strict `RequestErrorHandlerFunc` and `ResponseErrorHandlerFunc` (nil means the oapi-codegen default); `Verify(registered []string, cat Catalog) error`; `Build(impl Server, opts Options, cat Catalog) ([]Route, error)`, which runs the generated glue on a `Recorder` and then `Verify`.

`Verify` returns one joined error that names every problem: a registered pattern that is not in `Enabled()`, an enabled operation that was not registered, an action that the catalogue does not know, a class that differs from the catalogue. `Build` returns the routes sorted by `Operation.ID`.

`build.gen.go`: for a non-empty enable list it declares `type Server = rest.StrictServerInterface` and builds with `rest.NewStrictHandlerWithOptions` and `rest.HandlerWithOptions` (base URL `BaseURL`, base router the `Recorder`). For an empty list it declares `type Server = any` and registers nothing, so the package compiles whatever oapi-codegen emits for zero operations. Check which symbols oapi-codegen v2.8.0 emits for zero operations; the fixture covers the non-empty case, the real list the empty case.

`Pattern` is the string the generated `HandlerWithOptions` passes to `HandleFunc` (method, one space, base URL plus path template). A test compares the patterns the `Recorder` sees for the fixture with `Pattern`; if oapi-codegen formats them differently, the generator follows oapi-codegen.

### Start check

`internal/wire` calls `routes.Build(handlers, routes.Options{}, routes.SpecCatalog())` for roles `api` and `all`. `handlers` is the composite struct that embeds the `adapters/httpapi` handler of every module (empty now; 02 "Schichten im Modul" rule 3: the compiler proves completeness and that no operation is implemented twice). Each `Route.Handler` is mounted at `Route.Operation.Pattern` on the public mux of M1-11. On error, `bothy serve` logs `route check failed` with the joined error at level error and exits with code 1 before it reports ready. The catalogue is a parameter so a test can inject one that lacks an action, and so that M1-25 can pass the closed `authz` catalogue.

### Decisions to record in `docs/DECISIONS.md`

- P-04 implementation: pre-filtered spec for every list, with the observed behaviour of `include-operation-ids` for non-empty and empty lists.
- W2 checked: `openapi.yaml` has no health operation (name the check you ran).
- W6 checked: base URL `/api/v1` in `servers` and in the generated registration.
- Etappe assignment rule for `later` (the two-step rule above).

## Tests (write them first)

Generator, `api/routes/gen`:
- `TestGenerate_Fixture_Golden`: the fixture inputs produce the checked-in fixture packages byte for byte.
- `TestGenerate_RejectsEnabledNotInSpec`, `TestGenerate_RejectsOperationMissingFromList`, `TestGenerate_RejectsDuplicateEntry`, `TestGenerate_RejectsEntryInBothSections`, `TestGenerate_RejectsUnsortedSection`, `TestGenerate_RejectsBadEtappe`, `TestGenerate_RejectsTicketEtappeMismatch`: one negative fixture each; the error names the operationId.
- `TestGenerate_RejectsUnknownAction`, `TestGenerate_RejectsClassMismatch`, `TestGenerate_RejectsSystemAction`, `TestGenerate_RejectsMissingExtensions`.
- `TestGenerate_FilteredSpecKeepsAllComponents`, `TestGenerate_FilteredSpecDropsWebhooksAndDisabledMethods`.
- `TestGenerate_RemovesTempDir`, also on the error path.
- `TestRealInputs_Valid`: the validation passes on the real `docs/api/openapi.yaml`, `docs/spec/actions.yaml` and `api/operations.yaml` (gate check: every operation of the list exists in `openapi.yaml`, and the list is complete).

Routes, `api/routes` (with the fixture packages):
- `TestAZP1_FixtureRoutesMatchEnableList`: the recorded patterns equal the `Enabled()` patterns, each with a known action and matching class.
- `TestOperationMetadata_Fixture`: `Public`, `IdempotencyKey`, `IfMatch`, `RequestBody`, `BodyRequired` and `ETagStatuses` as declared in the fixture.
- `TestVerify_UnknownPattern`, `TestVerify_MissingRegistration`, `TestVerify_UnknownAction`, `TestVerify_ClassMismatch`: each returns an error naming the operation.

Generated code, `api/rest`:
- `TestEventUnion_UnknownTypeDecodes`: an `EventPage` with one valid `message.created` event and one event of type `future.example` unmarshals without error; the known item resolves through `ValueByDiscriminator`, the unknown one returns an error and still reports its discriminator.
- `TestSchemasGenerated`: compile-time references to `rest.EventEnvelope`, `rest.EventPage`, `rest.Problem`, `rest.GatewayHello`, `rest.GatewayIdentify` and `rest.GatewayReady` (proves `skip-prune`).

Wiring, `internal/wire`:
- `TestAZP1_RoutesMatchEnableList`: with the real generated code the number of mounted routes equals the number of enabled operations (zero at this ticket), each with a known action.
- `TestServe_AbortsOnUnknownAction`: a start with a catalogue that lacks an enabled action (fixture routes) fails before ready; the error names operation and action.

Commands that must pass: `cd services/server && go test -race ./api/... ./internal/wire/...`, `make codegen-check`, `make arch`, `make api-lint`, `make licenses`, `make check`.

## Acceptance criteria

- [ ] `api/operations.yaml` lists every operationId under `paths` exactly once; `enabled` is empty; every `later` entry has an etappe by the rule above.
- [ ] `make codegen-check` regenerates `api/rest`, `api/routes` and the fixture packages without a diff.
- [ ] The generated `rest` package has Go types for all component schemas, including events and gateway frames.
- [ ] Every negative generator fixture fails with a message naming the offending operationId or entry.
- [ ] AZ-P1 tests pass for the fixture and for the real wiring; a start with an unknown action fails before ready.
- [ ] The unknown-event-type test passes.
- [ ] W2 and W6 are checked and recorded in `docs/DECISIONS.md`, together with the P-04 probe result.
- [ ] oapi-codegen v2.8.0 is pinned as a Go tool, the runtime at v1.7.0; all new modules have allowed licenses.
- [ ] `make arch` and `make check` are green.

## Definition of done

- Code and tests as above; `make check` green.
- Documentation: `services/server/api/README.md` (how to regenerate, how a handler ticket enables an operation, what the start check does); doc comment on every exported symbol of `api/routes` (purpose, parameters, return value, errors, example where useful), comments explain why; the 06 "Codegenerierung" point; the `docs/DECISIONS.md` entries; a `services/server/CHANGELOG.md` entry under Unreleased. An ADR only if you take an architecture decision beyond ADR 0013 and P-04 (MADR 4, `make adr-index`).
- Independent review over the diff (`make review` plus a code review) and the red team gate (`docs/quality/red-team-gate.md`) over the diff, with the verdict `JA` or `NEIN` recorded in the report. On `NEIN`: fix, one test per finding, recheck the delta.
- Merge locally into the base branch only if review and red team are green and `make check` passes; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits, files with line counts, test results with counts (including the red run of each new test), what is not proven, and follow-ups that could not be done in scope with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
