# M1-08: Configuration reference generated from the schema

Status: ready | Depends on: M1-07 | Etappe: M1

## Goal

Ruling L4 takes effect: the configuration schema in the server code is the source, and section 4 of
`docs/operations/configuration.md` is generated from the schema plus the chapter tables. A new gate
`make config-doc-check` fails when a name appears twice, when schema and chapter disagree on a
default, when a schema key has no chapter row, when a planned chapter key has no etappe, or when a
retired synonym from W1 reappears anywhere. After this ticket an operator reads one reference that
says, for every key, whether this server version accepts it.

## Read first

- `docs/PLAN.md`: section "Arbeitsweise je Ticket", section "M1 Fundament" (row M1-08, "Bekannte
  Widersprüche, die M1 auflöst" with W1 and the table "Konfigurationsnamen (W1)", "Voraussetzungen
  aus dem Kern-Vertrag" row with `auth.dpop_nonce_ttl`), P-08, P-11, and the etappe tables of all
  sections (needed for planned keys).
- `docs/leitung` rulings as recorded in `docs/DECISIONS.md` (L4).
- `docs/operations/configuration.md` (whole file) and `tools/docs/config_reference.py` with
  `tools/docs/tests/test_config_reference.py` (current generator; its parsing rules stay).
- Chapter tables "Konfigurationswerte dieses Kapitels" in `docs/spec/02-architektur.md`,
  `06-api.md`, `08-identitaet.md`, `09-agenten.md`, `12-dateien.md`, `16-betrieb.md`,
  `19-sicherheit.md`, and the prose of 02 "Leistungsziele" and "Shutdown".
- `services/server/internal/platform/config` and `services/server/internal/wire/config*.go` (schema
  from M1-07), `services/server/architecture/` (register from M1-05).
- `infra/compose/compose.yaml` (anchors `x-api`, services `api`, `api-tailscale`, `worker`).
- Root `Makefile` (targets `docs`, `config-reference`, `codegen-check`, `gates`, `lint`),
  `tools/pyproject.toml`, `tools/README.md`, `docs/quality/gates.md`, ADR 0011.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain why,
   not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API
   docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: write the failing test, run it and keep its red output for the report, make it green,
   then clean up.
7. No en dash (U+2013) or em dash (U+2014) in any file you write; `grep -nP "[\x{2013}\x{2014}]"`
   over your changed files prints nothing.
8. White label: no company, person, host or instance names, no paths of a personal machine, no real
   data.
9. Configuration values only through named keys from `docs/operations/configuration.md`.
10. L4: one source per value; from M1 the schema in code is the source; the reference is generated
    from it and checked against the chapters; no name twice.
11. P-08 (name rule for W1): process keys after 16 and the reference (`server.*`, `database.*`,
    `shutdown.*`), API conventions after 06 (`api.*`), security defaults after 19, domain limits in
    their chapter; with different defaults the stricter one applies until the leading chapter
    decides. When a chapter and `docs/PLAN.md` disagree, the chapter wins and the plan is corrected
    in the same change (PLAN introduction).
12. Leistungsziele (`perf.*`) are thresholds of load tests, not runtime configuration
    (02 "Konfigurationswerte dieses Kapitels").
13. The orchestrator must wait longer than `shutdown.total_timeout` (02 "Shutdown", W1 row
    "Herunterfahren").
14. New repository tools: Python with the standard library or PyYAML, one test file per tool, SPDX
    line in every file, one make target, a row in `tools/README.md`.
15. The core contract (00, 03, 04, 05) is not edited here; a duplicate or synonym that can only be
    fixed there ends the ticket with `Verdict: BLOCKED` (P-11).

## Scope

Create:

- `services/server/tools/configschema/main.go` and `main_test.go`: exports `wire.ConfigSchema()` to
  `services/server/architecture/config-schema.json`; `//go:generate go run . -out
  ../../architecture/config-schema.json` in `main.go`.
- `services/server/architecture/config-schema.json` (generated).
- `tools/configdoc/configdoc.py` (generate and check), `tools/configdoc/planned.yaml`,
  `tools/configdoc/retired.yaml`, `tools/configdoc/tests/test_configdoc.py`.

Change:

- Delete `tools/docs/config_reference.py` and `tools/docs/tests/test_config_reference.py`; their
  parsing logic and test cases move into `tools/configdoc` (one generator, not two).
- Root `Makefile`: target `config-doc-check` (`configdoc.py --check`) in `gates` and `.PHONY`;
  `config-reference` runs `configdoc.py` (write mode); `docs` drops the `config_reference.py` line;
  `tools/configdoc` added to the bandit directories of `lint`. If `make codegen-check` (M1-01) does
  not run `go generate ./...`, add the one line that runs `tools/configschema` before its diff.
- `tools/pyproject.toml`: `configdoc` in pytest `testpaths` and `pythonpath`, coverage `source`, mypy
  `files`.
- `docs/operations/configuration.md`: introduction, section 1, section 4 (generated), section 10
  "Pflege".
- Chapter tables of 02, 06, 08, 09, 12, 16 and 19: only rows the check finds wrong (W1 residue).
  02 already has the column `Ort` (`Server`, `Lasttest` for `perf.*`); Compose already sets
  `stop_grace_period`; the PLAN W1 table already names `http.max_json_body_bytes`.
- `tools/README.md`, `docs/quality/gates.md` (rows `docs` and `config-doc-check`),
  `docs/DECISIONS.md`, root `CHANGELOG.md`, `services/server/CHANGELOG.md`.

## Out of scope

- Adding keys to the schema or changing the loader (M1-07 and each consumer's ticket).
- Secret catalogue (section 5), instance settings (6), feature flags (7), OpenTelemetry (8) and
  Compose variables (9) of `configuration.md` stay hand-written.
- Any other Compose change; making the installation Compose startable (M7).
- Defaults of `*_ref` keys that chapters list as `leer` although 16 "Secret-Provider" defines
  `file:<secrets.dir>/<name>`: they are planned keys; the ticket that adds them to the schema settles
  the default.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-08-config-reference`.
- Conventional Commits with DCO sign-off (`git commit -s`), scope `tools`, `docs`, `spec`,
  `server` or `compose`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except docs and changelog entries named here.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
- Temporary directories only via `mktemp -d`, pytest `tmp_path` or `t.TempDir()`; remove them.
- Never read or print secrets.
- No network calls except package downloads for pinned tools and Go modules, and what the
  repository gates in `make check` do themselves. Run Go with `GOTOOLCHAIN=local`.

## Design notes

### Schema export (`services/server/tools/configschema`)

- Writes JSON with two-space indent and a final newline:
  `{"version": 1, "keys": [{"name", "env", "kind", "default", "required", "enum", "min", "max",
  "chapter", "doc"}]}`, keys sorted by name, every field always present (`enum` as list, others as
  string or bool). `env` is `config.EnvName(name)`; `kind` is `Kind.String()`.
- `tools/` is excluded from go-arch-lint by the register, so this tool may import `internal/wire`.
- `make codegen-check` keeps the JSON current; `config-doc-check` reads only the committed JSON, so
  the documentation gate needs no Go toolchain.

### Chapter tables (rules kept from `config_reference.py`)

Section titled "Konfigurationswerte dieses Kapitels" of every `docs/spec/NN-*.md` except
`23-qualitaet.md` (quality thresholds, held by `tools/docs/tests/test_config_values.py`). First cell:
one name, a list (`` `a.x`, `a.y` ``) or the shorthand (`` `a.b.per_minute` / `.burst` `` with defaults
`30` / `10`). A table with an `Ort` column names the program; otherwise the chapter default place
applies (`01` and `15`: App; `22`: Werkzeug `bothy import`; all others: Server). Local links are
rewritten to `../spec/`.

### Checks of `configdoc.py --check` (each finding `config-doc: <path>[:<line>]: <id>: <message>`)

| Id | Rule |
|---|---|
| D1 | A name appears in more than one chapter table. |
| D2 | A schema key has no chapter row. |
| D3 | A schema key is found in a chapter other than its `chapter` field. |
| D4 | A schema key's chapter row has an `Ort` other than `Server`. |
| D5 | Default mismatch: chapter cell with backticks removed equals the schema `default`; the cell `leer` equals `""`; the cell `keiner, Pflicht` equals `required: true` with `""`. |
| D6 | A chapter key with `Ort` Server that is not in the schema matches no rule of `planned.yaml`, or a rule matches no such key (stale rule). |
| D7 | A retired name of `retired.yaml` occurs in backticks, or its `BOTHY_` form occurs anywhere, in a tracked text file (`git ls-files`) outside the allowlist `docs/PLAN.md`, `docs/DECISIONS.md`, `tools/configdoc/retired.yaml`, `tools/configdoc/tests/`. |
| D8 | A Compose service whose `command` (after YAML merge keys) starts with `serve` has no `stop_grace_period`, or one not strictly greater than `shutdown.total_timeout` (schema default if the key is in the schema, else the chapter 02 default). |
| D9 | Two names map to the same environment variable. |
| D10 | The committed section 4 differs from the rendered one (`run make config-reference`). |

Exit 0 prints `config-doc: ok (<n> keys, <s> in schema, <p> planned)`; findings exit 1; a missing
file or marker exits 2.

### Rendering of section 4

One subsection per chapter as today, table columns
`Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand`:
`Umgebung` only for place Server; `Typ` from the schema (`enum(api, worker, all)` for enums), empty
for planned keys; `Standard` and `Bedeutung` from the chapter; `Stand` is `aktiv` for schema keys,
`geplant (<etappe>)` for planned Server keys, empty for other places. Output is deterministic.

### `planned.yaml` and `retired.yaml`

```yaml
# SPDX-License-Identifier: Apache-2.0
# Etappe of chapter keys with place Server that are not in the schema yet (config-doc rule D6).
rules:
  - match: "file."     # ends with ".": prefix rule; otherwise an exact key name
    etappe: M3
```

Exact names win over prefixes, a longer prefix over a shorter one. Etappe of a key: the etappe of
the first ticket in `docs/PLAN.md` whose row names the key, its prefix or the feature that reads it;
where only the chapter is known, the etappe that builds that chapter's module (PLAN "Schnitt der
Etappen"). Keys whose consumer is an M1 ticket not built yet get `M1`. List uncertain assignments in
the report; the column is information, not a contract.

```yaml
# SPDX-License-Identifier: Apache-2.0
# Names retired by W1 (docs/PLAN.md); config-doc rule D7 keeps them out of the repository.
retired:
  - {name: http.listen_addr, replaced_by: server.listen_addr}
  - {name: http.ops_listen_addr, replaced_by: server.internal_listen_addr}
  - {name: api.trusted_proxy_cidrs, replaced_by: server.trusted_proxies}
  - {name: net.trusted_proxies, replaced_by: server.trusted_proxies}
  - {name: api.max_json_body_bytes, replaced_by: http.max_json_body_bytes}
  - {name: http.max_body_bytes, replaced_by: http.max_json_body_bytes}
  - {name: server.shutdown_grace, replaced_by: shutdown.total_timeout}
  - {name: upload.image_max_pixels, replaced_by: file.image.max_pixels}
  - {name: upload.strip_image_metadata, replaced_by: "file.image.* (12-dateien.md)"}
  - {name: db.acquire_timeout, replaced_by: database.acquire_timeout}
  - {name: db.statement_timeout, replaced_by: database.statement_timeout}
  - {name: db.lock_timeout, replaced_by: database.lock_timeout}
  - {name: db.idle_in_tx_timeout, replaced_by: database.idle_in_tx_timeout}
  - {name: db.tx_retry_max, replaced_by: database.tx_retry_max}
  - {name: db.listen_reconnect_backoff_cap, replaced_by: database.listen_reconnect_backoff_cap}
```

Retired names are exact names, never prefixes: `db.system.name` and other OpenTelemetry attribute
names are legitimate.

### W1 state and what this ticket changes

The chapters already use one name per term for every W1 row (checked 2026-10-10): listeners
`server.*` in 16, proxies `server.trusted_proxies`, database `database.*`, shutdown `shutdown.*`,
enrollment code TTL only in 08, images `file.image.*` in 12, no duplicate rows in 06; the JSON body
limit is `http.max_json_body_bytes` in 02, 06 and 19 (enforced by `platform/httpserver` for every
route class), and `api.max_json_body_bytes` is retired. Re-verify every row when you build; fix
residue in the chapter that P-08 names as leading.

Still open and done here: the guard (D1, D7), the rendering of `Ort` (D-rendering: no environment
name for load test thresholds), and the check D8 that holds Compose `stop_grace_period` (`75s`)
above `shutdown.total_timeout` (`60s` in 02). Verify `auth.dpop_nonce_ttl` occurs only
in the table of 08 (K-ID-8); if 04 still had a row, that is a core contract change: BLOCKED.

### Documentation

- `configuration.md` introduction and section 1: present tense ("Quelle ist das Schema im Code");
  `Stand` explains that planned keys are rejected by this server version as unknown.
- Section 10 "Pflege": a new server key enters the schema (`internal/wire/configkeys_*.go`) and its
  chapter table in the same change; then `go generate ./tools/configschema` and
  `make config-reference`; `make config-doc-check` and `make codegen-check` enforce it.
- DECISIONS.md: one entry per W1 row with its resolution and evidence (D1, D7 green); P-08 applied;
  generator moved from `tools/docs` to `tools/configdoc`.

## Tests (write them first)

`tools/configdoc/tests/test_configdoc.py` (fixtures built in `tmp_path`):

- Moved from `test_config_reference.py`: shorthand `/ .burst`, lists in the first cell, `Ort`
  column, link rewrite, missing markers, wrong default count.
- `test_name_in_two_chapters_fails` (D1)
- `test_schema_key_without_chapter_row_fails` (D2)
- `test_schema_key_in_other_chapter_fails` (D3)
- `test_schema_key_with_non_server_place_fails` (D4)
- `test_schema_default_differs_from_chapter_fails` and `test_required_and_empty_defaults_match` (D5)
- `test_planned_key_rendered_with_etappe`, `test_planned_key_without_rule_fails`,
  `test_stale_planned_rule_fails`, `test_exact_rule_beats_prefix` (D6)
- `test_retired_name_in_backticks_fails`, `test_retired_env_form_fails`,
  `test_retired_name_in_allowlisted_file_passes`, `test_otel_attribute_db_system_name_passes` (D7)
- `test_missing_stop_grace_period_fails`, `test_short_stop_grace_period_fails`,
  `test_merge_key_anchor_is_followed` (D8)
- `test_env_name_collision_fails` (D9)
- `test_check_detects_stale_reference` (D10), `test_render_is_deterministic`
- `test_perf_keys_have_no_env_name`
- `test_real_repository_has_no_duplicates_and_no_retired_names`: runs D1 and D7 against the real
  repository and asserts zero duplicates and zero retired names (the counter the plan asks for).
- `test_real_repository_check_passes`

`services/server/tools/configschema/main_test.go`: `TestExportMatchesCommittedFile`,
`TestExportIsSortedAndDeterministic`.

Commands that must pass:

```sh
uv run --project tools --locked pytest tools/configdoc/tests
cd services/server && GOTOOLCHAIN=local go test ./tools/configschema/...
make config-reference && git diff --exit-code docs/operations/configuration.md
make config-doc-check codegen-check docs linkcheck lint typecheck test coverage
make check
```

## Acceptance criteria

- [ ] `make config-doc-check` is part of `gates` and green; `make config-reference` regenerates
      section 4 without diff.
- [ ] Rules D1 to D10 each have a failing fixture test; all tests pass.
- [ ] The real repository has zero duplicate names and zero retired names (test above).
- [ ] Schema keys show `aktiv` with their type; planned Server keys show `geplant (<etappe>)`;
      `perf.*` rows have no environment name.
- [ ] `infra/compose/compose.yaml` sets `stop_grace_period` above `shutdown.total_timeout` for every
      service that runs `bothy serve`.
- [ ] `tools/docs/config_reference.py` and its test are gone; no reference to them remains
      (`git grep config_reference` prints nothing outside the changelog).
- [ ] The W1 row of `docs/PLAN.md` and DECISIONS.md match the chapters.
- [ ] Tools coverage at or above `quality.coverage.tools_min_percent`; mypy strict, ruff, bandit clean.
- [ ] `make check` is green.

## Definition of done

- Code, tests and docs as above: docstrings on every public function and class, doc comments in the
  Go exporter, rows in `tools/README.md` and `docs/quality/gates.md`, `configuration.md` section 10,
  DECISIONS.md entries, root `CHANGELOG.md` (`Changed`: configuration reference, chapter 02 table,
  Compose; `Added`: `config-doc-check`), `services/server/CHANGELOG.md` (`Added`: schema export).
- `make check` green.
- Independent review over the diff (a fresh reviewer, `make review`) and the red team gate of
  `docs/quality/red-team-gate.md` over the diff, verdict recorded in the report; fix every finding
  with a test and have the delta checked again.
- Merge locally into the base branch only if review and red team are green and `make check` passes
  on the merged state; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits
(SHA and subject), files with line counts, test results with counts (including the red output of
the first test run), the W1 rows with their final state, uncertain etappe assignments in
`planned.yaml`, what is not proven, and follow-ups that could not be done in scope with the reason.
Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
