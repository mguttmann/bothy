# Changelog

All notable changes to the repository as a whole (specification, documentation, tool
chain and the Compose installation bundle) are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). The version of
this level is the root [`VERSION`](VERSION) file; its release tags are `v<version>`.

Independently versioned components keep their own `VERSION` and `CHANGELOG.md` in their
directory and are tagged `<component>/v<version>` (docs/spec/18-release.md):
server (`services/server/`), client app (`apps/client/`), admin console
(`apps/console/`), Python SDK (`sdk/python/`), TypeScript SDK (`sdk/typescript/`), Hermes
Agent adapter (`plugins/hermes-adapter/`) and Claude Code runner
(`plugins/claude-code-runner/`). Each of these files is created with the component's
first code.

## [Unreleased]

Planning stage (milestone M0). Nothing is installable yet.

### Added

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

### Changed

- Chapter 02 "Konfigurationsladen" and the configuration reference describe the loader as built
  by the server: built-in defaults live in the schema, `bothy config print` shows secret
  references instead of `<redacted>`, and `docs/operations/configuration.md` sections 1 to 3 give
  the exact grammar of every kind (now with `list(cidr)` and `url`), the file rules and the output
  of `bothy config check` and `bothy config print`.
- CodeQL analyses the Go server module (manual build of `services/server`) in the public
  phase; a test keeps the CodeQL languages in step with the tracked components.

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
