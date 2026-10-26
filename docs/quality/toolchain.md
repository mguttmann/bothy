# Toolchain

Every tool of the gates with version, purpose, place of running, license and reason. A test keeps
the versions on this page equal to the pins (`tools/docs/tests/test_toolchain_doc.py`):
`tools/uv.lock`, `tools/node/package-lock.json`, `tools/bin/tools.lock.json`, `Makefile`,
`tools/sonarqube/compose.yaml`, `.pre-commit-config.yaml`, the workflows, the
`tool` directives of the Go modules under `services/` and (code generators of the SDKs)
`sdk/python/uv.lock` and `sdk/typescript/package-lock.json`.

Place of running: "local" means via `make` on the developer's host, "CI" means in the
same make target in GitHub Actions. All gate tools run in both places.

## Gate tools

| Tool | Version | Purpose | License | Why this one |
|---|---|---|---|---|
| `uv` | system tool | Python environment and lock for all Python tools | MIT OR Apache-2.0 | one lock with hashes for all Python tools; downloads the Python version itself |
| `pyyaml` | `6.0.3` | reading YAML in the check scripts | MIT | standard for YAML in Python; `on:` is deliberately not read as a boolean |
| `ruff` | `0.17.0` | format and lint for Python | MIT | replaces several linters, fast |
| `mypy` | `2.4.0` | type checking (strict) | MIT | reference type checker for Python |
| `playwright` | `1.64.0` | headless browsers of `make console-e2e` (SEC-T05, M6-03); the browsers come from the Playwright image of the same version, pinned by digest in `infra/compose/test/compose.console-e2e.yaml` | Apache-2.0 | the browser automation with Chromium, Firefox and WebKit, CDP access for the virtual WebAuthn authenticator |
| `bandit` | `1.9.4` | security patterns in Python | Apache-2.0 | required by the requirement; adds security rules to ruff |
| `pytest` | `9.1.1` | tests of the tools | MIT | standard |
| `pytest-cov` | `7.1.0` | coverage in pytest | MIT | standard |
| `coverage` | `7.16.2` | coverage threshold | Apache-2.0 | basis of pytest-cov |
| `schemathesis` | `4.30.1` | property-based contract tests G7 against the server of the working tree (`make contract-test`, `tools/contract/`) | MIT | 06 "Contract tests and gate": the version probed there; derives cases and stateful sequences from openapi.yaml and takes a custom auth for the DPoP proofs |
| `cryptography` | `50.0.2` | ES256 signatures of the DPoP proofs of G7 (`tools/contract/dpop.py`) | Apache-2.0 OR BSD-3-Clause | the maintained OpenSSL binding of Python; the standard library has no ECDSA |
| `markdown-it-py` | `4.2.0` | CommonMark parser as the oracle in the tests of `tools/configdoc` (code spans and code blocks for rule D7) | MIT | CommonMark-compliant parser with GFM tables ([markdown-it-py](https://github.com/executablebooks/markdown-it-py), retrieved 2026-10-12); only in tests, the tool itself stays with the standard library and PyYAML |
| `pre-commit` | `4.6.2` | hooks for commit, commit-msg, pre-push | MIT | widespread, hooks as a file in the repository |
| `pre-commit-hooks` | `v6.0.0` | file hygiene hooks | MIT | maintained standard hooks, pinned by commit SHA |
| `check-jsonschema` | `0.38.2` | schema check of issue forms, Renovate and CodeRabbit configuration | Apache-2.0 | ships the schemas of the formats |
| `zizmor` | `1.30.1` | security analysis of the workflows | MIT | required by the planning basis; checks injection, permissions, pinning |
| `mkdocs` | `1.6.1` | docs site | BSD | ADR 0045 |
| `mkdocs-material` | `9.7.7` | theme of the docs site | MIT | ADR 0045 |
| `mkdocstrings` | `1.0.8` | API reference of the Python SDK from its docstrings (`docs/sdk/python.md`) | ISC | the MkDocs plugin for references from source code (23 "Documentation in code", M2-29) |
| `mkdocstrings-python` | `2.0.10` | Python handler of mkdocstrings: reads the SDK with griffe without importing it, google docstrings | ISC | the official Python handler of mkdocstrings |
| `semgrep` | `1.180.0` | project rules (`.semgrep/`) | LGPL-2.1 | pattern rules for project invariants without an own parser (R-QA-05) |
| `@mermaid-js/mermaid-cli` | `11.17.0` | rendering diagrams | MIT | reference renderer of Mermaid; major 11 as at planning time |
| `puppeteer` | `25.13.0` | browser control for mermaid-cli | Apache-2.0 | dependency of mermaid-cli; determines the browser version |
| `katex` | `0.18.10` | math typesetting inside Mermaid diagrams | MIT | dependency of mermaid-cli and mermaid, which still ask for 0.16; pinned through `overrides` in `tools/node/package.json` to a release with the fix for GHSA-238p-pmpm-9mq7 (https://github.com/advisories/GHSA-238p-pmpm-9mq7, ranges in https://registry.npmjs.org/@mermaid-js/mermaid-cli/11.17.0 and https://registry.npmjs.org/mermaid/11.17.2, retrieved 2026-10-11; D-DEPS-1) |
| `@redocly/cli` | `2.62.1` | lint of OpenAPI and AsyncAPI | MIT | lints both formats with its own rules |
| `@sonar/scan` | `5.0.1` | SonarScanner without a local Java installation | LGPL-3.0 | same scanner locally and in CI, no Docker for the scan |
| `actionlint` | `1.7.12` | syntax and expressions of the workflows | MIT | required by the planning basis |
| `gitleaks` | `8.30.1` | secrets in all commits | MIT | as a CLI, not as an action (license obligation of the action in organizations) |
| `golangci-lint` | `2.14.0` | Go linter bundle including gosec and depguard | GPL-3.0 | standard for Go; dependency rules of the modules (chapter 02) |
| `oapi-codegen` | `v2.8.0` | Go server interface and models from `docs/api/openapi.yaml` (`tool` in `services/server/go.mod`, runs in `make codegen-check`) | Apache-2.0 ([LICENSE](https://github.com/oapi-codegen/oapi-codegen/blob/v2.8.0/LICENSE), retrieved 2026-10-12) | ADR 0013; 06 "Code generation" |
| `openapi-python-client` | `0.29.1` | core of the Python SDK from `docs/api/openapi.yaml` (dev group of `sdk/python/pyproject.toml`, runs in `make api-clients`) | MIT | 06 "Code generation", ADR 0026 |
| `openapi-typescript` | `7.13.0` | types of the TypeScript SDK from `docs/api/openapi.yaml` (`devDependencies` of `sdk/typescript/package.json`, runs in `make api-clients`) | MIT | 06 "Code generation", ADR 0026 |
| `typedoc` | `0.28.17` | API reference of the TypeScript SDK from its TSDoc comments (`devDependencies` of `sdk/typescript/package.json`): `npm run docs` builds it in `make docs`, `npm run docs:check` fails the script `lint` on an undocumented export (`validation.notDocumented`) | Apache-2.0 | 23 "Documentation in code" (TypeScript row), docs/decisions.d/M2-30.md D-M2-30-12 |
| `openapitools/openapi-generator-cli` | `v7.26.0@sha256:a304ddf1e2e5f24f68fa3153568d6174cea4959d09aa8e3db6d526fd0782326d` (digest of the multi-architecture index) | generator `dart-dio` for `packages/api_client` from `docs/api/openapi.yaml` (`GENERATOR_IMAGE` in `packages/api_client/Makefile`, runs with `docker run --network none` in `make api-clients`) | Apache-2.0 | 06 "Code generation", ADR 0026; the image brings the Java runtime the repository does not pin otherwise (docs/DECISIONS.md D-M4-02-1) |
| `gotestsum` | `1.13.0` | runs the Go tests in CI with reruns of failed tests and the flake report (`tools/ci/flakes.py`; gates.md, section "Flakes") | Apache-2.0 | only reruns the failed tests and keeps a test that fails in every attempt failing |
| `go-arch-lint` | `1.19.0` | module boundaries of the Go modules from the module register | MIT | checked in 02 "go-arch-lint" on a skeleton of all modules; checks packages and third-party libraries |
| `sqlc` | `1.31.1` | typed Go code from SQL, drift check | MIT | ADR 0007 |
| `govulncheck` | `v1.8.0` | reachable Go vulnerabilities | BSD-3-Clause | checks call graphs instead of only versions |
| `go-licenses` | `v2.0.1` | licenses of linked Go packages | Apache-2.0 | required by the research ruling |
| `lychee` | `0.24.2` | links and anchors in Markdown | Apache-2.0 OR MIT | checks anchors in Markdown files offline |
| `osv-scanner` | `2.6.0` | vulnerabilities and licenses of all lock files | Apache-2.0 | one tool for Go, Dart, Python and npm |
| `trivy` | `0.69.3` | misconfiguration and image vulnerabilities | Apache-2.0 | image scan at release; version see below |
| `shellcheck` | `0.11.0` | static analysis of the shell scripts of the Compose installation (`make lint`; `install.sh` as POSIX `sh`) | GPL-3.0-only | the standard shell linter the brief M7-29 names; a build tool only, nothing of it is shipped (like golangci-lint) |
| `debian` (container image) | `trixie-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f` (digest of the multi-architecture index) | runs the tests of `install.sh` with `dash` and util-linux `script` (`INSTALL_TEST_IMAGE` in the `Makefile`, `make install-test`); base of the host image of the terminal conformance suite (`infra/compose/test/host/Dockerfile`, M2-48), whose APT sources point at the timestamp `20261015T000000Z` of `snapshot.debian.org` (`ARG SNAPSHOT`, with `Acquire::Check-Valid-Until` off because the `Release` files of a snapshot expire), so its package set is reproducible | DFSG (Debian) | the shell most Linux hosts run as `/bin/sh`; no second shell is installed on the build host |
| `nginxinc/nginx-unprivileged` (container image) | the tag `BOTHY_NGINX_VERSION` of the example edge of M2-47 (`infra/compose/.env.example`, [configuration.md](../operations/configuration.md) 9) | the test-only TLS front `tls-front` of the terminal conformance suite for the instance origin (`infra/compose/test/compose.terminal.yaml`, M2-48) | BSD-2-Clause (nginx) | the same image and version as the example edge, so the suite adds no image of its own; it runs unprivileged and read-only |
| `cosign` | `3.1.3` | signs and verifies the release images, binaries and `SHA256SUMS` (`release.yml`, `make release-trial`) | Apache-2.0 | the Sigstore signing tool of ADR 0036; with a key pair and a signing configuration without Rekor, Fulcio and timestamp service in the private phase (ADR 0062) |
| `syft` | `1.54.1` | SPDX and CycloneDX SBOMs of every release image per platform and every binary | Apache-2.0 | the SBOM generator behind `anchore/sbom-action`, as a binary so that the trial release and `release.yml` run the same version (18, section 6) |
| `nfpm` | `2.48.0` | builds the `.deb` and `.rpm` packages of the host service from `infra/host/linux/nfpm.yaml` (`make -C services/host release-build`, brief M7-23) | MIT | one static binary for both formats with owners and modes per file, no `dpkg-deb` or `rpmbuild` on the build host |
| `wix` (WiX Toolset) | `5.0.2` | builds the MSI of the host service from `infra/host/windows/bothy-host.wxs` (`tools/distribution/hostdist.py`, nupkg checked against its SHA-256 there; only on Windows) | MS-RL | the MSI compiler of the Windows Installer documentation, runs on the preinstalled .NET; version 5 is the last one without the maintenance fee agreement of later versions (no cost, D-W-125); a build tool, not part of the MSI |
| `ubuntu`, `fedora`, `almalinux` (container images) | `24.04@sha256:68ee22004a9109b44973313b02ff7b1de250f1de0f5ecb628d73f579aaff33ac`, `43@sha256:a651ddf48ea28a06ed4e1e6519f51c9f47e7a5a138722ade87369b8fbb7e5b42`, `10-minimal@sha256:7127d3a958a0ba8bb673d7b4cd378c11052404ce1897940c32f1dd944448f7ab` (digests of the multi-architecture indexes) | throwaway containers in which `make release-trial COMPONENT=host` installs and removes the `.deb` and `.rpm` of the host service, next to the `debian` image above (`PACKAGE_TEST_IMAGES` in `tools/ci/release_trial.py`) | Ubuntu, Fedora and AlmaLinux image terms (free distributions) | one more distribution per package format than the build host's own; never installed on the build host |
| `git-cliff` | `2.14.2` | selects and parses the commits of one component for `make release-prep` | MIT OR Apache-2.0 | ADR 0060: path filter and tag pattern per component, offline |
| `registry` | `3.1.2` | local registry of `make release-trial` on `127.0.0.1` (image pinned by digest in `tools/ci/release_trial.py`) | Apache-2.0 | the reference implementation of the OCI distribution API (CNCF Distribution); no account, removed after the run |
| `scancode-toolkit` | `32.5.0` | license and copyright scan of the tracked tree before a release (`make scancode`, image from `tools/scancode/Dockerfile`) | Apache-2.0 | named by [licenses.md](licenses.md) section 4; text-level detection that the lock-file license checks lack |
| `otelcol-contrib` | `0.162.0` | `otelcol-contrib validate` for the collector base, every overlay and pairs of two overlays (`ops-check`) | Apache-2.0 | the same distribution and version as the collector image (`BOTHY_OTELCOL_VERSION`); only it knows the contrib components of the overlays |
| `promtool` | `3.15.0` | `promtool check config`, `check rules` and `test rules` (`ops-check`); `check rules` on the dashboard queries (`signal-check`, M1-68) | Apache-2.0 | check tool from the Prometheus release of the image version (`BOTHY_PROMETHEUS_VERSION`) |
| `amtool` | `0.34.1` | `amtool check-config` and routing tests (`ops-check`) | Apache-2.0 | check tool from the Alertmanager release of the image version (`BOTHY_ALERTMANAGER_VERSION`) |
| `loki` | `3.7.8` | `loki -verify-config` (`ops-check`) | AGPL-3.0-only | only Loki itself knows its configuration keys; version of the image (`BOTHY_LOKI_VERSION`). It is only run for the check and not distributed; AGPL-3.0 applies to distribution and to network use of modified versions ([AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html), sections [0](https://www.gnu.org/licenses/agpl-3.0.html#section0) "convey" and [13](https://www.gnu.org/licenses/agpl-3.0.html#section13), retrieved 2026-10-12), and neither happens here |
| `tempo` | `3.1.0` | `tempo -config.verify=true` (`ops-check`) | AGPL-3.0-only | only Tempo itself knows its configuration keys; version of the image (`BOTHY_TEMPO_VERSION`). It is only run for the check and not distributed; AGPL-3.0 applies to distribution and to network use of modified versions ([AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html), sections [0](https://www.gnu.org/licenses/agpl-3.0.html#section0) "convey" and [13](https://www.gnu.org/licenses/agpl-3.0.html#section13), retrieved 2026-10-12), and neither happens here |
| `sloth` | `0.16.0` | regenerate the SLO rules and compare them with the checked-in ones (`ops-check`) | Apache-2.0 | generator of the burn-rate rules (ADR 0035); the same version stands in the header of the generated files |
| `tofu` | `1.13.1` | `tofu init` and `tofu validate` per module under `infra/tofu/modules/` (`ops-check`); `make tofu-check` (fmt, offline init and validate of the roots under `infra/tofu/test/`, `tofu test` of the cloud modules with mocked providers, brief M7-16), the plan of the generic-vm test and `make tofu-vm-test` (brief M7-15) | MPL-2.0 | tool of the modules (ADR 0012); checks HCL, types and validations without access to a provider |
| `sonarqube` | `26.9.0.129388-community` | local SonarQube Community Build | LGPL-3.0 | ADR 0040 |
| `postgres` | `18.6-trixie` | database of the local SonarQube | PostgreSQL | required by the Community Build for continuous operation (H2 only for tests) |

The API reference pages of the site (M7-19) need no tool of their own: the MkDocs hook
`tools/docs/mkdocs_api_reference.py` renders them with `pyyaml` and `mkdocs` from the table
above, and `tools/docs/external_assets_check.py` uses only the standard library. Redocly's
`build-docs` was compared and not taken (docs/DECISIONS.md, M7-19/1).

The terminal conformance suite (M2-48, `services/server/test/terminal`) pulls no server image: each
end-to-end test package builds `services/server/Dockerfile` from the worktree with
`docker buildx build --load` for the platform of the Docker host, with the pinned stages of that
Dockerfile, and tags the result `termsuite-server:<first 12 hex of the image ID>`. The host image
`termsuite-host:<first 12 hex of the SHA-256 of infra/compose/test/host/Dockerfile>` is built from
the `debian` image above when the tag is missing. Both tags stay as local build cache;
[infra/compose/test/README.md](../../infra/compose/test/README.md) says how to remove them. The
other images of the stack are those of the installation and of the smoke overlay of M1-73.

Licenses: PyPI, npm and the LICENSE files of the projects, retrieved 2026-10-10 [verified].
Development tools are not shipped; the license rules for shipped
dependencies are in [licenses.md](licenses.md).

## Language toolchains

Go and Flutter are not pinned gate tools; they come from the components themselves:

- Go from the `go` and `toolchain` lines of the modules under `services/`. All modules carry the same
  lines, because CI installs one Go (ADR 0005); `tools/ci/tests/test_ci_toolchains.py`
  (`test_go_directives_agree`) holds that.
- Flutter from `environment.flutter` of the root `pubspec.yaml` of the Dart workspace, one exact
  version that the root and every member repeat (`test_flutter_pins_agree`, docs/DECISIONS.md
  D-M4-01-1); CI reads it with `flutter-version-file: pubspec.yaml`. The pure Dart packages use
  `flutter test` only as their test runner.

## GitHub Actions

All pinned by full commit SHA; the comment names the release tag. The SHAs were
looked up on 2026-10-10 with `git ls-remote` in the original repository (for annotated
tags, the commit SHA behind the tag).

| Action | Version | For what |
|---|---|---|
| `actions/checkout` | `v7.0.1` | checkout without stored credentials |
| `actions/setup-node` | `v7.1.0` | Node for the Node tools, with npm cache |
| `actions/setup-go` | `v7.0.0` | one Go for all modules under `services/`, from the `go.mod` of the server (otherwise of the host service) |
| `astral-sh/setup-uv` | `v10.3.0` | uv, with cache over `tools/uv.lock` |
| `actions/cache` | `v6.1.0` | binary cache and pre-commit environments |
| `actions/upload-artifact` | `v7.0.2` | release artifacts, Scorecard result |
| `actions/download-artifact` | `v8.0.2` | collecting release artifacts |
| `actions/attest` | `v4.2.2` | build provenance and SBOM attestation (public) |
| `anchore/sbom-action` | `v0.24.3` | SBOM (SPDX) of the release artifacts |
| `docker/setup-buildx-action` | `v4.4.1` | Buildx with provenance and SBOM; tag only after the scan (`imagetools create`) |
| `docker/login-action` | `v4.6.0` | login to ghcr.io |
| `docker/build-push-action` | `v7.4.0` | build and push the server image (multi-arch by cross compilation in the Dockerfile, without QEMU) |
| `subosito/flutter-action` | `v2.23.0` | Flutter for every Dart package (jobs `gates`, `sonar`, `apps (macOS)`, `apps (iOS)`, `apps (Windows)` and the Pages build), from `environment.flutter` of the root `pubspec.yaml` |
| `actions/setup-java` | `v6.1.0` | Temurin 17 for the Gradle builds of the Android code (jobs `gates`, `sonar`, `apps (Android API 26)`, `apps (Android API 36)`, brief M5-01) |
| `android-actions/setup-android` | `v4.0.4` | Android SDK for the JVM tests of the Android core in `make test` (jobs `gates`, `sonar`) |
| `reactivecircus/android-emulator-runner` | `v2.38.0` | Android emulators with API 26 and 36 for the instrumented tests (jobs `apps (Android API 26)` and `apps (Android API 36)`) |
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
* oasdiff is not a gate tool. The breaking change check G6 (`make api-breaking`) is the own
  comparison `tools/api-checks/breaking.py` over PyYAML, because oasdiff 1.33.0 missed rows of the
  06 13 table on `docs/api/openapi.yaml` (docs/DECISIONS.md D-M7-11-1); nothing new is pinned.
