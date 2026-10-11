# M1-67: Gateway frame conformance (G9)

Status: ready | Depends on: M1-62, M1-66 | Etappe: M1

## Goal

After this ticket every frame the server sends during the gateway integration tests is checked
against its message in `docs/api/asyncapi.yaml` and the body schemas in `docs/api/openapi.yaml`,
and the runtime register of gateway ops and event types is checked against
`docs/spec/actions.yaml` and `asyncapi.yaml`. A frame that drifts from the contract, or an event
type or op registered without a catalogue and AsyncAPI entry, fails `make check`. This is gate G9
from 06 "Vertragstests und Gate" and exit criterion 6 of M1.

## Read first

- `docs/PLAN.md`: section "Arbeitsweise je Ticket"; section "M1 Fundament", row M1-67, rows M1-17,
  M1-25, M1-64 to M1-66 and M1-71; "Teststrategie" of M1; decision P-04 (enable list).
- `docs/spec/06-api.md`: "Grundregeln" (no `null`, integers below 2^53, unknown fields),
  "Echtzeit-Gateway", "Codegenerierung" item 3 (no AsyncAPI generator), "Vertragstests und Gate"
  (G9, G2, G8), "WebSocket-Ops und Ereignisse".
- `docs/spec/04-sync-protokoll.md`: "Umschlag (Envelope)", "Rahmen auf dem WebSocket", "Ops",
  "Schließ-Codes", "Zustellgarantie", "Reihenfolge".
- `docs/spec/05-aktionen.md`: "Verbindliche Regeln" rule 4, "Dauerhafte Ereignistypen und
  Empfangs-Aktion", "Kurzlebige Ereignisse", "WebSocket-Ops und Aktion", "Tests, die diesen
  Katalog halten" item 2.
- `docs/spec/07-rechte.md`: "Pflicht-Tests" AZ-P2.
- `docs/api/asyncapi.yaml` (channel `gateway`, `operations`, `components.messages`,
  `x-close-codes`, `x-receive-action`, `x-event-kind`), `docs/api/openapi.yaml`
  (`components.schemas`: `Gateway*`, `*Event`, `*Payload`, `EventEnvelope`), `docs/spec/actions.yaml`
  (M1-02), `services/server/api/operations.yaml` (M1-17).
- ADRs: `docs/adr/0013-*`, 0049 (action catalogue, written by M1-02), 0053 (no JSON null).
- Code you build on: the gateway test client and in-process server of M1-64 to M1-66, the op
  register and the event-type register (M1-25), `internal/platform/pg/pgtest`.
- `docs/quality/gates.md`, `docs/quality/licenses.md`, `CONTRIBUTING.md`.

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
6. Test first: every behaviour change has a test that was red before the change (record the red
   run in the report), then green, then clean up.
7. No en dash or em dash (U+2013, U+2014) in any file you write.
8. White label: only the examples Alex, Sam, Robin, `example.com` and the areas Support, Marketing,
   Engineering, Research; no paths under `/Users`.
9. Limits only through named keys from `docs/operations/configuration.md`.
10. G9: every frame sent by the server matches its schema; the register of ops and event types
    equals the catalogue (06 "Vertragstests und Gate"; 05 "Tests, die diesen Katalog halten"
    item 2; AZ-P2).
11. Frames are `{"op": <name>, "d": <body>}` text frames of at most `sync.ws_max_frame_bytes`;
    events travel as `dispatch` (04 "Rahmen auf dem WebSocket").
12. No JSON `null` anywhere; integers stay below 2^53 (06 "Grundregeln"; ADR 0053).
13. Durable events carry `seq` and `permission_version`, ephemeral ones carry neither (04
    "Umschlag"); a connection delivers durable events in strictly increasing `seq` (04
    "Zustellgarantie" item 2).
14. Every close code the server sends is one of 04 "Schließ-Codes" and of `x-close-codes`.
15. `resource.version` is present exactly for event types whose catalogue entry says so (04
    "Zustellgarantie" item 3; `EventSpec.ResourceVersion` from M1-25).

## Scope

- `api/specschema/` (new package in the `api` component; test support, imported only by tests):
  loads `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` from the repository root (found by
  walking up from the working directory to the directory that contains `docs/api/openapi.yaml`),
  builds one JSON Schema 2020-12 bundle and exposes:

```go
// Load builds the validator from the two contract files. It fails on any $ref form it does not
// know, so a new reference style cannot silently skip validation.
func Load(repoRoot string) (*Contract, error)
// ValidateServerFrame checks one frame the server sent: message lookup by op (and by d.type for
// dispatch), schema, no null, integer bound, frame size, durable/ephemeral envelope rule, and
// presence of resource.version against the catalogue.
func (c *Contract) ValidateServerFrame(raw []byte) error
// ValidateClientFrame checks one frame a test client sent.
func (c *Contract) ValidateClientFrame(raw []byte) error
// CloseCodes returns x-close-codes (code -> name).
func (c *Contract) CloseCodes() map[int]string
// Operations and EventMessages expose the AsyncAPI register for the register test.
```

- The gateway test client package of M1-64 (if none exists, create
  `internal/gateway/adapters/ws/wstest`): `Start(t, opts)` for the in-process server and
  `Dial(t, ...)` for clients. Every client reads frames in a background goroutine, records them,
  validates each server frame on receipt and each client frame on send, checks per-connection
  order (`seq` of durable `dispatch` strictly increasing; `cursor` and `catchup.complete` never
  below the last delivered `seq`; first server frame `hello`) and the close code. A `Pause()` mode
  for slow-consumer tests stops reading; cleanup drains and validates the rest. Violations fail the
  test in `t.Cleanup` with test name, frame index, op and JSON pointer.
- Refactor every existing gateway integration test (M1-64, M1-65, M1-66) to use this client.
  Negative tests that send invalid frames on purpose opt out of client-frame validation only
  (`wstest.WithRawClientFrames()`); server frames are always validated.
- `TestHarness_OnlyEntryPoint`: parses all `_test.go` files under `services/server` with
  `go/parser` and fails if any of them calls the WebSocket dial function of
  `github.com/coder/websocket` outside the harness package.
- Register test (`TestRegister_MatchesActionsAndAsyncAPI`) with the comparison as a pure
  function `Compare(reg Register, actions Catalogue, async AsyncAPI) []Problem` so a fixture test
  can feed it a broken register.
- `TestConformance_M1EventCorpus`: one scenario that makes every event type an M1 producer emits
  reach a client (messages, reactions, pins, threads including `thread.moved` (M1-62),
  spaces, channels, `user.updated`, `agent.updated`, `instance.updated`, `grant.changed`,
  `group.changed`, `access.changed`, `access.expiring`, `session.revoked`, personal sync events,
  `typing.started`) and every server op M1 sends (`hello`, `ready`, `resumed`, `dispatch`,
  `cursor`, `catchup.complete`, `resync_required`, `heartbeat_ack`, `reauth_required`, `error`).
  It fails if a listed type or op was not observed. Keep the list in the test with a comment, and
  assert that each listed type is registered.
- Gateway op enable list: M1-64 created the section `gateway_ops` of
  `services/server/api/operations.yaml` (PLAN P-04, which carries this decision). The register test also compares it: every op in
  `enabled` is registered, every op in `later` is not, and together they are all client ops of
  `asyncapi.yaml`.
- Dependency: a JSON Schema validator with draft 2020-12 support including
  `unevaluatedProperties`. Reuse one if the module already has it; otherwise add
  `github.com/santhosh-tekuri/jsonschema/v6` (Apache-2.0) pinned in `go.mod`; `make licenses` must
  stay green. Use the YAML library already in `go.mod`.
- Docs: `docs/spec/06-api.md` "Vertragstests und Gate" row G9 (name the harness and the
  commands), `docs/quality/gates.md` if it lists contract tests, `services/server/internal/gateway/README.md`,
  `services/server/CHANGELOG.md` under "Unreleased", `docs/DECISIONS.md` (rulings below).

## Out of scope

- G7 and G8 (Schemathesis, REST response validation): M1-71. If M1-71 is already merged and
  brought a validator, reuse it.
- Document-level comparison of asyncapi.yaml with actions.yaml and 04 (G2): M1-03.
- New gateway behaviour. If a frame fails, fix the producer or, when the server is right by the
  chapter text, fix `openapi.yaml`/`asyncapi.yaml` (spec first, G1 to G3 stay green). If 04 or 05
  (core contract) are themselves contradictory, stop with `Verdict: BLOCKED` (P-11).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-67-gateway-conformance`.
- Conventional Commits with scope (`test(gateway): ...`, `feat(api): ...`) and DCO sign-off.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp`, removed at the
  end. Tests bind `127.0.0.1` and port 0 only; PostgreSQL through `pgtest`.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

Building the schema bundle (deterministic and independent of library quirks)

1. Parse both YAML files to JSON values.
2. `$defs` = every entry of openapi `components.schemas`, plus one entry `msg.<message key>` per
   AsyncAPI message in `components.messages` holding its `payload`.
3. Rewrite `$ref`: `#/components/schemas/X` (openapi) and `./openapi.yaml#/components/schemas/X`
   (asyncapi) become `#/$defs/X`. Any other `$ref` form is a load error.
4. Remove keywords outside JSON Schema 2020-12 from schemas: `discriminator`, `xml`,
   `externalDocs` and every `x-*` key.
5. Strictness: add `unevaluatedProperties: false` to every schema object that has `properties` or
   `allOf` and declares neither `additionalProperties` nor `unevaluatedProperties`, except (a)
   inline schemas that are direct members of an `allOf` array and (b) definitions referenced by a
   `$ref` that is itself an `allOf` member (base schemas such as `EventEnvelope`). Reason: in
   2020-12, `unevaluatedProperties` on the composing schema sees the properties of all `allOf`
   members, while the same keyword on a base schema would reject its siblings' fields. Clients
   ignore unknown fields (06 "Grundregeln"), but the server must not send undeclared ones: an
   undeclared field is contract drift.
6. Root `$schema: https://json-schema.org/draft/2020-12/schema`; enable format assertion so `uuid`
   and `date-time` are checked.

Frame lookup

- `op = dispatch`: message `event.<d.type>`; unknown `d.type` is a failure. Check `x-event-kind`:
  `durable` requires `seq` and `permission_version`, `ephemeral` forbids both.
- Other ops: message `op.<op>`; it must be a server-to-client message (`send.*` operation).
- Beyond the schema: no `null` at any depth; integers at most 2^53 - 1; byte length at most
  `sync.ws_max_frame_bytes`.

Register comparison (`Compare`)

- Each registered client-to-server op exists as `receive.<op>` in asyncapi.yaml with the same
  `x-authz-action` and `x-rate-limit-class`, and as a WebSocket op in actions.yaml with the same
  action.
- Each registered event type exists in actions.yaml with the same kind and receive action, in
  asyncapi.yaml as message `event.<type>` with the same `x-receive-action` and `x-event-kind`,
  and as schema `<PascalCase>Event` in openapi.yaml.
- Each client-to-server op of asyncapi.yaml is either registered (and in `gateway_ops.enabled`)
  or listed in `gateway_ops.later` with an etappe.
- Fixture tests: a register with an extra event type `test.unlisted` yields exactly two problems
  (missing in actions.yaml, missing in asyncapi.yaml); an op registered with a different action
  yields one problem.

Rulings to record in `docs/DECISIONS.md`:

1. G9 validates on the client side of the test harness, not with a hook in production code; a
   parser test keeps the harness the only entry point.
2. Server frames are validated strictly (no undeclared properties) by the bundle rule above.

The gateway op enable list is not a ruling of this ticket; PLAN P-04 carries it.

## Tests (write them first)

- `api/specschema`: `TestLoad_RealContracts` (both files load, every AsyncAPI message compiles),
  `TestLoad_UnknownRefFormFails`, `TestValidate_ValidHelloPasses`,
  `TestValidate_UndeclaredFieldFails`, `TestValidate_NullFails`, `TestValidate_BigIntegerFails`,
  `TestValidate_DurableWithoutSeqFails`, `TestValidate_EphemeralWithSeqFails`,
  `TestValidate_UnknownEventTypeFails`, `TestValidate_ClientOpAsServerFrameFails`,
  `TestValidate_OversizedFrameFails` (fixtures built from the real schemas).
- Harness: `TestHarness_OrderViolationReported`, `TestHarness_UnknownCloseCodeReported`,
  `TestHarness_OnlyEntryPoint`.
- Register: `TestRegister_MatchesActionsAndAsyncAPI`, `TestCompare_UnlistedEventTypeFails`,
  `TestCompare_WrongOpActionFails`.
- `TestConformance_M1EventCorpus` against PostgreSQL 18 with all real modules.
- The whole existing gateway integration suite runs through the harness and stays green.

Commands that must pass:

```sh
cd services/server && go test -race ./api/... ./internal/gateway/... ./internal/wire/...
make test
make licenses
make check
```

## Acceptance criteria

- [ ] Every server frame of the gateway integration suite is validated against asyncapi.yaml and
      openapi.yaml, strictly, with the extra rules above; an invalid frame fails the test.
- [ ] No gateway test opens a WebSocket outside the harness (parser test green).
- [ ] The register test fails for an event type or op without matching entries in actions.yaml
      and asyncapi.yaml (fixture tests red before the implementation, green after).
- [ ] The M1 event corpus and every M1 server op are observed and valid.
- [ ] Close codes sent by the server are all in `x-close-codes`.
- [ ] `make check` and `make licenses` green.

## Definition of done

- Code and tests as above; doc comment on every public symbol; comments explain why.
- 06 row G9 names the harness and how to run it; gateway README and `services/server/CHANGELOG.md`
  updated; rulings in `docs/DECISIONS.md`. Write an ADR only if you take an architecture decision
  beyond this brief.
- `make check` green.
- Independent review (`make review` plus a code review over the diff) and the red team gate
  (`docs/quality/red-team-gate.md`) over the diff, verdict recorded in the report.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then
commits, files with line counts, test results with counts (including the red runs), the number
of validated frames, every contract fix made to openapi.yaml or asyncapi.yaml with its reason,
what is not proven, and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
