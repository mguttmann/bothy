# Changelog

All notable changes to the repository as a whole (specification, documentation, tool
chain and the Compose installation bundle) are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). The version of
this level is the root [`VERSION`](VERSION) file; its release tags are `v<version>`.

Independently versioned components keep their own `VERSION` and `CHANGELOG.md` in their
directory and are tagged `<component>/v<version>` (docs/spec/18-release.md):
server (`services/server/`), host service (`services/host/`), client app (`apps/client/`),
admin console (`apps/console/`), Python SDK (`sdk/python/`), TypeScript SDK (`sdk/typescript/`), Hermes
Agent adapter (`plugins/hermes-adapter/`) and Claude Code runner
(`plugins/claude-code-runner/`). Each of these files is created with the component's
first code.

## [Unreleased]

Planning stage (milestone M0). Nothing is installable yet.

### Added

- Design tokens: `make tokens` checks the chart series `color.viz.series.1` to `.8` (adjacent
  pairs and series 8 with series 1) and the traffic light states `success.fg`, `warning.fg` and
  `danger.fg` under simulated protanopia, deuteranopia and tritanopia (Machado et al. 2009,
  severity 1.0) against the minimum OKLab distance `cvd_min_delta_e_ok`; failures name theme,
  deficiency, pair and distance, and `docs/design/contrast.md` lists every row (M6-10). The chart
  roles `color.viz.*`, their contrast pairing and `tokens.py previews --view` come with it (plan
  ruling D-W-13).
- Design system package: `packages/design_system` exports the generated token classes (a copy
  that `tokens.py build` writes next to `design/build/dart/` and `make tokens` compares), a
  settings resolver from `appearance.theme`, `appearance.density`, `appearance.reduce_motion` and
  the instance accent to theme, density, motion and accent, a theme builder with the five token
  extensions and the color scheme mapping, `DsThemeScope` (no animated switch under reduced
  motion), the bundled variable fonts Atkinson Hyperlegible Next and Mono with their OFL texts and
  a checked manifest, and the alchemist golden setup with `dsGoldenMatrix` (M4-04).

- Design tokens: accent variants for every instance accent of `branding.accent_color` in light and
  dark. The generator swaps the accent roles for the same step of the chosen scale, moves a
  failing step, checks every pairing with a swapped role and writes `DsAccent` (Dart),
  `data-accent` blocks (`tokens.css`), the member `accents` of `tokens.resolved.json` and one
  section per accent and theme in `docs/design/contrast.md`; `make tokens` fails below AA (M4-03).
- The docs site has a REST reference, a schema reference and a gateway reference, rendered in
  every `make docs` from `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` by the MkDocs
  hook `tools/docs/mkdocs_api_reference.py`; `make docs` also fails when a built page loads a
  script, stylesheet or font from another origin (`tools/docs/external_assets_check.py`)
  (M7-19).
- One Dart pub workspace at the repository root (`pubspec.yaml`, one `pubspec.lock`) with the
  fourteen members of docs/spec/15-client-architektur.md 3: the apps `apps/client` and
  `apps/console` and the packages under `packages/` as skeletons, `bothy_noise` taken over;
  the Flutter pin moves to the root `pubspec.yaml` (M4-01).
- Architecture test `tools/arch-check` in `make arch`: the dependency rules of 15 3.1 for every
  member, with the reach of the diagram as an allow list (M4-01).
- Documentation in code as a gate for Dart: `public_member_api_docs` as an error in every
  member, and `make docs` puts the `dart doc` pages of every package into the site
  (`docs/reference/dart-api.md`) (M4-01).
- `make apps-platform` and the CI jobs `apps-macos` and `apps-windows` for the platform tests
  of the apps (M4-01).
- Host service module `services/host` (own `VERSION` and `CHANGELOG.md`) and the Dart package
  `packages/bothy_noise` with the Noise core of the terminal (M2-38); the gates, CI and CodeQL
  cover both.
- `plugins/hermes-adapter`: recording harness, probe plugin and fixtures of the live recording
  against the pinned Hermes Agent version (`plugins/hermes-adapter/tests/hermes_pin.toml`); the
  answers of docs/spec/09-agenten.md 17.10 come from it (M2-32).
- The named value `quality.coverage.python_min_percent` also holds every Python project under
  `plugins/` (M2-32).
- CI path filter: a change of docs/spec/09-agenten.md selects the gate component `plugins`.
- Core contract of the specification: glossary, data model, sync protocol and action
  catalogue (`docs/spec/00`, `03`, `04`, `05`).
- Quality and repository tool chain: `make check` with every gate identical to CI,
  pre-commit and pre-push hooks, pinned tools (`tools/`), workflow policy check, deny-list
  scanner for white-label checks, SonarQube profile and gate shared by the local
  Community Build and SonarQube Cloud, Semgrep project rules, gitleaks rules, Renovate.
- Workflows for pull requests, releases, documentation, CodeQL, OpenSSF Scorecard and the
  portability test, with one runner switch for the private and the public phase.
- Community files: license (Apache-2.0), contributing guide with DCO, security policy,
  code of conduct, support, governance, issue and pull request templates.
- Architecture decision records 0000 to 0003, 0038 to 0042 and 0045; quality chapter
  `docs/spec/23-qualitaet.md` and the guides under `docs/quality/`.
- Server gates in the make contract: `codegen-check`, `arch` and `migrations-lint` in
  `make gates`, `make perf` beside them. The gates and `make perf` skip until the first Go
  module is tracked and then fail without Go, a Docker-compatible runtime or the pinned
  go-arch-lint and sqlc. The migration rules of ADR 0007 are checked by
  `tools/ci/migrations_lint.py`; `make migrations-lock` records the checksums of new
  migrations.
- Machine-readable action catalogue `docs/spec/actions.yaml` as the single source of
  actions, rate-limit classes, event types and ops (ADR 0049): `make catalog` generates the
  tables of chapter 05 and the lists in `docs/api/redocly.yaml`, and the gate
  `catalog-check` (`tools/catalog/`) fails on drift or on a contradiction with the
  hand-written parts of chapter 05.
- Gate `register-check` (`tools/doccheck/register_check.py`): the module register
  `services/server/architecture/modules.yaml` must equal the tables and their modules in
  chapter 03 and the actions, event types and ops of `docs/spec/actions.yaml`, and the module
  tables of chapter 02, the server component diagram and the module READMEs must equal the
  register (ADR 0050); every difference is one finding with file and item.
- API contract checks in the gate (06 G2 and G3, `tools/api-checks/`): `make api-checks`
  compares `openapi.yaml` and `asyncapi.yaml` with the action catalogue (actions, classes,
  ops, events, app scopes, the union `Event`) and the close codes of chapter 04, holds
  `x-idempotency-exempt`, the deprecation dates against
  `api.deprecation_min_notice`, `x-problem-codes` and `x-status`, and fails on a stale
  generated table in chapter 06 or `docs/api/webhooks.md`; `make api-tables` regenerates
  these tables. An operation message without `$ref` is a finding, and no input ends in a
  traceback.
- Operations configuration gate `make ops-check` in `make gates` (`tools/opscheck/`): the
  OpenTelemetry Collector base alone, with each overlay and with every ordered pair of
  overlays, the Prometheus configuration, rules and rule unit tests, the Alertmanager
  configuration and routes, Loki, Tempo and the OpenTofu modules are validated with the
  pinned upstream tools; generated SLO rules, the SLO table, the collector allowlist, alert
  thresholds and validator versions are checked against their sources. Seven validators are
  pinned with sha256 (`otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo`, `sloth`,
  `tofu`); `tools/bin/fetch_tools.py` reads zip release assets. Every hand-written alert has
  `promtool` unit tests. Renovate updates the helper images of the Compose stack in one group
  "Compose-Images" and leaves their validators to `make tools-lock`.
- `make config-doc-check` in `make gates` (`tools/configdoc/configdoc.py`): the configuration
  reference against the server schema, the chapter tables, the etappe of planned keys
  (`tools/configdoc/planned.yaml`), the names retired by W1 (`tools/configdoc/retired.yaml`) and
  the Compose `stop_grace_period` of every service that runs `serve` (rules D1 to D10).

### Changed

- Design tokens: series and state roles moved along their scales so that they stay apart under
  the simulation: light `color.viz.series.3` `spruce.600` to `spruce.700`, `.6` `moss.600` to
  `moss.900`, `color.warning.fg` `amber.700` to `amber.800`, `color.danger.fg` `rowan.700` to
  `rowan.900`; dark `color.viz.series.3` `spruce.400` to `spruce.300`, `.7` `rowan.400` to
  `rowan.500`, `color.danger.fg` `rowan.300` to `rowan.400`. Every contrast pairing stays green;
  the previews are regenerated (M6-10).

- Gates for a second Go module and the first Dart package (M2-38): `make perf` runs only for
  the Go modules with performance targets (`PERF_MODULES`), a Dart package without `test/` fails
  `make test` and one without `coverage/lcov.info` fails `make coverage`; CI installs Go for every
  tracked `services/*/go.mod` and Flutter for every tracked Dart package, CodeQL builds the host
  module, a change under `services/host/testdata/noise/` selects the apps gates as well, and
  SonarQube reads the coverage of both new components.
- Chapter 02 "Konfigurationsladen" and the configuration reference describe the loader as built
  by the server: built-in defaults live in the schema, `bothy config print` shows secret
  references instead of `<redacted>`, and `docs/operations/configuration.md` sections 1 to 3 give
  the exact grammar of every kind (now with `list(cidr)` and `url`), the file rules and the output
  of `bothy config check` and `bothy config print`.
- CodeQL analyses the Go server module (manual build of `services/server`) in the public
  phase; a test keeps the CodeQL languages in step with the tracked components.
- `make coverage` leaves generated Go files (`*.gen.go`) out of the Go total, and the license
  gate requires the Go generated-code marker in every file named as generated Go code; a
  directory named `gen` no longer counts as generated for the license gate, Sonar and the lint
  rules gocognit and goconst of the server module.
- The CI path filter selects the server component for a change to `docs/spec/actions.yaml`,
  from which the server generates its route table and action catalogue.
- Configuration reference: section 4 of `docs/operations/configuration.md` is generated from the
  configuration schema of the server plus the chapter tables, with the new columns type and state
  (`aktiv` for keys this server version reads, `geplant (<etappe>)` for keys of a later ticket);
  the generator moved from `tools/docs/config_reference.py` to `tools/configdoc/`, and `make docs`
  no longer checks the reference.
- `docs/PLAN.md`: the W1 rows for trusted proxies and database timeouts and the acceptance test of
  M3-06 follow the chapters.
- `make api-clients` (gate G4) runs `generate-check` of `sdk/python` and `sdk/typescript`, fails
  for a client directory with a tracked manifest but without that target and respects
  `COMPONENTS`; `make bootstrap-ci` installs the locked SDK dependencies (`npm ci
  --ignore-scripts`, `uv sync --locked`); `make typecheck` runs mypy inside each Python project and
  `make lint` keeps bandit out of its `.venv` and generated code. New named values
  `quality.coverage.python_min_percent` and `quality.coverage.typescript_min_percent`.
- The CI path filter selects both SDK components for a change under `sdk/testdata/` (shared G5
  fixtures); CodeQL analyses `javascript-typescript` (the TypeScript SDK); Renovate groups the SDK
  code generators.

- Chapter 12 "4.1 Port" and "4.2 Dateisystem-Treiber (Standard)" describe the port contract
  (key grammar, one byte range, error contract, idempotent `Delete` and `AbortMultipart`) and the
  file system rules (start check of the root, no symbolic links, temporary files in `.tmp`,
  directory removal on delete, multipart layout and ETag) as built by M3-01; chapter 02
  "13. Konfigurationsladen" item 4 lists the cross-check `file_fs_root_absolute`; ADR 0010
  "Confirmation" names the conformance suite and the property test.

### Fixed

- `docs/api/openapi.yaml` 0.2.0: the response `Conflict` names the problem code
  `file_blocked`, which no response listed; `createHost` and `issueHostPairingCode` take
  `Idempotency-Key` and list `422 idempotency_key_reuse` instead of the exemption `auth_flow`,
  as 06 section 7 asks for a POST that returns a one-time secret (`openTerminalSession` and
  `createExtensionSession` keep `auth_flow`); by 06 section 13 this changes the idempotency
  rule of two operations, a breaking change made while the API is still `0.x`; the enum
  `AppScope` contains `costs:reconcile`, the scope of `cost.reconcile.submit` (05 section
  2.6).
- `docs/api/asyncapi.yaml` 0.1.1: `dispatch` lists the event types in the order of the
  catalogue; the event table of chapter 06, generated from it, now follows that order too.
- Chapter 06, section 7: the table of operations without `Idempotency-Key` lists the host
  pairing and terminal session operations that `openapi.yaml` exempts.
- The alert `BothyMalwareScanFailing` now fires when no malware scan has ever produced a
  result; before, it stayed silent exactly when the scanner never worked since the start.
- `make ops-check`: `slo-docs` follows `ticket_alert.disable` (a row may say `nur Page` or
  `keine`) and reads the `le` bound in every PromQL quoting; `tofu` also checks modules made
  only of `.tofu` or JSON files; `alertmanager` parses the templates named by their
  `/etc/alertmanager/` path and fails on an entry that matches no file or lies outside the
  mount; `collector` fails on a reference to a config provider other than `env`, `file` and
  `yaml` (`${https:...}`, `${s3:...}`) before any `validate`; SIGTERM stops a run like
  SIGINT, killing the running validators and removing the work directory. The alert unit
  tests now also catch an added `for` on an alert without one.
- `tools/bin/fetch_tools.py` reports an empty, cut off or corrupt release archive as an error
  instead of a traceback.
