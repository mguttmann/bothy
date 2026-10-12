# Toolchain

Every tool of the gates with version, purpose, place of running, license and reason. A test keeps
the versions on this page equal to the pins (`tools/docs/tests/test_toolchain_doc.py`):
`tools/uv.lock`, `tools/node/package-lock.json`, `tools/bin/tools.lock.json`, `Makefile`,
`tools/sonarqube/compose.yaml`, `.pre-commit-config.yaml` and the workflows.

Place of running: "local" means via `make` on the developer's host, "CI" means in the
same make target in GitHub Actions. All gate tools run in both places.

## Gate tools

| Tool | Version | Purpose | License | Why this one |
|---|---|---|---|---|
| `uv` | system tool | Python environment and lock for all Python tools | MIT OR Apache-2.0 | one lock with hashes for all Python tools; downloads the Python version itself |
| `pyyaml` | `6.0.3` | reading YAML in the check scripts | MIT | standard for YAML in Python; `on:` is deliberately not read as a boolean |
| `ruff` | `0.17.0` | format and lint for Python | MIT | replaces several linters, fast |
| `mypy` | `2.4.0` | type checking (strict) | MIT | reference type checker for Python |
| `bandit` | `1.9.4` | security patterns in Python | Apache-2.0 | required by the requirement; adds security rules to ruff |
| `pytest` | `9.1.1` | tests of the tools | MIT | standard |
| `pytest-cov` | `7.1.0` | coverage in pytest | MIT | standard |
| `coverage` | `7.16.2` | coverage threshold | Apache-2.0 | basis of pytest-cov |
| `pre-commit` | `4.6.2` | hooks for commit, commit-msg, pre-push | MIT | widespread, hooks as a file in the repository |
| `pre-commit-hooks` | `v6.0.0` | file hygiene hooks | MIT | maintained standard hooks, pinned by commit SHA |
| `check-jsonschema` | `0.38.2` | schema check of issue forms, Renovate and CodeRabbit configuration | Apache-2.0 | ships the schemas of the formats |
| `zizmor` | `1.30.1` | security analysis of the workflows | MIT | required by the planning basis; checks injection, permissions, pinning |
| `mkdocs` | `1.6.1` | docs site | BSD | ADR 0045 |
| `mkdocs-material` | `9.7.7` | theme of the docs site | MIT | ADR 0045 |
| `semgrep` | `1.180.0` | project rules (`.semgrep/`) | LGPL-2.1 | pattern rules for project invariants without an own parser (R-QA-05) |
| `@mermaid-js/mermaid-cli` | `11.17.0` | rendering diagrams | MIT | reference renderer of Mermaid; major 11 as at planning time |
| `puppeteer` | `25.13.0` | browser control for mermaid-cli | Apache-2.0 | dependency of mermaid-cli; determines the browser version |
| `katex` | `0.18.10` | math typesetting inside Mermaid diagrams | MIT | dependency of mermaid-cli and mermaid, which still ask for 0.16; pinned through `overrides` in `tools/node/package.json` to a release with the fix for GHSA-238p-pmpm-9mq7 (https://github.com/advisories/GHSA-238p-pmpm-9mq7, ranges in https://registry.npmjs.org/@mermaid-js/mermaid-cli/11.17.0 and https://registry.npmjs.org/mermaid/11.17.2, retrieved 2026-10-11; D-DEPS-1) |
| `@redocly/cli` | `2.62.1` | lint of OpenAPI and AsyncAPI | MIT | lints both formats with its own rules |
| `@sonar/scan` | `5.0.1` | SonarScanner without a local Java installation | LGPL-3.0 | same scanner locally and in CI, no Docker for the scan |
| `actionlint` | `1.7.12` | syntax and expressions of the workflows | MIT | required by the planning basis |
| `gitleaks` | `8.30.1` | secrets in all commits | MIT | as a CLI, not as an action (license obligation of the action in organizations) |
| `golangci-lint` | `2.14.0` | Go linter bundle including gosec and depguard | GPL-3.0 | standard for Go; dependency rules of the modules (chapter 02) |
| `go-arch-lint` | `1.19.0` | module boundaries of the Go modules from the module register | MIT | checked in 02 "go-arch-lint" on a skeleton of all modules; checks packages and third-party libraries |
| `sqlc` | `1.31.1` | typed Go code from SQL, drift check | MIT | ADR 0007 |
| `govulncheck` | `v1.8.0` | reachable Go vulnerabilities | BSD-3-Clause | checks call graphs instead of only versions |
| `go-licenses` | `v2.0.1` | licenses of linked Go packages | Apache-2.0 | required by the research ruling |
| `lychee` | `0.24.2` | links and anchors in Markdown | Apache-2.0 OR MIT | checks anchors in Markdown files offline |
| `osv-scanner` | `2.6.0` | vulnerabilities and licenses of all lock files | Apache-2.0 | one tool for Go, Dart, Python and npm |
| `trivy` | `0.69.3` | misconfiguration and image vulnerabilities | Apache-2.0 | image scan at release; version see below |
| `otelcol-contrib` | `0.162.0` | `otelcol-contrib validate` for the collector base, every overlay and pairs of two overlays (`ops-check`) | Apache-2.0 | the same distribution and version as the collector image (`BOTHY_OTELCOL_VERSION`); only it knows the contrib components of the overlays |
| `promtool` | `3.15.0` | `promtool check config`, `check rules` and `test rules` (`ops-check`) | Apache-2.0 | check tool from the Prometheus release of the image version (`BOTHY_PROMETHEUS_VERSION`) |
| `amtool` | `0.34.1` | `amtool check-config` and routing tests (`ops-check`) | Apache-2.0 | check tool from the Alertmanager release of the image version (`BOTHY_ALERTMANAGER_VERSION`) |
| `loki` | `3.7.8` | `loki -verify-config` (`ops-check`) | AGPL-3.0-only | only Loki itself knows its configuration keys; version of the image (`BOTHY_LOKI_VERSION`). It is only run for the check and not distributed; AGPL-3.0 applies to distribution and to network use of modified versions ([AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html), sections [0](https://www.gnu.org/licenses/agpl-3.0.html#section0) "convey" and [13](https://www.gnu.org/licenses/agpl-3.0.html#section13), retrieved 2026-10-12), and neither happens here |
| `tempo` | `3.1.0` | `tempo -config.verify=true` (`ops-check`) | AGPL-3.0-only | only Tempo itself knows its configuration keys; version of the image (`BOTHY_TEMPO_VERSION`). It is only run for the check and not distributed; AGPL-3.0 applies to distribution and to network use of modified versions ([AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html), sections [0](https://www.gnu.org/licenses/agpl-3.0.html#section0) "convey" and [13](https://www.gnu.org/licenses/agpl-3.0.html#section13), retrieved 2026-10-12), and neither happens here |
| `sloth` | `0.16.0` | regenerate the SLO rules and compare them with the checked-in ones (`ops-check`) | Apache-2.0 | generator of the burn-rate rules (ADR 0035); the same version stands in the header of the generated files |
| `tofu` | `1.13.1` | `tofu init` and `tofu validate` per module under `infra/tofu/modules/` (`ops-check`) | MPL-2.0 | tool of the modules (ADR 0012); checks HCL, types and validations without access to a provider |
| `sonarqube` | `26.9.0.129388-community` | local SonarQube Community Build | LGPL-3.0 | ADR 0040 |
| `postgres` | `18.6-trixie` | database of the local SonarQube | PostgreSQL | required by the Community Build for continuous operation (H2 only for tests) |

Licenses: PyPI, npm and the LICENSE files of the projects, retrieved 2026-10-10 [verified].
Development tools are not shipped; the license rules for shipped
dependencies are in [licenses.md](licenses.md).

## GitHub Actions

All pinned by full commit SHA; the comment names the release tag. The SHAs were
looked up on 2026-10-10 with `git ls-remote` in the original repository (for annotated
tags, the commit SHA behind the tag).

| Action | Version | For what |
|---|---|---|
| `actions/checkout` | `v7.0.1` | checkout without stored credentials |
| `actions/setup-node` | `v7.1.0` | Node for the Node tools, with npm cache |
| `actions/setup-go` | `v7.0.0` | Go from `go.mod`, once it exists |
| `astral-sh/setup-uv` | `v10.3.0` | uv, with cache over `tools/uv.lock` |
| `actions/cache` | `v6.1.0` | binary cache and pre-commit environments |
| `actions/upload-artifact` | `v7.0.2` | release artifacts, Scorecard result |
| `actions/download-artifact` | `v8.0.2` | collecting release artifacts |
| `actions/attest` | `v4.2.2` | build provenance and SBOM attestation (public) |
| `anchore/sbom-action` | `v0.24.3` | SBOM (SPDX) of the release artifacts |
| `docker/setup-qemu-action` | `v4.4.0` | multi-arch build |
| `docker/setup-buildx-action` | `v4.4.1` | Buildx with provenance and SBOM; tag only after the scan (`imagetools create`) |
| `docker/login-action` | `v4.6.0` | login to ghcr.io |
| `docker/build-push-action` | `v7.4.0` | build and push the server image |
| `subosito/flutter-action` | `v2.23.0` | Flutter from `pubspec.yaml`, once the app exists |
| `github/codeql-action` | `v4.38.3` | CodeQL and SARIF upload (public) |
| `ossf/scorecard-action` | `v2.4.4` | OpenSSF Scorecard (public) |
| `actions/upload-pages-artifact` | `v5.0.0` | upload the docs site (public) |
| `actions/deploy-pages` | `v5.0.1` | publish the docs site (public) |

## Updates

* Renovate proposes updates grouped and with a waiting period (ADR 0041). An update is only
  done when this page names the new version; the test enforces this.
* Release binaries: after the Renovate proposal, `make tools-lock TOOL=<name>
  VERSION=<version>`; the command takes over the sha256 values that GitHub reports for the
  release files. Check the diff, then `make bootstrap` and `make check`.
* The validators of the helper services (`otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo`)
  follow the image versions in `infra/compose/`: Renovate raises the images (group
  "Compose-Images", ADR 0041) and is switched off for these five pins. In the pull request of the
  image update the maintainer refreshes them with `make tools-lock`; until then the check
  `tool-versions` of `make ops-check` fails, because a validator of another version accepts or
  rejects other keys than the running component.
* New tools come with a row on this page, a pin, a license check and a reason.
* A transitive Node package pinned through `overrides` in `tools/node/package.json` (for a
  security fix that the tool releases do not take yet) also gets a row here; the test
  enforces this. Remove the override once the tool asks for a fixed version itself.

## Trivy: keep the version

Trivy stays on `0.69.3`. Background: on 2026-03-19, a malicious Trivy version 0.69.4 was
published with stolen credentials, and tags of `trivy-action` and
`setup-trivy` were redirected; the advisory names 0.69.3 as not affected, protected
by immutable releases
(https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23,
retrieved 2026-10-10) [verified]. Renovate is switched off for Trivy. A move to a
newer version needs:

1. The release is marked as immutable (GitHub release API, field `immutable`).
2. The signature of the checksum file is verified with cosign against the workflow identity of the
   Trivy repository.
3. The advisory or a later one names no incident for this version.
4. A pull request with `make tools-lock TOOL=trivy VERSION=<new>` and the check record
   in the description.

Trivy is never included via `trivy-action` or `setup-trivy`, but as a pinned
binary via `tools/bin/fetch_tools.py`.

## Known limitations

* actionlint 1.7.12 does not yet know the new notation `uses: $/...` for workflows in the same
  repository; zizmor recommends it (audit `self-repository`). Until actionlint
  understands it, `./` stays, with a justified zizmor exception in `release.yml`.
* Without `GH_TOKEN`, zizmor checks only offline; online audits (for example fake commits
  behind a SHA) run in CI with the workflow token.
* There are no binary pins for Windows hosts; contributors use WSL.
