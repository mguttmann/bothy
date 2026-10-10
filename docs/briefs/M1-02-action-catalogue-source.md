# M1-02: Action catalogue as the single data source (ADR 0049)

Status: ready | Depends on: none | Etappe: M1

## Goal

After this ticket `docs/spec/actions.yaml` is the only place where actions, rate-limit classes,
event types with their receive action and WebSocket ops are written. The tables of chapter 05 and
the action and class lists in `docs/api/redocly.yaml` are generated from it, and the gate
`catalog-check` fails on any drift or inconsistency. Later tickets read the same file: M1-03 for
the API contract checks, M1-06 for the module register, M1-25 for the Go constants of `authz`.
ADR 0049 records the decision.

## Read first

- `docs/PLAN.md`: section "Arbeitsweise je Ticket"; section "M1 Fundament" (Block A, row M1-02;
  "Voraussetzungen aus dem Kern-Vertrag", row "maschinenlesbarer Aktionskatalog"); P-11, P-12 in
  "Entscheidungen dieses Plans".
- `docs/spec/05-aktionen.md`, the whole chapter, especially "Verbindliche Regeln" (rules 3, 4, 10),
  "Scopes für Apps" (2.6), "Rate-Limit-Klassen", "Aktionen" (4.1 to 4.13), "Dauerhafte
  Ereignistypen und Empfangs-Aktion", "WebSocket-Ops und Aktion", "Tests, die diesen Katalog
  halten", "Konfigurationswerte dieses Kapitels".
- `docs/spec/00-glossar.md` (terms), `docs/spec/04-sync-protokoll.md` "Ops" (6.2) and "Zwei Arten"
  (2.1).
- `docs/spec/02-architektur.md`: "Abgleich mit dem Kern-Vertrag" (row "Maschinenlesbarer
  Aktionskatalog"), "Aktionen je Modul", "Ereignistypen und WebSocket-Ops je Modul" (read only;
  M1-06 checks them).
- `docs/api/redocly.yaml` (rules `authz-action-known`, `rate-limit-class-known`,
  `receive-action-known-message`, `receive-action-known-webhook`), `docs/api/asyncapi.yaml`
  (`operations`, `components.messages` with `x-event-kind`, `x-receive-action`, `x-audience`),
  `docs/api/README.md` ("Ändern").
- ADRs: `docs/adr/0017-eine-autorisierungsfunktion-404-statt-403-registrierungszwang.md`,
  `docs/adr/0013-spec-first-openapi-31-und-asyncapi-30-normativ.md`,
  `docs/adr/0001-adrs-im-madr-format-decisions-als-ruling-log.md`,
  `docs/adr/0000-vorlage-madr-4.md`. ADR 0049 does not exist yet; this ticket writes it (P-12).
  Its title comes from the list of the planning lead: "Maschinenlesbarer Aktionskatalog als
  einzige Quelle für Doku, Lint und Code".
- `docs/quality/gates.md`, `docs/spec/23-qualitaet.md` "Gates", `tools/README.md`,
  `tools/pyproject.toml`, `Makefile` (targets `gates`, `lint`, `api-lint`), `CHANGELOG.md`,
  `docs/DECISIONS.md`.

Action ids: all of chapter 05 section "Aktionen". Event types and ops: all of sections 6 and 7.
No API operation and no table of chapter 03 changes.

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
9. Limits and thresholds are named configuration values from `docs/operations/configuration.md`;
   prose uses the name, never the number. This ticket adds no new value.
10. The catalogue is closed: an unknown action is a program error (05 rule 5); every REST
    operation names exactly one known action and its class (05 rule 3); every op and every event
    type names its action or receive action (05 rule 4); system actions are not reachable over API
    or gateway (05 rule 10, test 7 in "Tests, die diesen Katalog halten").
11. Chapter 05 is core contract and normative (`docs/PLAN.md` "Grundsätze" item 3). The content of
    every table row stays exactly as it is; only its source moves. If a row cannot be expressed
    in the schema below without changing its text, stop with `Verdict: BLOCKED` and name the row
    (P-11).
12. One source per fact: actions, classes, event types and ops exist in `actions.yaml` only; every
    copy is generated and checked (ADR 0049, to be written here).

## Scope

1. `docs/spec/actions.yaml` (new): the catalogue in the schema of "Design notes", first line
   `# SPDX-License-Identifier: Apache-2.0`, then a short English header comment (purpose, ADR
   0049, "edit here, then run `make catalog`").
2. `tools/catalog/action_catalog.py` (new; Python 3.12, standard library plus PyYAML). CLI:
   `action_catalog.py check [--root <repo root>]` (exit 0 clean, 1 findings) and
   `action_catalog.py generate [--root <repo root>]` (rewrites the generated blocks, exit 0, or 1
   if the catalogue itself is invalid; never writes a block from an invalid catalogue). Default
   root: the repository containing the script. Public interface used by M1-03, M1-06 and M1-25,
   each with a docstring:
   - `class CatalogError(Exception)` carrying `findings: list[str]`.
   - Frozen dataclasses `RateLimitClass`, `Action`, `Event`, `WsOp`, `Catalog` with the fields of
     "Design notes"; `Action.system` is a property (`right == "system"`). `Catalog` offers
     `actions_by_id`, `events_by_type`, `ws_ops_by_name`, `classes_by_id` as read-only mappings.
   - `def load_catalog(path: Path) -> Catalog` (validates the schema and every rule that needs only
     the YAML; raises `CatalogError` with all findings).
   - `def generated_blocks(text: str) -> dict[str, str]` and
     `def replace_generated_block(text: str, name: str, body: str) -> str` for Markdown markers
     `<!-- BEGIN GENERATED: <name> -->` and `<!-- END GENERATED: <name> -->` (the form already used
     in `docs/spec/06-api.md`); `body` is everything between the two marker lines and ends with a
     newline. Unbalanced, nested or duplicate markers raise `CatalogError`.
   - `def check(root: Path) -> list[str]` and `def generate(root: Path) -> list[str]` (returns
     the paths it rewrote).
3. `tools/catalog/tests/test_action_catalog.py`, `tools/catalog/tests/table_import.py` (test
   helper, see "Design notes"), `tools/catalog/tests/fixtures/05-aktionen-before.md.txt` (byte copy
   of `docs/spec/05-aktionen.md` from the base commit of this ticket; suffix `.txt` so that
   `make linkcheck` does not check its relative links). `tools/catalog/README.md` (German, short:
   purpose, commands, schema pointer).
4. `docs/spec/05-aktionen.md`: wrap each generated table in markers (names in "Design notes"); the
   rows themselves are produced by `make catalog`. Rewrite rule 3 of "Verbindliche Regeln": the
   sentence "Eine maschinenlesbare Fassung ... ist beschlossen (ADR 0049); bis sie steht, hält ein
   Abgleich ..." becomes: the single source is [actions.yaml](actions.yaml) (ADR 0049); the
   tables of sections 3, 4, 6 and 7 and the lists in `docs/api/redocly.yaml` are generated from it
   (`make catalog`) and `make catalog-check` holds them in the gate. Rule 4: add that sections 6
   and 7 are generated from the same file. No other text of the chapter changes.
5. `docs/api/redocly.yaml`: replace the three hand lists by generated blocks (YAML comment markers,
   see "Design notes"); update the header comment (source `docs/spec/actions.yaml`, regenerate with
   `make catalog`, checked by `make catalog-check`). `make api-lint` must stay green.
6. `Makefile`: targets `catalog` (`## Regenerate the tables of chapter 05 and the lists in
   docs/api/redocly.yaml from docs/spec/actions.yaml`) and `catalog-check` (`## Action catalogue
   valid and its generated copies current (ADR 0049)`); `catalog-check` goes into `gates` directly
   before `api-lint`; add `tools/catalog` to the bandit directory list of `lint`; add both to
   `.PHONY`.
7. `tools/pyproject.toml`: add `catalog` to `testpaths`, `pythonpath`, `[tool.coverage.run]
   source` and `[tool.mypy] files`.
8. `docs/adr/0049-maschinenlesbarer-aktionskatalog-als-einzige-quelle-fuer-doku-lint-und-code.md`
   (MADR 4, German text, English headings, front matter `status: accepted`, `date:` the day of the
   merge, `decision-makers: Maintainer`); then `make adr-index`.
9. Other documents: `docs/spec/02-architektur.md` "Abgleich mit dem Kern-Vertrag", row
   "Maschinenlesbarer Aktionskatalog": now `docs/spec/actions.yaml` (ADR 0049), checked by
   `make catalog-check`. `docs/api/README.md` "Ändern" items 2 and 3: a missing action or a new
   event type enters `docs/spec/actions.yaml` first, then `make catalog`.
   `docs/quality/gates.md`: row `catalog-check`. `docs/spec/23-qualitaet.md` "Gates", group "Doku":
   add `catalog-check`. `tools/README.md`: row for `catalog/`. `CHANGELOG.md` `[Unreleased]`.
   `docs/DECISIONS.md`: the rulings in "Design notes".

## Out of scope

- Checks of `openapi.yaml`, `asyncapi.yaml` and `webhooks.md` against the catalogue, and the
  generated tables of chapter 06: M1-03.
- The tables of chapter 02 ("Aktionen je Modul", "Ereignistypen und WebSocket-Ops je Modul") and
  module ownership of actions: M1-06 (module ownership lives in `architecture/modules.yaml`, ADR
  0050, never in `actions.yaml`).
- Go constants and the closed catalogue in code: M1-25.
- The "Konfigurationswerte dieses Kapitels" table of 05 stays hand-written (it feeds
  `docs/operations/configuration.md`); this ticket only cross-checks it.
- The scope table "Scopes für Apps" (2.6) stays hand-written (its rows mix prose such as
  "Ereignis-Abos"); this ticket cross-checks it.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-02-action-catalogue-source`.
- Conventional Commits with DCO sign-off (`git commit -s`), for example `docs(spec):
  Aktionskatalog als Datenquelle (ADR 0049)` and `build(catalog): ...`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `docs/DECISIONS.md` and `CHANGELOG.md` as
  named.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
  Temporary directories only via `mktemp -d` or pytest `tmp_path`, removed at the end.
- Never read or print secrets. The launcher provides the private deny list; do not set
  `DENYLIST_FLAGS=--allow-missing` or `SONAR_SKIP=1`.
- No network calls except package downloads for pinned tools.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

### Schema of `docs/spec/actions.yaml`

Top level, exactly these keys in this order; unknown keys anywhere are findings:

| Key | Type | Meaning |
|---|---|---|
| `version` | integer `1` | schema version |
| `rate_limit_classes` | list of class | row order of 05 "Rate-Limit-Klassen" |
| `sections` | list of strings | keys of the action tables, in order: `public` (4.1), `instance` (4.2), `identity` (4.3), `grants` (4.4), `structure` (4.5), `threads` (4.6), `messages` (4.7), `files` (4.8), `personal` (4.9), `agents` (4.10), `apps` (4.11), `operations` (4.12), `dev` (4.13) |
| `actions` | list of action | row order of 05 section 4 |
| `events` | list of event | durable rows, then ephemeral rows, in 05 row order |
| `ws_ops` | list of op | row order of 05 "WebSocket-Ops und Aktion" |

Class: `id` (`rl.<name>`), `key` (cell "Schlüssel"), `purpose` (cell "Zweck").

Action (05 column in brackets):

| Field | Type | Required | Renders as |
|---|---|---|---|
| `id` | string, pattern `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`, unique | yes | `` `id` `` (ID) |
| `section` | one of `sections` | yes | (which table) |
| `description` | string, verbatim Markdown | yes | Beschreibung |
| `resource` | string, verbatim | yes | Ressource |
| `right` | string, verbatim; the value `system` marks a system action | yes | Recht |
| `owner` | `true`, `false` or `as_target` | yes | `ja`, `nein`, `wie Zielaktion` (Owner) |
| `step_up` | boolean | yes | `ja` / `nein` (Step-up) |
| `audit` | `always`, `denials`, `never` or `conditional` | yes | `ja`, `abl`, `nein`, or `audit_text` (Audit) |
| `audit_text` | string | only and always with `conditional` | verbatim cell, for example `ja bei fremder` |
| `rights_change` | boolean | yes | `ja` / `nein` (Rechte) |
| `rl_class` | a class id or `none` | yes | `` `rl.x` `` or `keine` (RL-Klasse) |
| `delivery_check` | boolean, default `false` | no | marks the internal delivery checks (`push.deliver`, `webhook_out.deliver`) |
| `app_scope` | one of `messages:read`, `messages:write`, `files:read`, `files:write`, `commands:write`, `components:write`, `users:read`, `events:subscribe`, `costs:reconcile`, `none` | no | not rendered; mirrors 2.6 (`none`: usable by apps without a scope; absent: 2.6 names no scope) |

Event:

| Field | Type | Required | Meaning |
|---|---|---|---|
| `type` | string, same pattern as action ids, unique | yes | Ereignistyp |
| `kind` | `durable` or `ephemeral` | yes | which of the two tables |
| `resource` | string | yes | Ressource |
| `receive_action` | a non-system action id or `self` | yes | machine receive action (equals `x-receive-action` in asyncapi.yaml) |
| `audience` | `self` | no | only the named principal receives it (equals `x-audience: self`); absent means every permitted recipient |
| `receivers` | `principal`, `agent` or `agent_or_app` | only with `audience: self`, default `principal` | who can be that principal; M1-03 uses it for the webhook table |
| `receive_text` | string | no | verbatim cell override when the cell is more than the action |
| `note` | string | no | Hinweis (durable) or Zusatzbedingung (ephemeral); empty when absent |

Receive cell: `receive_text` if set; else `self` (no backticks) for `self`; else `` `id` ``.
`receive_action: self` requires `audience: self`. When set, `receive_text` must contain
`` `<receive_action>` `` (or the word `self` for `self`).

Values for the current rows (from 05 section 6 and `docs/api/asyncapi.yaml`): every event whose
cell starts with `self` has `audience: self`. `receivers: agent` for `agent.invocation`,
`agent.followup`, `run.cancel_requested`, `agent.config_changed`; `receivers: agent_or_app` for
`interaction.created`, `command.invoked`, `command.autocomplete_requested`; all other `self`
events have `principal`. `receive_action` of the composite cells: `agent.events.consume` for
`agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`,
`command.invoked`, `command.autocomplete_requested`; `agent.config.read` for
`agent.config_changed`; `push.deliver` for `notification.created`.

WebSocket op:

| Field | Type | Required | Meaning |
|---|---|---|---|
| `op` | string, unique | yes | Op |
| `direction` | `client_to_server`, `agent_to_server`, `agent_or_app_to_server`, `server_to_client` | yes | renders `Client an Server`, `Agent an Server`, `Agent oder App an Server`, `Server an Client` |
| `action` | non-system action id | yes | the action checked for the op (equals `x-authz-action` in asyncapi.yaml) |
| `also_actions` | list of action ids | no | further checks (`heartbeat`: `agent.heartbeat` for agents) |
| `action_text` | string | no | verbatim cell override |
| `row_group` | string | no | consecutive ops with the same group share one table row |
| `per_event_receive_action` | boolean | no, only on `dispatch` | delivery checks the receive action of each event |

Current values: `identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`,
`typing.stop` are `client_to_server`; `ack` and `run.update` are `agent_to_server`;
`command.autocomplete_result` is `agent_or_app_to_server`; `hello`, `ready`, `resumed`, `cursor`,
`catchup.complete`, `resync_required`, `heartbeat_ack`, `reauth_required`, `error` are
`server_to_client` with action `gateway.connect`, `row_group: session` and the action text
"keine eigene; Teil der Sitzung aus `gateway.connect`"; `dispatch` is `server_to_client` with
action `gateway.connect`, `per_event_receive_action: true` and the action text "Empfangs-Aktion
des Ereignistyps (Abschnitt 6)"; `heartbeat` has `also_actions: [agent.heartbeat]` and the action
text "`gateway.connect` (Sitzung muss gültig sein); bei Agenten zusätzlich `agent.heartbeat`";
`focus` has the action text "`gateway.focus` je Thread". These actions equal `x-authz-action` in
`docs/api/asyncapi.yaml` today; M1-03 keeps them equal.

### Validation rules (`load_catalog` and `check`)

Each rule yields one finding line `actions.yaml: <rule>: <detail>` (or the path of the chapter or
redocly file for block rules):

1. Schema: required keys, types, enums, no unknown keys, `version == 1`.
2. Uniqueness: class ids, action ids, event types, ops.
3. Every `section` is listed; every listed section has at least one action.
4. `rl_class` is a known class or `none`; `rl_class == none` exactly for actions with
   `right: system` or `delivery_check: true` (05 "Rate-Limit-Klassen", last sentence); every class
   is used by at least one action.
5. Every class has the rows `ratelimit.<id>.per_minute` / `.burst` in 05 "Konfigurationswerte
   dieses Kapitels" (row form `` `ratelimit.rl.x.per_minute` / `.burst` ``), and that table names
   no other class.
6. Events: `receive_action` present ("event type without receive action"), known, not a system
   action; the `audience`, `receivers` and `receive_text` rules above.
7. Ops: `action` present ("op without action"), known, not a system action; `also_actions` known;
   members of a `row_group` are consecutive and share `direction`, `action` and `action_text`.
8. 2.6 consistency: parse the table under "Scopes für Apps". In column one every backticked token
   is an action id or a pattern ending in `.*` (prefix match); prose without backticks is ignored.
   In column two the first backticked token is the scope; a cell without one means `none`. Every
   action named there has that `app_scope`, and every action with `app_scope` is named in exactly
   one row.
9. Generated blocks: the set of block names in 05 equals the expected set; each block equals the
   rendering ("generated block differs from docs/spec/actions.yaml; run make catalog"); the same
   for the three YAML blocks in `docs/api/redocly.yaml`.

### Generated blocks

In `docs/spec/05-aktionen.md`, each block contains the whole table including its header line and
separator line:

| Block name | Table | Header |
|---|---|---|
| `rate_limit_classes` | "Rate-Limit-Klassen" | `\| Klasse \| Schlüssel \| Zweck \|` |
| `actions.<section key>` (13 blocks) | each table of section 4 | `\| ID \| Beschreibung \| Ressource \| Recht \| Owner \| Step-up \| Audit \| Rechte \| RL-Klasse \|` |
| `events.durable` | first table of section 6 | `\| Ereignistyp \| Ressource \| Empfangs-Aktion \| Hinweis \|` |
| `events.ephemeral` | second table of section 6 | `\| Ereignistyp \| Ressource \| Empfangs-Aktion \| Zusatzbedingung \|` |
| `ws_ops` | section 7 | `\| Op \| Richtung \| Aktion \|` |

Separator lines are `|---|---|...` with one `---` per column. Row rendering in chapter 05:
`"|" + "".join(f" {cell} |" if cell else " |" for cell in cells)`; an empty cell renders as a
single space (`| |`), as in the current chapter. All prose between the tables (for example
"Kurzlebige Ereignisse:", the paragraph after 4.4, the intro of 4.13) stays outside the markers
and hand-written. Marker lines stand directly before the header line and directly after the last
row, without blank lines, like in chapter 06.

In `docs/api/redocly.yaml` the blocks use YAML comment markers at the indentation of the list
items, for example:

```yaml
      enum:
        # BEGIN GENERATED: authz_actions (make catalog)
        - access.explain
        # END GENERATED: authz_actions
```

| Block | Rule | Content |
|---|---|---|
| `authz_actions` | `rule/authz-action-known` | every non-system action id, sorted |
| `rate_limit_classes` | `rule/rate-limit-class-known` | `none`, then every class id, sorted |
| `receive_actions` | `rule/receive-action-known-message` (anchor `&receive_actions`, reused by the webhook rule) | every non-system action id plus `self`, sorted |

Excluding system actions from both lists is deliberate: 05 test 7 forbids a route or op with a
system action, and no event has one as receive action; the lint now enforces it. Replace the old
comment line `# catalogue (docs/spec/05-aktionen.md section 4)` by the markers. The generator
rewrites only lines between markers and keeps every other byte of the file.

### Migration without content change (proof)

`tools/catalog/tests/table_import.py` is a test helper with
`def import_chapter(chapter: str) -> dict`: it parses the pre-migration tables of chapter 05
(located by their headings in the fixture) into the YAML structure above. Cell mapping:
`ja`/`nein` to booleans, `wie Zielaktion` to `as_target`, Audit `ja`/`abl`/`nein` to
`always`/`denials`/`never` and any other text to `conditional` plus `audit_text`, `keine` to
`none`, backticked `rl.x` to `rl.x`, `delivery_check: true` where the class is `keine` and the
right is not `system`; `app_scope` from 2.6; for event cells that are a single backticked action
or `self` the obvious values, for the composite cells the `receive_action`, `audience` and
`receivers` listed above (a constant table in the helper, so the test depends on nothing but the
frozen fixture); ops as listed above. Cross-check the result once against `x-receive-action` and
`x-audience` in `docs/api/asyncapi.yaml` before committing. Use it once to write
`docs/spec/actions.yaml` (`yaml.safe_dump(..., sort_keys=False, allow_unicode=True, width=100)`
behind the header comment), review the file by hand, and keep the helper only for the fixture
test. The proof that no row changed is twofold: the fixture test below, and
`git diff <base> -- docs/spec/05-aktionen.md` shows only added marker lines plus the edited
sentences of rules 3 and 4 (put that diff summary into the report).

### ADR 0049 content

Context: actions, classes, event types and ops are copied into 05 tables, redocly lists, 02
tables, the asyncapi file and soon Go constants; copies drift silently. Options (at least): (1)
YAML file in `docs/spec/` as source, everything else generated or checked; (2) Markdown tables of
05 as source, parsed by tools; (3) Go constants as source, docs generated from code; (4) `x-`
extensions in openapi.yaml as source. Decision: (1), because it is reviewable next to the chapter,
typed, readable by Python tools and the Go generator, and independent of the API files that only
reference it. Consequences: one more file to edit; the chapter text around the tables stays
hand-written. Confirmation: `make catalog-check` in `gates` and the tests in
`tools/catalog/tests/`; M1-03, M1-06 and M1-25 read the same file.

### Rulings for `docs/DECISIONS.md`

1. The redocly lists of known actions exclude system actions. Reason: 05 test 7; the lint enforces
   what a route test would find later.
2. The scope table 2.6 and the configuration table of 05 stay hand-written and are cross-checked
   by `catalog-check`. Reason: their rows contain prose and the configuration table feeds the
   configuration reference.
3. `actions.yaml` carries no module ownership. Reason: ownership belongs to the module register
   (ADR 0050, M1-06); one fact, one source.

## Tests (write them first)

`tools/catalog/tests/test_action_catalog.py`. Negative cases copy `docs/spec/actions.yaml`,
`docs/spec/05-aktionen.md` and `docs/api/redocly.yaml` into `tmp_path` with the same relative
paths, apply one mutation, run `check(tmp_root)` and assert a finding naming the rule:

- `test_repository_catalog_loads` and `test_repository_blocks_are_current` (real repository, no
  findings).
- `test_render_reproduces_pre_migration_tables`: import the fixture with `table_import`, render
  every block and compare each with the corresponding table of the fixture, byte for byte. The
  test depends only on the frozen fixture, so it stays valid when the catalogue grows later.
- `test_generate_is_idempotent`: `generate` on a copy twice; the second run rewrites nothing; on
  the repository `generate` rewrites nothing.
- `test_duplicate_action_id_fails`, `test_unknown_rate_limit_class_fails`,
  `test_unused_rate_limit_class_fails`, `test_none_class_only_for_system_or_delivery_check`.
- `test_event_without_receive_action_fails`, `test_event_receive_system_action_fails`,
  `test_self_receive_requires_self_audience`, `test_receive_text_must_name_receive_action`.
- `test_op_without_action_fails`, `test_op_with_system_action_fails`,
  `test_row_group_must_be_consecutive`.
- `test_unknown_key_fails`, `test_wrong_version_fails`.
- `test_app_scope_mismatch_with_scope_table_fails` (change one `app_scope`), and
  `test_scope_table_names_unknown_action_fails`.
- `test_missing_ratelimit_config_row_fails`.
- `test_edited_chapter_block_fails` (change one cell in `actions.identity`),
  `test_edited_redocly_list_fails`, `test_unknown_block_name_fails`,
  `test_unbalanced_markers_fail`.
- `test_redocly_lists_exclude_system_actions`.
- `test_cli_exit_codes` (`check` 0 and 1, `generate` on an invalid catalogue exits 1 and writes
  nothing).

Commands that must pass:

```
uv run --project tools --locked pytest tools/catalog/tests
make catalog && git diff --exit-code    # regeneration gives no diff
make catalog-check
make api-lint
make check
```

## Acceptance criteria

- [ ] `docs/spec/actions.yaml` exists and is the only source of actions, classes, event types and
      ops; 05 tables and redocly lists sit in generated blocks.
- [ ] `pytest tools/catalog/tests`: duplicate action id, unknown rate-limit class, event type
      without receive action, op without action and an edited table block fail.
- [ ] A test proves that the 05 tables before and after the change have the same rows
      (`test_render_reproduces_pre_migration_tables`), and the report shows the 05 diff contains
      only markers and the edited sentences of rules 3 and 4.
- [ ] `make catalog-check` and `make api-lint` are green; `make catalog` gives no diff.
- [ ] ADR 0049 exists, is in the ADR index, and names the confirmation.
- [ ] Docs updated: 02 row, `docs/api/README.md`, `docs/quality/gates.md`, chapter 23,
      `tools/README.md`, `tools/catalog/README.md`, `CHANGELOG.md`, `docs/DECISIONS.md`.
- [ ] `make check` green; no file of the diff contains U+2013 or U+2014.

## Definition of done

Code, tests and docs as above: docstrings on every public symbol of `action_catalog.py`; README of
the tool; ADR 0049; CHANGELOG entry under `[Unreleased]`; `make check` green. Then `make review`
and an independent code review over the diff, then the red team gate
(`docs/quality/red-team-gate.md`) over the diff with verdict recorded. Fix every finding with a
test and re-check the delta. Merge locally into the base branch only if the review is clean, the
red team says `JA` and `make check` is green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red runs), the
05 diff summary, what is not proven (for example: consumers M1-03, M1-06 and M1-25 do not exist
yet), and follow-ups that could not be done in scope, each with its reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<merged sha>` or `exit=1 <reason>`.
