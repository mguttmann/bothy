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
