# M1-25: Action catalogue in code and registration duty

Status: ready | Depends on: M1-18 | Etappe: M1

## Goal

After this ticket the server carries the closed action catalogue of chapter 05 as generated Go
code: typed identifiers for every action, event type, WebSocket op and resource kind in
`internal/kernel/authorize`, and the catalogue metadata (resource kinds, right, owner, step-up,
audit, rights, rate-limit class) in `internal/authz`. The server refuses to start when a
registered route, op or event type is missing from the catalogue, names a system action or
disagrees with the catalogue's rate-limit class, and an unknown action ends in `500` with an
error log. This makes B7 and ADR 0017 hold mechanically before `authorize` itself exists
(M1-28).

## Read first

- `docs/PLAN.md`: section 3 "Arbeitsweise je Ticket"; section 7 (M1) with the rows M1-02,
  M1-17, M1-18, M1-25, M1-28, M1-67 and the subsection "Voraussetzungen aus dem Kern-Vertrag".
- `docs/spec/05-aktionen.md`: section 1 rules 1, 3, 4, 5 and 10; section 2.1 to 2.5; section 3
  "Rate-Limit-Klassen"; section 4 (all action tables); section 6 (durable and ephemeral event
  types with receive action); section 7 (WebSocket ops); section 9 "Tests, die diesen Katalog
  halten".
- `docs/spec/actions.yaml` and the schema or README of `tools/catalog/` (both from M1-02): this
  is the only input of the generator.
- `docs/spec/07-rechte.md`: section 4.1 (reason `unknown_action`), section 4.2 (first three lines
  of the pseudo-code), section 11.2 (AZ-P1, AZ-P2, AZ-P3).
- `docs/spec/02-architektur.md`: section 4.5 "Querschnitts-Ports" (`kernel/authorize`), 4.6,
  4.7, 5 "Schichten im Modul", 6 "Code-Layout", 7 "Automatische Prüfung der Regeln", 9.3
  "Nachweis der Prüfung", 14.1 "Fehlerarten", 15.1 "Start" step 7.
- `docs/spec/06-api.md`: section 4 "Autorisierung in der API", section 5 "Fehler" (problem code
  `internal_error`).
- ADRs: `docs/adr/0017-eine-autorisierungsfunktion-404-statt-403-registrierungszwang.md`,
  `docs/adr/0008-hexagonale-module-und-gepruefte-abhaengigkeitsregeln.md`,
  `docs/adr/0013-spec-first-openapi-31-und-asyncapi-30-normativ.md`, ADR 0049 (machine-readable
  catalogue, written by M1-02).
- Code of earlier tickets (read before writing anything): `services/server/internal/kernel/authorize`
  (M1-05 skeleton, M1-18 check log), `services/server/api/routes` and
  `services/server/api/operations.yaml` (M1-17), `services/server/internal/wire`,
  `services/server/internal/platform/httpserver` (M1-18), `services/server/internal/kernel/errs`,
  the location of `tools/archgen` (M1-05), the `codegen-check` target in `Makefile` (M1-01),
  `.semgrep/bothy-go-route-registration.yaml` with its fixture (rule format).
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/push-discipline.md`,
  `docs/quality/red-team-gate.md`.
- API operations: none are added or changed. Tables: none. Action ids: all of them (the whole
  catalogue).

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments
   explain why, not what. API documentation is generated from code (OpenAPI from the routes,
   dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour gets a test that is red before the code exists (plan section 3).
7. No en dash or em dash in any text you write; white label (no company, person, host or path
   names; examples use Alex, Sam, Robin and example.com); configuration only through named keys
   from `docs/operations/configuration.md`.
8. B7 and 05 rule 3: every registered route carries a known action and its rate-limit class; the
   server checks this at start (02 section 15.1 step 7).
9. 05 rule 4 and 07 AZ-P2: a WebSocket op or event type without catalogue entry must not be
   registrable.
10. 05 rule 5 and 07 AZ-P3: an unknown action is a programming error; it ends in `500` and an
    alarm (here: an ERROR log line; see Design notes), never in an allow.
11. 05 rule 10 and section 9 test 7: no route and no op names an action whose right is `system`.

## Scope

All paths below are relative to the repository root unless they start with `internal/`, `api/`
or `tools/catalogen`, which are relative to `services/server/`.

1. **Generator** `services/server/tools/catalogen/` (`package main` plus an internal package for
   the logic, place it next to `tools/archgen` from M1-05; if M1-05 put `archgen` elsewhere,
   mirror that location). It reads `docs/spec/actions.yaml` and writes two files:
   - `internal/kernel/authorize/catalog_ids_gen.go`: types `Action`, `EventType`, `Op`,
     `ResourceKind` (all `string` based), one exported constant per catalogue entry, the parse
     functions `ParseAction(string) (Action, bool)`, `ParseEventType(string) (EventType, bool)`,
     `ParseOp(string) (Op, bool)`, `ParseResourceKind(string) (ResourceKind, bool)`, and
     `AllActions() []Action`, `AllEventTypes() []EventType`, `AllOps() []Op`,
     `AllResourceKinds() []ResourceKind` (sorted ascending, fresh slice per call).
   - `internal/authz/domain/catalog_gen.go`: the metadata as domain values keyed by plain
     strings (the domain layer may not import `kernel/authorize`, 02 section 5).
   Both files start with the SPDX line `// SPDX-License-Identifier: Apache-2.0` followed by
   `// Code generated by catalogen. DO NOT EDIT.` and are excluded from coverage and Sonar like
   other generated code (configuration from M1-05).
2. **Wiring of the generator**: a `//go:generate` directive in
   `internal/kernel/authorize/doc.go`:
   `//go:generate go run ../../../tools/catalogen -catalog ../../../../../docs/spec/actions.yaml -ids catalog_ids_gen.go -domain ../../authz/domain/catalog_gen.go`.
   Make sure `make codegen-check` runs it (if `codegen-check` lists generators explicitly, add
   this one).
3. **Catalogue types** in `internal/authz/domain/catalog.go` (hand-written): `ActionSpec`,
   `EventSpec`, `OpSpec` and the lookups `LookupAction(id string) (ActionSpec, bool)`,
   `LookupEvent(t string) (EventSpec, bool)`, `LookupOp(op string) (OpSpec, bool)`,
   `ActionIDs() []string`.
4. **Catalogue service** in `internal/authz/app/catalog.go`: `type Catalog struct`,
   `NewCatalog(log *slog.Logger) *Catalog`,
   `(*Catalog) Spec(a authorize.Action) (domain.ActionSpec, error)` and
   `(*Catalog) VerifyRegistrations(routes []authorize.RouteBinding, reg *authorize.Registry) error`.
5. **Registry** in `internal/kernel/authorize/registry.go`: `type Registry`, `NewRegistry()`,
   `(*Registry) Event(module string, t EventType)`, `(*Registry) Op(module string, op Op)`,
   `(*Registry) Events() []EventRegistration`, `(*Registry) Ops() []OpRegistration`, and
   `type RouteBinding struct { OperationID, Method, Pattern, Action, RateLimitClass string }`.
   Modules register through a function `Register(r *authorize.Registry)` in their port package;
   `internal/wire` calls these functions. No module has to register anything in this ticket;
   later tickets add their types (for example M1-29 registers `access.changed`).
6. **Start check** in `internal/wire`: build the registry, convert the generated route table of
   M1-17 to `[]authorize.RouteBinding`, call `VerifyRegistrations` in the start step that M1-17
   uses for its route check (02 section 15.1 step 7). A failure aborts `bothy serve` with exit
   code 1 and one log line per violation. M1-17 passes `routes.SpecCatalog()` to `routes.Build` in
   `internal/wire`; replace that argument with the closed `authz` catalogue so there is exactly
   one definition of "known".
7. **No second constant set**: if `api/routes` (M1-17) declares its own action constants, make
   `kernel/authorize` the only declaration and regenerate `api/routes` to refer to it.
8. **Semgrep rule** `.semgrep/bothy-go-catalog-id-conversion.yaml` with fixture
   `.semgrep/bothy-go-catalog-id-conversion.go`: forbids converting a string to
   `authorize.Action`, `authorize.EventType`, `authorize.Op` or `authorize.ResourceKind` outside
   files named `*_gen.go`, `*.gen.go` and `*_test.go` (if M1-17's generated route table uses
   another file name, add exactly that path to the rule's exclusions). Code obtains identifiers
   only from constants or the generated `Parse*` functions.
9. **Documentation**: `internal/authz/README.md` (catalogue, start check, how a module registers
   ops and event types), package documentation of `kernel/authorize`, a sentence in
   `docs/spec/02-architektur.md` section 4.5 row `kernel/authorize` that names `Registry`,
   `services/server/CHANGELOG.md` under `Unreleased`.

## Out of scope

- `authorize`, `Decision`, `Reason`, the resource descriptor and the `Authorizer` interface:
  M1-28. Do not add a decision function here.
- The request check log and `authz_not_checked`: M1-18 (already merged; use it, do not change it).
- Generating `api/routes` from `openapi.yaml` and the operation allow-list: M1-17.
- The catalogue file itself, its Python checks and the generated tables in 05 and
  `docs/api/redocly.yaml`: M1-02. Do not edit `docs/spec/actions.yaml` or chapter 05.
- Consistency of 02 tables with the register: M1-06. Frame conformance of the gateway: M1-67.
- Registering real event types or ops of modules: each module's own ticket.
- Metrics such as `bothy.authz.decisions`: M1-68.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-25-authz-catalogue`.
- Conventional Commits with scope (`feat(authz): ...`, `build(server): ...`) and DCO sign-off
  (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` (only if you record a ruling) and the documentation files named in Scope.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them by their literal path at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- If `docs/spec/actions.yaml` lacks information this brief requires (see Design notes, input
  contract), stop with `Verdict: BLOCKED` (plan P-11); do not invent catalogue content.

## Design notes

**Input contract.** The generator needs, per action: `id`, resource kinds, right (as written in
05), owner, step-up, audit (as written in 05), rights, rate-limit class (`keine` meaning none);
per event type: type, durable or ephemeral, resource kind, receive action (as written in 05),
audience self or not; per op: name(s), direction, action(s); per channel op (M1-02 `channel_ops`, the
terminal channels of 05 section 7): channel, op name(s), direction, action. Use the field names
M1-02 chose.
Decode strictly (unknown keys are an error; use the YAML module already required in
`services/server/go.mod`, do not add a second YAML library).

**Normalisation rules** (only for fields that M1-02 stores as 05 text):

| Field | Accepted input | Output |
|---|---|---|
| owner, step-up, rights | `ja`, `nein` (or YAML booleans) | `bool` |
| step-up | additionally `bedingt` / `conditional` and `frisch` / `fresh` | `StepUp = true` plus `StepUpConditional` or `StepUpFresh` |
| rate-limit class | a class from 05 section 3, or `keine` | class string, `""` for none |
| resource | a list of kind names, or 05 text: comma-separated kind names, each optionally followed by a parenthetical note (dropped), or the literal `wie die Zielaktion` | `[]string` (empty for `wie die Zielaktion`) |
| right, audit, receive action, op action | any text | stored raw in `Right`, `Audit`, `ReceiveAction`, `ActionsText` |
| mentioned actions | every token in the raw text (after removing backticks) that equals a catalogue action id | `MentionedActions []string`, sorted |
| op names | one name, or several separated by commas | one `OpSpec` per name |

Anything else fails generation with a message naming the entry and field. `System` is `true`
exactly when the right text equals `system`.

**Identifier rule.** Split the id on `.` and `_`, capitalise the first letter of each part,
join, add the prefix: `Action` for actions, `Event` for event types, `Op` for ops, `Resource`
for resource kinds. Examples: `message.read` gives `ActionMessageRead`,
`agent.billing_mode.set_all` gives `ActionAgentBillingModeSetAll`, `access.changed` gives
`EventAccessChanged`, `command.autocomplete_result` gives `OpCommandAutocompleteResult`,
`auth_session` gives `ResourceAuthSession`. Two ids that map to the same identifier fail
generation. Ids must match `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`.

**Determinism.** Output sorted by id, `gofmt`-formatted, no timestamps; two runs give identical
bytes.

**Domain types** (`internal/authz/domain/catalog.go`):

```go
type ActionSpec struct {
    ID               string
    Resources        []string
    Right            string   // raw 05 text, interpreted by M1-28
    Owner            bool
    StepUp           bool     // true for "ja" and for "bedingt" (05 2.4; until the ticket that
                              // supplies the condition, "bedingt" counts as "ja", 07 4.2)
    StepUpConditional bool    // "bedingt": M1-39 and M1-40 add the condition in authz
    StepUpFresh      bool     // "frisch" (05 2.4): a fresh, unconsumed step-up that the action
                              // consumes; only the terminal actions of 05 4.14, deferred until
                              // M2-40, which evaluates it
    Audit            string   // raw 05 text
    Rights           bool     // the action raises permission_version (05 rule 7)
    RateLimitClass   string   // "" when 05 says "keine"
    System           bool     // Right == "system" (05 rule 10)
    Effect           string   // "read" or "write" (05 2.7); used by the maintenance layer (M7-08)
}
type EventSpec struct {
    Type             string
    Durable          bool
    Resource         string
    ReceiveAction    string   // raw 05 text
    MentionedActions []string
    AudienceSelf     bool
    ResourceVersion  bool     // the envelope carries resource.version (04 "Zustellgarantie" 3)
}
type OpSpec struct {
    Op               string
    Direction        string   // raw 05 text
    ActionsText      string   // raw 05 text
    MentionedActions []string
}
type ChannelOpSpec struct {   // terminal channels (05 section 7, second table); not gateway ops
    Channel          string   // terminal_stream, host_control, host_data
    Op               string   // message name; noise_frame for binary frames
    Direction        string
    Action           string
}
```

Channel ops get their own constants of type `ChannelOp` (value `<channel>/<op>`, identifier
`ChannelOp` plus channel plus op, for example `ChannelOpHostControlSessionOpen`), the lookup
`LookupChannelOp(channel, op string) (ChannelOpSpec, bool)` and the registry methods
`(*Registry) ChannelOp(module string, op ChannelOp)` and `ChannelOps()`. No module registers one in
M1; the terminal tickets of M2 (M2-43, M2-44) do.

**VerifyRegistrations** collects all violations (never stops at the first) and returns one error
listing them, each naming the operation id, op or event type:

1. Route: `Action` parses and is in the catalogue; the action is not `System`; `RateLimitClass`
   equals the catalogue class (an action with class "" can never be a route).
2. Op: the op is in the catalogue; its direction is client or agent to server (server-to-client
   ops are never registered); every mentioned action is known and not `System`.
3. Event type: the type is in the catalogue; every mentioned action of its receive action is
   known; no type is registered by two modules.
3a. Channel op: the pair of channel and op is in the catalogue and its action is not `System`; a
   gateway op name never counts as a channel op and the reverse.
4. An op or event type registered twice by the same module is one registration (idempotent).

**Unknown action.** `Catalog.Spec` with an action that is not in the catalogue returns a
`kernel/errs` error of kind `internal` with code `internal_error` and cause `unknown_action` (02
section 14.1), writes one ERROR log record with the attributes `bothy.action` (the offending
value; it comes from code, never from user input) and `error.type=unknown_action`, and increments
`bothy.authz.errors` with `bothy.reason = unknown_action` (signals.md; alarm `BothyAuthzError`).
The HTTP chain of M1-18 renders it as `500 internal_error` (06 section 5); the action value never
appears in the response. Every `Authorizer` implementation (M1-28) must start with
`Catalog.Spec`.

**Why a registry instead of reading asyncapi.yaml at run time:** the registry lists what this
binary actually emits and accepts, which is what AZ-P2 asks; the documents are checked against
the catalogue by M1-03 and M1-67.

## Tests (write them first)

Go tests run with `-race`. Test names carry the AZ id where one applies.

1. `tools/catalogen`: golden test on `testdata/valid.yaml` (a small catalogue with at least one
   action of each normalisation case including `frisch`, one durable and one ephemeral event, one
   op row with two names, one channel op row) against checked-in golden outputs; `TestGenerate_Deterministic` (two runs, equal
   bytes); failure cases, each with its own fixture and an assertion on the message:
   duplicate action id, identifier collision (`a.b_c` and `a_b.c`), unknown key, owner value
   `vielleicht`, unknown rate-limit class, resource text `thread oder channel`, id `Message.Read`.
2. `internal/authz/domain`: `TestLookupAction_Known` for `message.read` (resources `message`,
   right `read`, class `rl.read`, not system) and `instance.bootstrap` (system, class "");
   `TestLookupAction_Unknown`; `TestEventSpecs_DurableAndEphemeral` for `message.created` and
   `typing.started`; `TestOpSpecs_MentionedActions` for `heartbeat` (mentions `gateway.connect`
   and `agent.heartbeat`).
3. `internal/authz/app`:
   - `TestAZP1_RoutesNeedKnownActionAndClass`: the real route table of `api/routes` passes;
     fixtures with an unknown action, with `instance.bootstrap`, and with a wrong class each fail
     and the error names the operation id.
   - `TestAZP2_OpsAndEventTypesNeedCatalogueEntry`: the real registry built by `internal/wire`
     passes; a registry with `authorize.EventType("no.such.event")`, with
     `authorize.Op("no_such_op")`, with the server-to-client op `hello`, and with one event type
     registered by two modules each fails.
   - `TestAZP3_UnknownActionFailsClosed`: `Spec(authorize.Action("no.such.action"))` returns kind
     `internal` with cause `unknown_action`, exactly one ERROR record (capture with a test
     `slog.Handler`) and one increment of `bothy.authz.errors{bothy.reason=unknown_action}`
     (manual metric reader).
4. `internal/platform/httpserver` (or wherever M1-18 keeps its chain tests):
   `TestAZP3_UnknownActionIs500`: a test route whose handler returns the error of
   `Catalog.Spec` for an unknown action answers `500`, problem code `internal_error`, and the
   body does not contain the action string.
5. `internal/wire`: `TestStartAbortsOnBadRegistration`: the start step with an injected bad
   route binding returns an error and does not open listeners (reuse M1-17's start-abort test
   pattern).
6. `.semgrep/bothy-go-catalog-id-conversion.go`: `ruleid` cases for each of the four
   conversions, `ok` cases for constants and `Parse*` calls.

Commands that must pass:

```sh
cd services/server && go generate ./internal/kernel/authorize/... && git diff --exit-code
cd services/server && go test -race ./tools/catalogen/... ./internal/kernel/authorize/... ./internal/authz/... ./internal/wire/...
make codegen-check
make arch
make semgrep
make check
```

## Acceptance criteria

- [ ] `make codegen-check` regenerates both files without diff.
- [ ] Every action, event type, op and resource kind of `docs/spec/actions.yaml` has exactly one
      constant; no other constants of these types exist outside the generated file.
- [ ] AZ-P1, AZ-P2 and AZ-P3 are green under these names.
- [ ] `bothy serve` exits with code 1 and names each violation when a route, op or event type
      breaks a rule of VerifyRegistrations.
- [ ] Generator failure cases from the Tests section each fail with a message naming entry and
      field.
- [ ] The Semgrep rule finds all four conversions in its fixture and nothing in the code base.
- [ ] `internal/authz/README.md` explains the catalogue, the start check and how modules
      register; every exported symbol has a doc comment.
- [ ] `make check` is green.

## Definition of done

Code, tests and documentation are complete: module README updated, doc comments on every public
symbol, an ADR only if you took an architecture decision not covered by ADR 0017 and ADR 0049,
CHANGELOG entry under `Unreleased` in `services/server/CHANGELOG.md`. `make check` is green.
An independent review and the red team (`docs/quality/red-team-gate.md`) have run over the diff
with their verdicts recorded in the report. Merge locally into the base branch only if both are
green; then remove the worktree with `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test before the code), what is not proven, and follow-ups that could not be done in scope with
the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
