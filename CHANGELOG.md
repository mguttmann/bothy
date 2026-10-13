# Changelog

All notable changes to the repository as a whole (specification, documentation, tool
chain and the Compose installation bundle) are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). The version of
this level is the root [`VERSION`](VERSION) file; its release tags are `v<version>`.

Independently versioned components keep their own `VERSION` and `CHANGELOG.md` in their
directory and are tagged `<component>/v<version>` (docs/spec/18-release.md; the list of
components is `RELEASE_COMPONENTS` in `tools/ci/components.py`): server
(`services/server/`), host service for the terminal (`services/host/`), client app
(`apps/client/`), web console (`apps/console/`), Python SDK (`sdk/python/`), TypeScript SDK
(`sdk/typescript/`), Hermes Agent adapter (`plugins/hermes-adapter/`), Claude Code runner
(`plugins/claude-code-runner/`) and Grafana panel plugin (`plugins/grafana-bothy-live/`).
Each of these files is created with the component's first code.

## [Unreleased]

Planning stage (milestone M0). Nothing is installable yet. Bothy is planned as a command
center for AI agents in chat form: chat, live view, grants, terminal, costs and
statistics in one open-source app that you host yourself. This level holds the complete plan for
building it: what the product does, the rules it must never break, how it is built and
operated, and the order of the build.

### Added

- Rules for AI coding agents (`AGENTS.md`, loaded by `CLAUDE.md`): public-repository
  hygiene, English only, one pull request per ticket, review findings fixed before merge.
- Specification entry point (`docs/SPEC.md`) with goals, non-goals, hard invariants,
  system overview, chapter index and reading paths per role.
- Specification chapters 00 to 25 (`docs/spec/`): core contract (glossary, data model,
  sync protocol, action catalogue), product with screens as wireframes, architecture,
  API conventions, permissions, identity and devices, agents and reference adapters,
  backend policy with invariants I1 to I6, extensions and SDKs, files, push and
  notifications, search, client architecture, operations, observability, release,
  security, data protection, isolated development environments, Discord import, quality,
  terminal and visualization.
- Terminal as a core feature: host service `bothy-host` on connected hosts, an
  end-to-end encrypted stream (Noise) relayed by the server, a `terminal` capability per
  host, fresh confirmation on every open, audit of every session and invariant I6
  for hosts with a personal AI subscription (`docs/spec/24-terminal.md`, ADR 0068,
  ADR 0070).
- Visualization on two levels: the server's own measurement with a live stream
  `stats.live`, native views in the apps and the web console (live topology of agent
  runs, backend view, delivery, executive page, timeline, TV mode), and the Grafana panel
  plugin "Bothy Live" in the same design language (`docs/spec/25-visualization.md`,
  ADR 0069).
- Normative API contracts: OpenAPI 3.1, AsyncAPI 3.0 and webhooks (`docs/api/`).
- Architecture decision records 0000 to 0070 with a generated index (`docs/adr/`).
- Architecture diagrams as Mermaid sources with rendered SVGs: C4 levels 1 to 3, request
  and event paths, sequences and entity-relationship diagrams (`docs/architecture/`).
- Build plan with milestones, exit criteria and build tickets (`docs/PLAN.md`), a public
  roadmap and one build brief per ticket of milestone M1 (`docs/briefs/`).
- Ruling log, open questions for the owner and product risks (`docs/DECISIONS.md`,
  `docs/OPEN-QUESTIONS.md`, `docs/RISKS.md`).
- Design system: design tokens as the single source with a generator, palette with
  contrast checks, typography, icons, components and preview images in light and dark
  (`design/`, `docs/design/`), including the terminal view with its own color roles
  (16 ANSI colors, cursor, selection) and the chart roles for the statistics views.
- Planned one-command install: a setup wizard (`install.sh`) that checks the host, fetches
  the bundle of its version with a checksum, asks for address, edge and first owner,
  starts the stack and prints the enrollment code (`docs/spec/16-operations.md`, section 3.1).
- Entry screens for every app: server address, login, enrollment code, device waiting
  for approval, pairing, app lock and signed out (`docs/spec/01-product.md`, section 5.6).
- Planned reference integrations: GitHub and Jira apps, monitoring alerts and cron
  messages through incoming webhooks (`docs/spec/11-extensions.md`, section 10).
- Operations: Compose installation bundle, OpenTelemetry Collector configuration,
  monitoring stack with dashboards, SLOs and alerts, OpenTofu and Helm skeletons, and
  guides for configuration, backup and restore, disaster recovery, upgrade and moving to
  a cloud (`infra/`, `docs/operations/`, `docs/observability/`).
- Threat model, and templates for the record of processing activities and the list of
  processors (`docs/security/`, `docs/compliance/`).
- Quality and repository tool chain: `make check` with every gate identical to CI,
  pre-commit and pre-push hooks, pinned tools (`tools/`), workflow policy check, deny-list
  scanner for white-label checks, SonarQube profile and gate shared by the local
  Community Build and SonarQube Cloud, Semgrep project rules, gitleaks rules, Renovate.
  `make docs` also checks that every status with a problem code named in the documentation
  is registered in `x-problem-codes` (`tools/docs/problem_code_refs.py`). The
  `secret-formats` hook rejects values in the Stripe webhook secret format (`whsec_`) that
  GitHub secret scanning reports, the EXAMPLE marker exempts only this product's own token
  formats and generic key-like values from gitleaks, and the tool tests run without a git
  identity, like a fresh CI runner.
- English as the language of all repository content (D-W-94): the term list `docs/glossary.md`
  maps the German terms of earlier planning material, and `make docs` fails on German text in
  tracked files (`tools/docs/language_check.py`).
- Workflows for pull requests, releases, documentation, CodeQL, OpenSSF Scorecard and the
  portability test, all on GitHub-hosted runners with fixed labels.
- Documentation site (MkDocs) whose navigation lists every page; a page missing from the
  navigation fails the strict build. Its start page shows the product with a preview, the
  six pillars, the planned quick start and a reading path per role; `make linkcheck`
  rejects links to the placeholder repository.
- Community files: license (Apache-2.0), contributing guide with DCO, security policy,
  code of conduct, support, governance, issue and pull request templates.
- Folder structure with READMEs for apps, packages, plugins, SDKs, services and each
  server module.
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
- Gate `release-pins` (`tools/ci/components.py pins`): the server version that the product
  bundle pins in `infra/compose/.env.example` is SemVer or the placeholder, and only a product
  beta pins a server pre-release (D-W-98).
- `runners-check` finds release jobs by every spelling of the environment name, rejects an
  environment name with an expression (`environment-not-literal`, D-W-110) and rejects
  `actions: write` and `write-all` on top level and on jobs (`actions-write`, D-W-111).
- Core contract for the briefs of M2 and M3: check without a request (R-AZ-29), the action
  `cost.reconcile.read` with `listCostEstimates`, the system action `webhook_out.transition`,
  the durable event `device.pending`, hints without a thread (R-AZ-25 to R-AZ-31), and the
  data model rules R-DM-42 to R-DM-48; rulings D-W-100 to D-W-108 and OQ-24.
- `brief_deps.py` fails when a start check of a brief names a ticket outside the brief's
  dependency closure, and when a brief calls a ticket of its own closure missing or optional
  (D-W-113).
- Build briefs of the milestones M2 and M3 (`docs/briefs/M2-*.md`, `docs/briefs/M3-*.md`) in the
  brief index with waves and graph, and in the site navigation (D-W-100).
- Build briefs of milestone M4 (`docs/briefs/M4-*.md`, desktop clients) in the brief index with
  waves and graph and in the site navigation, written before M1 closes (D-W-114, which also
  corrects the column "Depends on" of nine M4 tickets in the plan); a docs test holds the shape of
  every brief (title, status line, sections, report placeholders).

- Gate `register-check` (`tools/doccheck/register_check.py`): the module register
  `services/server/architecture/modules.yaml` must equal the tables and their modules in
  chapter 03 and the actions, event types and ops of `docs/spec/actions.yaml`, and the module
  tables of chapter 02, the server component diagram and the module READMEs must equal the
  register (ADR 0050); every difference is one finding with file and item.
- `make config-doc-check` in `make gates` (`tools/configdoc/configdoc.py`): the configuration
  reference against the server schema, the chapter tables, the milestone of planned keys
  (`tools/configdoc/planned.yaml`), the names retired by W1 (`tools/configdoc/retired.yaml`) and
  the Compose `stop_grace_period` of every service that runs `serve` (rules D1 to D10).
- Workflow `sonar-config.yml` (manual, `main` only): applies the shared quality gate and
  quality profiles of `quality/sonar/` to SonarQube Cloud with `make sonar-config` and checks
  them with `make sonar-config-check` (public-switch.md, step 12).

### Fixed

- `quality_sync.py` addresses the quality gate by its id on SonarQube Cloud, whose
  `api/qualitygates/create_condition` and `set_as_default` take `gateId` and `id` instead of
  the gate name (https://sonarcloud.io/api/webservices/list, retrieved 2026-10-13), and lets the profiles inherit there from `cloud_parent` of
  `quality/sonar/profiles.json` ("Sonar way comprehensive"), because the cloud has no built-in
  profile "Sonar way" (ADR 0040); before, `make sonar-config` could not apply the setup to the cloud.
- `quality_sync.py` on SonarQube Cloud: when the plan refuses a custom default quality gate
  (HTTP 403 on `api/qualitygates/set_as_default`), `make sonar-config` still applies the
  quality profiles of every language and `make sonar-config-check` reports the gap
  "gate not default: plan does not allow it" instead of failing; every other difference
  still fails the check. On a SonarQube Server the check now also fails when the gate is not
  the default. Before, the first refusal stopped the run before any profile was applied.
- `quality_sync.py` prints the reason the SonarQube Web API gives for a refused call (the
  `errors[].msg` of the response, shortened) after the HTTP status; before, a refusal such as
  HTTP 400 on `api/qualityprofiles/set_default` showed only the status and could not be
  diagnosed from the run log.
- `docs/api/openapi.yaml` 0.2.0: the response `Conflict` names the problem code
  `file_blocked` in the order of the codes; `createHost` and `issueHostPairingCode` take
  `Idempotency-Key` and list `422 idempotency_key_reuse` instead of the exemption `auth_flow`,
  as 06 section 7 asks for a POST that returns a one-time secret (the host and terminal
  operations `pairHost`, `openTerminalSession` and `attachTerminalSession` keep `auth_flow`,
  ruling M1-03/6); by 06 section 13 this changes the idempotency rule of two operations, a
  breaking change made while the API is still `0.x`.
- `docs/api/asyncapi.yaml` 0.1.1: `dispatch` lists the event types in the order of the
  catalogue; the event table of chapter 06, generated from it, now follows that order too.
- Chapter 06, section 7: the table of operations without `Idempotency-Key` lists the host
  pairing and terminal session operations and `renewInvocationToken`, which `openapi.yaml`
  exempts.
- The alert `BothyMalwareScanFailing` now fires when no malware scan has ever produced a
  result; before, it stayed silent exactly when the scanner never worked since the start.
- `make ops-check`: `slo-docs` follows `ticket_alert.disable` (a row may say `Page only` or
  `none`) and reads the `le` bound in every PromQL quoting; `tofu` also checks modules made
  only of `.tofu` or JSON files; `alertmanager` parses the templates named by their
  `/etc/alertmanager/` path and fails on an entry that matches no file or lies outside the
  mount; `collector` fails on a reference to a config provider other than `env`, `file` and
  `yaml` (`${https:...}`, `${s3:...}`) before any `validate`; SIGTERM stops a run like
  SIGINT, killing the running validators and removing the work directory. The alert unit
  tests now also catch an added `for` on an alert without one.
- `tools/bin/fetch_tools.py` reports an empty, cut off or corrupt release archive as an error
  instead of a traceback.

### Changed

- CodeRabbit reviews with the `chill` profile and reports bugs, security issues, broken
  contracts and problems that break CI, without style, wording or formatting nits; it asks
  for a source URL and retrieval date only in `docs/spec/**`, `docs/security/**` and
  `docs/adr/**` (CRT-1 in [DECISIONS.md](docs/DECISIONS.md)).
- Chapter 02 "Configuration loading" and the configuration reference describe the loader as built
  by the server: built-in defaults live in the schema, `bothy config print` shows secret
  references instead of `<redacted>`, and `docs/operations/configuration.md` sections 1 to 3 give
  the exact grammar of every kind (now with `list(cidr)` and `url`), the file rules and the output
  of `bothy config check` and `bothy config print`.
- `make coverage` leaves generated Go files (`*.gen.go`) out of the Go total, and the license
  gate requires the Go generated-code marker in every file named as generated Go code; a
  directory named `gen` no longer counts as generated for the license gate, Sonar and the lint
  rules gocognit and goconst of the server module.
- The CI path filter selects the server component for a change to `docs/spec/actions.yaml`,
  from which the server generates its route table and action catalogue.
- CodeQL analyses the Go server module (manual build of `services/server`) in the public
  phase; a test keeps the CodeQL languages in step with the tracked components.
- Configuration reference: section 4 of `docs/operations/configuration.md` is generated from the
  configuration schema of the server plus the chapter tables, with the new columns type and
  status (`active` for keys this server version reads, `planned (<milestone>)` for keys of a later
  ticket); the generator moved from `tools/docs/config_reference.py` to `tools/configdoc/`, and
  `make docs` no longer checks the reference.
- `docs/PLAN.md`: the W1 rows for trusted proxies and database timeouts and the acceptance test of
  M3-06 follow the chapters.
- Pull requests of the dependency bots Dependabot and Renovate pass CI like fork pull
  requests: a run the bot starts gets the generic deny rules and skips SonarQube with a
  "Reduced checks" notice, because GitHub gives Dependabot runs no secrets, and the bot's own
  commits in its own pull request need no DCO sign-off; Conventional Commits still apply, and a
  maintainer's edit of the pull request runs every check (D-DEPS-3).

### Security

- Development tools: `katex` (pulled in by mermaid-cli) is pinned through `overrides` to
  0.18.10, which has the fix for
  [GHSA-238p-pmpm-9mq7](https://github.com/advisories/GHSA-238p-pmpm-9mq7); mermaid still
  asks for 0.16, so the Dependabot update could not resolve it. mermaid's own bundle still
  carries katex 0.16 code, which the diagrams do not reach because they contain no math
  (D-DEPS-1 in [DECISIONS.md](docs/DECISIONS.md), whose sources were retrieved on
  2026-10-11).
- Development tools:
  [GHSA-86w9-cpqp-85rv](https://github.com/advisories/GHSA-86w9-cpqp-85rv) in node-forge
  (pulled in by `@sonar/scan`) has no fixed release and is not reachable from the
  scanner's use of node-forge; the alert is dismissed and the OSV exception keeps its
  expiry date (D-DEPS-2 in [DECISIONS.md](docs/DECISIONS.md), whose sources were
  retrieved on 2026-10-11).
- Pull request title and body are deny-list scanned: they become the squash commit on
  `main`. Both deny-list jobs of `ci.yml` read them from the event payload through the
  environment (`PR_TITLE`, `PR_BODY`) and fail `ci-ok` on a finding, `ci.yml` runs again
  when title or body is edited, and `make denylist` checks the planned squash message
  locally (`PR_TITLE`, `PR_BODY` or `PR_BODY_FILE`). The review report stays local; pull
  request bodies are written fresh, without raw reviewer output or local paths
  (`AGENTS.md`, [push discipline](docs/quality/push-discipline.md)). CodeRabbit writes its
  summary into the walkthrough comment instead of the body (`high_level_summary_in_walkthrough`
  in `.coderabbit.yaml`, documented in <https://coderabbit.ai/integrations/schema.v2.json>,
  retrieved on 2026-10-12). The `ci-ok` job starts with
  every run and waits for the other jobs (`tools/ci/ci_ok.py`), so an older green result is
  no longer the latest one while the run after an edit is going (the remaining gaps are
  listed in the push discipline), and identity trailers count only as
  `Name <local@domain>`, with everything but the local part of the address checked by the
  generic rules (LG/1 to LG/5 in [DECISIONS.md](docs/DECISIONS.md)).
