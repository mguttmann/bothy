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
