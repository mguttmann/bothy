# SPDX-License-Identifier: Apache-2.0
#
# Repository gates. `make check` runs every gate; CI runs the same three targets
# `gates`, `denylist` and `sonar` (docs/quality/gates.md). Gates of a component skip
# with a SKIP line while the component has no code; a missing tool is an error.
#
# Tool versions are pinned in tools/pyproject.toml + tools/uv.lock (Python tools),
# tools/node/package.json + package-lock.json (Node tools) and tools/bin/tools.lock.json
# (release binaries with sha256). `make bootstrap` installs all of them.

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
# GNU make 3.81 (macOS) ignores .SHELLFLAGS and runs `$(SHELL) -c <line>`, split at blanks,
# so the flags go into SHELL there. Only make 3.82 and later know .SHELLFLAGS and list the
# feature `oneshell`. Recipes still end each step with `|| exit 1` (docs/DECISIONS.md M1-01/8,
# M1-05/22).
ifeq ($(filter oneshell,$(.FEATURES)),)
SHELL := bash -euo pipefail
endif
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
MAKEFLAGS += --no-builtin-rules --no-print-directory

# ---- configuration (named values: docs/spec/23-quality.md) -------------------------
COMPONENTS ?= all
# Repository-wide areas whose gates run (`all`, or `tools`, or `none`); only the pre-push gate
# narrows them (tools/hooks/push_plan.py, docs/quality/push-discipline.md). CI never sets it.
AREAS ?= all
# The file hygiene hooks of `make lint` run on the files changed between these two refs instead
# of every file, when both are set (the pre-push gate sets them); CI never sets them.
LINT_FROM_REF ?=
LINT_TO_REF ?=
LINT_SCOPE = $(if $(and $(LINT_FROM_REF),$(LINT_TO_REF)),--from-ref $(LINT_FROM_REF) --to-ref $(LINT_TO_REF),--all-files)
COVERAGE_GO_MIN ?= 80
COVERAGE_DART_MIN ?= 80
# Time limit of `make perf` per Go module (named value quality.perf.timeout).
PERF_TIMEOUT ?= 60m
# Time limit of each Go test binary in `make test` (named value quality.test.go_timeout).
GO_TEST_TIMEOUT ?= 20m
# One shard I/N of the Go tests (CI job "test (server)"): `make test` then runs only the packages
# tools/ci/go_shards.py assigns to shard I and records them in GO_SHARD_RECORD; empty runs all.
GO_TEST_SHARD ?=
GO_SHARD_RECORD ?= .cache/go-shard/go-shard.txt
# Build tags of `make test` (G8 response validation); the shard split lists packages with them.
GO_TEST_TAGS := contract_validate
# Host system of the Dart performance tests: they run in profile mode on macOS only.
PERF_HOST ?= $(shell uname -s)

# Examples per operation of G7 and its time limit (named values quality.contract.max_examples,
# quality.contract.timeout).
CONTRACT_MAX_EXAMPLES ?= 100
CONTRACT_TIMEOUT ?= 180m
SONAR_HOST_URL ?= http://127.0.0.1:9000
SONAR_ORGANIZATION ?=
SONAR_PROJECT_KEY ?= bothy
# Whether the scan waits for the quality gate and fails with it; ci.yml sets false on main, whose
# analysis records the overall state and the new code baseline and must not block main.
SONAR_QUALITYGATE_WAIT ?= true
SONAR_STARTUP_TIMEOUT_S ?= 300
# renovate: datasource=go depName=github.com/google/go-licenses/v2
GO_LICENSES_VERSION := v2.0.1
# renovate: datasource=go depName=golang.org/x/vuln
GOVULNCHECK_VERSION := v1.8.0
# renovate: datasource=pypi depName=semgrep
SEMGREP_VERSION := 1.180.0
REVIEW_BASE ?= main
# Licenses allowed for dependencies of shipped components (`make licenses`).
LICENSE_ALLOWLIST := quality/licenses/allowed.txt
DENYLIST_FLAGS ?=
GITLEAKS_FLAGS ?=
GITLEAKS_LOG_OPTS ?= HEAD

UV := uv
UVRUN := $(UV) run --project tools --locked --quiet
NODE_BIN := tools/node/node_modules/.bin
# git exports GIT_DIR and friends to hooks (the pre-push hook runs `make check`); Flutter reads
# its own version with git inside the SDK, so it runs without them (docs/DECISIONS.md D-M2-38-22).
FLUTTER := env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE -u GIT_COMMON_DIR -u GIT_PREFIX flutter
FETCH := $(UVRUN) python tools/bin/fetch_tools.py
# go, a Docker-compatible runtime and the pins of the server gates; a gap fails (R-QA-02).
GO_PREREQS := $(UVRUN) python tools/ci/go_prereqs.py
GO_SHARDS := $(UVRUN) python tools/ci/go_shards.py
CODEGEN_CHECK := $(UVRUN) python tools/ci/codegen_check.py
MIGRATIONS_LINT := $(UVRUN) python tools/ci/migrations_lint.py
SEMGREP := $(UV)x --quiet --python 3.12 --from semgrep==$(SEMGREP_VERSION) semgrep
# Path of a pinned release binary, resolved (and fetched on first use) inside a recipe.
tool = "$$($(FETCH) path $(1))"
# A pinned Node tool must be installed (`make bootstrap`); without it the gate fails with a hint.
need_node = [ -x $(NODE_BIN)/$(1) ] || { echo "$(1): $(NODE_BIN)/$(1) not found; run \`make bootstrap\`" >&2; exit 2; }
skip = printf '  SKIP  %-14s %s\n' '$(1)' '$(2)'
want = $(if $(filter all $(1),$(COMPONENTS)),yes,)
area = $(if $(filter all $(1),$(AREAS)),yes,)
# Sets `allowed` to the allowlist joined by commas. An empty list would become `--licenses=`
# and `--allowed_licenses=`, which no longer limit the licenses, so it fails.
allowed_licenses = allowed=$$(awk '!/^\#/ && NF' $(LICENSE_ALLOWLIST) | paste -sd, -) || exit 1; \
  [ -n "$$allowed" ] || { echo "licenses: $(LICENSE_ALLOWLIST) lists no license" >&2; exit 1; }

# ---- component discovery (tracked files only, like CI) --------------------------------
GO_MODS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'services/*go.mod' 2>/dev/null))))
# Modules with code per platform (the host service runs on Linux, macOS and Windows): typecheck
# and lint check them for every GOOS, so code for another platform is checked on every build host
# (docs/DECISIONS.md D-M2-42-21).
GO_CROSS_MODULES := services/host
GO_CROSS_OS := linux darwin windows
# Members of the Dart workspace; the root pubspec.yaml is the workspace itself, not a member.
DART_PKGS := $(filter-out .,$(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'apps/*pubspec.yaml' 'packages/*pubspec.yaml' 2>/dev/null)))))
# Generated Dart files: out of the format check, the doc and style lint (root
# analysis_options.yaml) and coverage (tools/ci/lcov_filter.py); docs/DECISIONS.md D-M4-01-4.
# Gradle modules of the Dart members with JVM tests (brief M5-01): <member>/android/<module>
# with a tracked src/test/; each runs through the member's Gradle wrapper in <member>/android.
JVM_TEST_MODULES := $(sort $(shell git ls-files -- 'packages/*/android/*/src/test/*' 2>/dev/null | sed -E 's|^(packages/[^/]+/android/[^/]+)/src/test/.*|\1|'))
DART_GENERATED := \.(g|freezed|drift)\.dart$$|/lib/src/generated/|^(apps|packages)/[^/]+/build/
# Hand-written Dart files of the members, set in a recipe as `files`.
dart_files = files=$$(git ls-files -- $(foreach p,$(DART_PKGS),'$(p)/*.dart')) || exit 1; \
  files=$$(printf '%s\n' "$$files" | grep -Ev '$(DART_GENERATED)' || true)
PY_PROJECTS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'sdk/python/pyproject.toml' 'plugins/*pyproject.toml' 2>/dev/null))))
TS_PROJECTS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'sdk/typescript/package.json' 'plugins/*package.json' 2>/dev/null))))
# Lock files OSV-Scanner reads; named explicitly instead of a directory walk, so that
# only tracked files count and .gitignore rules of enclosing checkouts cannot hide them.
LOCK_NAMES := uv.lock package-lock.json pnpm-lock.yaml pubspec.lock go.mod
PRODUCT_SOURCES := $(shell git ls-files -- 'services/*.go' 'apps/*.dart' 'packages/*.dart' 'sdk/*.py' 'sdk/*.ts' 'plugins/*.py' 'plugins/*.ts' 2>/dev/null | head -n 1)
# Go modules with performance targets (docs/spec/02-architecture.md "Performance targets", R-AR-20);
# the host service has none (docs/spec/24-terminal.md), so `make perf` skips it
# (docs/DECISIONS.md D-M2-38-3). A listed module without perf/ still fails.
PERF_MODULES := services/server
PERF_MODS = $(filter $(PERF_MODULES),$(GO_MODS))
GO_ON := $(and $(GO_MODS),$(call want,server))
DART_ON := $(and $(DART_PKGS),$(call want,apps))
PY_ON := $(and $(PY_PROJECTS),$(or $(call want,sdk-python),$(call want,plugins)))
TS_ON := $(and $(TS_PROJECTS),$(or $(call want,sdk-typescript),$(call want,plugins)))
# The Python tools under tools/ (format, lint, types, tests, coverage).
TOOLS_ON := $(call area,tools)
tools_skip = $(call skip,$(1),AREAS=$(AREAS) leaves the Python tools out (pre-push gate))
# The tools tests that read source files of the components (the working name in every Go and
# Dart file, the patience of the Go test helper, the Dart sources of bothy_noise): they run
# also when AREAS leaves the Python tools out, because a component change alone can fail them.
TOOLS_SOURCE_TESTS := ci/tests/test_product_name_places.py docs/tests/test_config_values.py \
  ci/tests/test_bothy_noise_package.py

.PHONY: help bootstrap bootstrap-ci check gates fmt fmt-check lint typecheck test coverage \
        sonar sonar-scan sonar-up sonar-down sonar-config sonar-config-check sonar-main-branch sonar-visibility sonar-new-code semgrep trivy osv gitleaks \
        gitleaks-staged denylist denylist-history licenses docs adr-index config-reference config-doc-check app-config event-catalog contract-documents catalog catalog-check linkcheck \
        trivy-image portability linkcheck-external diagrams tokens api-lint api-checks api-breaking api-tables api-clients \
        runners-check release-pins commitlint \
        review labels-check labels-sync tools-lock release-build release-prep release-trial scancode clean \
        codegen-check arch register-check migrations-lint migrations-lock sql-visibility flag-check perf ops-check signal-check \
        image image-test postgres-image postgres-image-test apps-platform contract-test

help: ## Show the targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---- setup ----------------------------------------------------------------------------
bootstrap: bootstrap-ci ## Install pinned tools and the git hooks (once per clone)
	$(UVRUN) pre-commit install --install-hooks
	@if [ -n "$${BOTHY_DENYLIST_FILE:-}" ] || [ -s .denylist-path ]; then \
	  echo "  deny list: configured"; \
	else \
	  echo "  deny list: NOT configured; set BOTHY_DENYLIST_FILE or write its path to .denylist-path"; \
	  echo "             (maintainers only; contributors see CONTRIBUTING.md)"; \
	fi
	@docker info >/dev/null 2>&1 && echo "  docker: ok (needed for make sonar-up and the server gates)" || \
	  echo "  docker: not reachable; make sonar-up and the server gates (once a Go module exists) need a Docker-compatible runtime"

bootstrap-ci: ## Install pinned tools without git hooks (CI)
	@command -v $(UV) >/dev/null || { echo "uv is required: https://docs.astral.sh/uv/"; exit 1; }
	@node -e 'const [a,b]=process.versions.node.split(".").map(Number); process.exit(a>22||(a===22&&b>=13)?0:1)' \
	  || { echo "Node.js >= 22.13 is required"; exit 1; }
	$(UV) sync --project tools --locked --quiet
	npm ci --prefix tools/node --ignore-scripts --no-audit --no-fund --loglevel=error
	@# The browser for mermaid-cli, unless MMDC_PUPPETEER_CONFIG names its own executable.
	@if ! grep -qs executablePath "$${MMDC_PUPPETEER_CONFIG:-/dev/null}"; then \
	  $(NODE_BIN)/puppeteer browsers install chrome-headless-shell >/dev/null; fi
	$(FETCH) fetch
	@# SDK components (M2-23/8): their own locks, installed when tracked.
	@if git ls-files --error-unmatch sdk/typescript/package-lock.json >/dev/null 2>&1; then \
	  npm ci --prefix sdk/typescript --ignore-scripts --no-audit --no-fund --loglevel=error || exit 1; fi
	@if git ls-files --error-unmatch sdk/python/uv.lock >/dev/null 2>&1; then \
	  $(UV) sync --project sdk/python --locked --quiet || exit 1; fi
	@# The Grafana panel plugin (M7-25): its own lock, installed when tracked.
	@if git ls-files --error-unmatch plugins/grafana-bothy-live/package-lock.json >/dev/null 2>&1; then \
	  npm ci --prefix plugins/grafana-bothy-live --ignore-scripts --no-audit --no-fund --loglevel=error || exit 1; fi

# ---- aggregate ------------------------------------------------------------------------
check: gates denylist sonar ## Every gate, identical to CI (before every push)
	@echo "check: all gates passed"

gates: ## All gates except denylist and sonar (CI job "gates")
# Generated code is checked before lint and type checks, so that stale generator output is
# reported as drift and not as a confusing compile error. `perf` is not a gate (P-07).
gates: fmt-check codegen-check lint typecheck arch register-check migrations-lint sql-visibility flag-check test coverage contract-test commitlint \
       docs config-doc-check linkcheck diagrams catalog-check api-lint api-checks api-breaking ops-check signal-check \
       api-clients tokens runners-check labels-check release-pins gitleaks semgrep trivy osv licenses

# ---- code quality ---------------------------------------------------------------------
fmt: ## Format all code in place
	$(UVRUN) ruff format tools
	$(UVRUN) ruff check --fix tools
	@if [ -n "$(GO_ON)" ]; then for m in $(GO_MODS); do (cd $$m && gofmt -w .); done; fi
	@if [ -n "$(DART_ON)" ]; then $(dart_files); [ -z "$$files" ] || dart format $$files || exit 1; fi
	@if [ -n "$(PY_ON)" ]; then for p in $(PY_PROJECTS); do $(UV) run --project $$p --locked ruff format $$p; done; fi
	@if [ -n "$(TS_ON)" ]; then for p in $(TS_PROJECTS); do npm --prefix $$p run format; done; fi

fmt-check: ## Formatting is clean (no changes)
	@if [ -z "$(TOOLS_ON)" ]; then $(call tools_skip,tools-format); else $(UVRUN) ruff format --check tools; fi
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-fmt,no Go module selected under services/); \
	else for m in $(GO_MODS); do out=$$(cd $$m && gofmt -l .) || exit 1; [ -z "$$out" ] || { echo "$$out"; exit 1; }; done; fi
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-format,no Dart package selected under apps/ or packages/); \
	else $(dart_files); [ -z "$$files" ] || dart format --output=none --set-exit-if-changed $$files || exit 1; fi
	@if [ -z "$(PY_ON)" ]; then $(call skip,py-format,no Python project selected under sdk/ or plugins/); \
	else for p in $(PY_PROJECTS); do $(UV) run --project $$p --locked ruff format --check $$p; done; fi
	@if [ -z "$(TS_ON)" ]; then $(call skip,ts-format,no TypeScript project selected); \
	else for p in $(TS_PROJECTS); do npm --prefix $$p run format:check; done; fi

lint: ## Linters: file hygiene hooks, ruff, bandit, Dockerfile pins, actionlint, zizmor, component linters
	SKIP=ruff-format,ruff-check,gitleaks-staged $(UVRUN) pre-commit run $(LINT_SCOPE) --hook-stage pre-commit
	@if [ -z "$(TOOLS_ON)" ]; then $(call tools_skip,tools-lint); else $(UVRUN) ruff check tools; fi
	@# Every FROM of a tracked Dockerfile is pinned by digest (SEC-SC-4, docs/spec/18-release.md "Server images").
	$(UVRUN) python tools/ci/dockerfile_pins.py
	@if [ -z "$(TOOLS_ON)" ]; then $(call tools_skip,tools-bandit); else $(UVRUN) bandit -q -c tools/pyproject.toml -r tools/api-checks tools/arch-check tools/bin tools/catalog tools/ci tools/configdoc tools/contract tools/denylist tools/design-tokens tools/doccheck tools/docs tools/hooks tools/lint tools/observability tools/opscheck tools/signalcheck tools/sonarqube; fi
	$(UVRUN) check-jsonschema --builtin-schema vendor.github-issue-forms $(filter-out %/config.yml,$(wildcard .github/ISSUE_TEMPLATE/*.yml))
	$(UVRUN) check-jsonschema --builtin-schema vendor.github-issue-config .github/ISSUE_TEMPLATE/config.yml
	$(UVRUN) check-jsonschema --builtin-schema vendor.renovate renovate.json
	$(UVRUN) check-jsonschema --schemafile https://coderabbit.ai/integrations/schema.v2.json .coderabbit.yaml
	$(call tool,actionlint) -shellcheck= -pyflakes= .github/workflows/*.yml
	@if [ -n "$${GH_TOKEN:-}" ]; then $(UVRUN) zizmor --min-severity low .github/workflows; \
	else echo "  zizmor: offline (set GH_TOKEN for the online audits)"; $(UVRUN) zizmor --offline --min-severity low .github/workflows; fi
	@# Pins are looked up in the repository root: the lookup command uses paths relative to it.
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-lint,no Go module selected); \
	else gl=$(call tool,golangci-lint) || exit 1; \
	  for m in $(GO_MODS); do (cd $$m && go vet -tags contract_validate,contract_live,perf ./... && \
	    "$$gl" run --build-tags contract_validate,contract_live,perf ./...) || exit 1; done; \
	  for m in $(filter $(GO_CROSS_MODULES),$(GO_MODS)); do for os in $(GO_CROSS_OS); do \
	    (cd $$m && GOOS=$$os go vet ./... && GOOS=$$os "$$gl" run ./...) || exit 1; done; done; fi
	@# One analysis at the root covers every member of the workspace (docs/DECISIONS.md D-M4-01-5).
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-lint,no Dart package selected); \
	else $(FLUTTER) pub get --enforce-lockfile && $(FLUTTER) analyze --fatal-infos || exit 1; \
	  for p in $(DART_PKGS); do if grep -q '^ *custom_lint:' $$p/pubspec.yaml; then (cd $$p && dart run custom_lint) || exit 1; fi; done; fi
	@# bandit skips the project's virtual environment and generated code (docs/DECISIONS.md M2-23/7).
	@if [ -z "$(PY_ON)" ]; then $(call skip,py-lint,no Python project selected); \
	else for p in $(PY_PROJECTS); do $(UV) run --project $$p --locked ruff check $$p && \
	  $(UVRUN) bandit -q -r $$p -x "$$p/tests,$$p/.venv,*/generated/*" || exit 1; done; fi
	@if [ -z "$(TS_ON)" ]; then $(call skip,ts-lint,no TypeScript project selected); \
	else for p in $(TS_PROJECTS); do npm --prefix $$p run lint; done; fi

typecheck: ## Type checks: mypy (strict), go build, flutter analyze, tsc
	@if [ -z "$(TOOLS_ON)" ]; then $(call tools_skip,tools-types); else cd tools && uv run --locked --quiet mypy; fi
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-build,no Go module selected); \
	else for m in $(GO_MODS); do (cd $$m && go build ./...) || exit 1; done; \
	  for m in $(filter $(GO_CROSS_MODULES),$(GO_MODS)); do for os in $(GO_CROSS_OS); do \
	    (cd $$m && GOOS=$$os go vet ./...) || exit 1; done; done; fi
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-types,no Dart package selected); \
	else echo "  dart types: covered by flutter analyze in make lint"; fi
	@# Inside each project: mypy finds its configuration and targets only from there.
	@if [ -z "$(PY_ON)" ]; then $(call skip,py-types,no Python project selected); \
	else for p in $(PY_PROJECTS); do (cd $$p && $(UV) run --locked --quiet mypy) || exit 1; done; fi
	@if [ -z "$(TS_ON)" ]; then $(call skip,ts-types,no TypeScript project selected); \
	else for p in $(TS_PROJECTS); do npm --prefix $$p run typecheck; done; fi

test: ## All tests with coverage data
	@mkdir -p .cache/coverage
	@if [ -z "$(TOOLS_ON)" ]; then $(call skip,tools-test,AREAS=$(AREAS): only the tools tests that read component sources); \
	  cd tools && uv run --locked --quiet pytest -q $(TOOLS_SOURCE_TESTS); \
	else cd tools && uv run --locked --quiet pytest --cov --cov-report=xml:../.cache/coverage/tools.xml --cov-report=term; fi
	@# Prerequisites first: without a Docker runtime the integration tests fail clearly (P-09).
	@# The architecture tests of tools/archgen run the pinned go-arch-lint named by GO_ARCH_LINT.
	@# -coverpkg=./...: a statement counts as covered when any test of its module runs it, also a
	@# test in another package (a use case tested through its HTTP adapter); see PL/1.
	@# -timeout: each test binary gets GO_TEST_TIMEOUT instead of the default 10m, because the
	@# instrumentation of the whole module slows the larger packages down (PL/2).
	@# GO_TEST_SHARD: the same command for the packages of one shard; the CI job
	@# "test (server coverage)" merges the shards' profiles (tools/ci/go_shards.py).
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-test,no Go module selected); \
	else $(GO_PREREQS) test || exit 1; gal=$(call tool,go-arch-lint) || exit 1; \
	  if [ -n "$(GO_TEST_SHARD)" ]; then mkdir -p "$(dir $(GO_SHARD_RECORD))" && : > "$(GO_SHARD_RECORD)" || exit 1; fi; \
	  for m in $(GO_MODS); do pkgs=./...; \
	    if [ -n "$(GO_TEST_SHARD)" ]; then \
	      pkgs=$$($(GO_SHARDS) packages --shard "$(GO_TEST_SHARD)" --record "$(abspath $(GO_SHARD_RECORD))" --tags $(GO_TEST_TAGS) --module $$m $(GO_MODS)) || exit 1; \
	      [ -n "$$pkgs" ] || { echo "  go-test: no package of $$m in shard $(GO_TEST_SHARD)"; continue; }; fi; \
	    (cd $$m && GO_ARCH_LINT="$$gal" go test -race -tags $(GO_TEST_TAGS) -covermode=atomic -coverpkg=./... -coverprofile=coverage.out -timeout=$(GO_TEST_TIMEOUT) $$pkgs) || exit 1; done; fi
	@# --enforce-lockfile: a pubspec.lock that no longer matches pubspec.yaml fails instead of being
	@# rewritten silently, so osv and licenses always read the lock file the tests ran with.
	@# The workspace resolves once at the root (one pubspec.lock); then every member runs its tests.
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-test,no Dart package selected); \
	else for p in $(DART_PKGS); do \
	  [ -d $$p/test ] || { echo "$$p: no test/ directory (docs/spec/23-quality.md, 3)" >&2; exit 1; }; done; \
	  $(FLUTTER) pub get --enforce-lockfile || exit 1; \
	  for p in $(DART_PKGS); do (cd $$p && $(FLUTTER) test --coverage) || exit 1; done; fi
	@# JVM tests of the Android code (brief M5-01): without a JDK 17 or the Android SDK
	@# (ANDROID_HOME) the Gradle wrapper fails, and so does this gate (P-09).
	@if [ -z "$(DART_ON)" ]; then $(call skip,jvm-test,no Dart package selected); \
	elif [ -z "$(JVM_TEST_MODULES)" ]; then $(call skip,jvm-test,no Gradle module with src/test under packages/*/android); \
	else for m in $(JVM_TEST_MODULES); do \
	  (cd $${m%/*} && ./gradlew --no-daemon --quiet --console=plain -p $${m##*/} testDebugUnitTest) || exit 1; done; fi
	@if [ -z "$(PY_ON)" ]; then $(call skip,py-test,no Python project selected); \
	else for p in $(PY_PROJECTS); do (cd $$p && $(UV) run --locked pytest --cov --cov-report=xml:coverage.xml); done; fi
	@if [ -z "$(TS_ON)" ]; then $(call skip,ts-test,no TypeScript project selected); \
	else for p in $(TS_PROJECTS); do npm --prefix $$p test; done; fi

# G7 (06 "Contract tests and gate"): the Go test starts the server of this tree with a throwaway
# PostgreSQL and runs tools/contract/live_contract.py (Schemathesis) against it; G8 validates its
# responses on the server side (tag contract_validate).
contract-test: ## Contract tests G7: Schemathesis against the server of this tree (Docker runtime)
	@if [ -z "$(GO_ON)" ]; then $(call skip,contract-test,no Go module selected); \
	else $(GO_PREREQS) test || exit 1; \
	  cd services/server && CONTRACT_MAX_EXAMPLES=$(CONTRACT_MAX_EXAMPLES) go test -count=1 -tags contract_validate,contract_live \
	    -run '^TestG7_' -timeout $(CONTRACT_TIMEOUT) ./test/contract/... || exit 1; fi

coverage: test ## Coverage thresholds (named values quality.coverage.*)
	@if [ -z "$(TOOLS_ON)" ]; then $(call tools_skip,tools-coverage); \
	else cd tools && uv run --locked --quiet coverage report >/dev/null && echo "  tools coverage: ok (fail_under in tools/pyproject.toml)"; fi
	@# Production code only: packages under <module>/test/ and <module>/perf/ are test code
	@# (docs/spec/02-architecture.md "Code layout"), and generated files (*.gen.go, docs/DECISIONS.md
	@# M1-05/7 and M1-17) are not hand-written code. The total comes from a filtered copy;
	@# coverage.out stays as go test wrote it, so SonarQube reads the same file in
	@# `make check` and in the CI job sonar, which scans the reports of the job gates.
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-coverage,no Go module selected); \
	else for m in $(GO_MODS); do \
	  mod=$$(awk '$$1 == "module" { gsub(/"/, "", $$2); print $$2; exit }' $$m/go.mod); \
	  [ -n "$$mod" ] || { echo "coverage: no module path in $$m/go.mod" >&2; exit 1; }; \
	  f=$$(mktemp "$${TMPDIR:-/tmp}/bothy-coverage.XXXXXX") || exit 1; \
	  awk -v p="$$mod/" 'NR == 1 || (index($$0, p "test/") != 1 && index($$0, p "perf/") != 1 && index($$0, ".gen.go:") == 0)' \
	    $$m/coverage.out > "$$f" || { rm -f "$$f"; exit 1; }; \
	  out=$$(cd $$m && go tool cover -func="$$f") || { rm -f "$$f"; exit 1; }; rm -f "$$f"; \
	  total=$$(printf '%s\n' "$$out" | awk '/^total:/ {sub("%","",$$3); print $$3}'); \
	  [ -n "$$total" ] || { echo "coverage: go tool cover printed no total for $$m" >&2; exit 1; }; \
	  awk -v t="$$total" -v min=$(COVERAGE_GO_MIN) -v m=$$m 'BEGIN { if (t+0 < min) { printf "%s: coverage %s%% < %s%%\n", m, t, min; exit 1 } }' || exit 1; done; fi
	@# Generated files leave the sum first (tools/ci/lcov_filter.py, docs/DECISIONS.md D-M4-01-4).
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-coverage,no Dart package selected); \
	else for p in $(DART_PKGS); do \
	  [ -f $$p/coverage/lcov.info ] || { echo "$$p: coverage/lcov.info is missing; make test writes it" >&2; exit 1; }; \
	  sums=$$($(UVRUN) python tools/ci/lcov_filter.py --totals $$p/coverage/lcov.info) || exit 1; \
	  [[ "$$sums" =~ ^[0-9]+\ [0-9]+$$ ]] || { echo "coverage: tools/ci/lcov_filter.py printed no totals for $$p" >&2; exit 1; }; \
	  awk -v s="$$sums" -v min=$(COVERAGE_DART_MIN) -v p=$$p 'BEGIN { split(s, a, " "); lf = a[1]; lh = a[2]; pct = lf ? 100*lh/lf : 100; if (pct < min) { printf "%s: coverage %.1f%% < %s%%\n", p, pct, min; exit 1 } }' || exit 1; done; fi
	@[ -z "$(PY_ON)$(TS_ON)" ] || echo "  python/typescript components enforce their threshold in their own test config"

commitlint: ## Conventional Commits and DCO sign-off for commits not yet on the remote main
	@base=""; \
	if git rev-parse --abbrev-ref '@{upstream}' >/dev/null 2>&1; then base='@{upstream}'; \
	elif git rev-parse --verify -q origin/main >/dev/null; then base=origin/main; fi; \
	range="$${COMMIT_RANGE:-$${base:+$$base..HEAD}}"; \
	if [ -z "$$range" ]; then $(call skip,commitlint,no remote main yet; the commit-msg hook checks each commit); \
	else $(UVRUN) python tools/hooks/commit_msg.py --range "$$range" --require-signoff \
	  $${PR_AUTHOR:+--pr-author "$$PR_AUTHOR"}; fi
	@# CI sets PR_AUTHOR to the pull request author: a dependency bot's own commits need no
	@# sign-off in its own pull request (D-DEPS-3).
	@# The pull request title becomes the squash commit on main (CI sets PR_TITLE).
	@if [ -n "$${PR_TITLE:-}" ]; then f=$$(mktemp); printf '%s\n' "$$PR_TITLE" > "$$f"; \
	  rc=0; $(UVRUN) python tools/hooks/commit_msg.py "$$f" || rc=$$?; rm -f "$$f"; exit $$rc; fi

# ---- server gates ---------------------------------------------------------------------
# Each server gate checks its prerequisites first (go, a Docker-compatible runtime, the pins
# of go-arch-lint and sqlc; tools/ci/go_prereqs.py). Every step ends with `|| exit 1`, so a
# recipe stops on its own even in a shell without errexit (docs/DECISIONS.md M1-01/8); the
# flags in SHELL (top of this file, also for GNU make 3.81) are the second line of defence.
codegen-check: ## Generated code equals a fresh generation (sqlc, go generate, flutter gen-l10n, Pigeon, drift, Riverpod, the domain tables, the contract copy of the fake)
	@if [ -z "$(GO_ON)" ]; then $(call skip,codegen-check,no Go module selected under services/); \
	else $(GO_PREREQS) codegen-check || exit 1; \
	  gal=$(call tool,go-arch-lint) || exit 1; sqlc=$(call tool,sqlc) || exit 1; \
	  for m in $(GO_MODS); do \
	    if [ -f $$m/sqlc.yaml ]; then $(CODEGEN_CHECK) --dir $$m -- "$$sqlc" generate -f sqlc.yaml || exit 1; fi; \
	    PATH="$$(dirname "$$gal"):$$(dirname "$$sqlc"):$$PATH" $(CODEGEN_CHECK) --dir $$m -- go generate ./... || exit 1; \
	  done; fi
	@# Localizations: each Dart member with an l10n.yaml runs `flutter gen-l10n` in a temporary
	@# copy; an untranslated message or a stale lib/src/generated/ fails (docs/DECISIONS.md D-M4-06-2).
	@if [ -z "$(DART_ON)" ]; then $(call skip,codegen-dart,no Dart package selected); \
	else l10n=""; for p in $(DART_PKGS); do [ ! -f $$p/l10n.yaml ] || l10n="$$l10n $$p"; done; \
	  if [ -z "$$l10n" ]; then $(call skip,codegen-dart,no Dart member with l10n.yaml); \
	  else for p in $$l10n; do $(UVRUN) python tools/ci/l10n_check.py --dir $$p || exit 1; done; fi; fi
	@# Pigeon channels of the Dart members (briefs M4-10, M4-11): every tracked pigeons/*.dart is generated
	@# again and the Dart output formatted, as the generation step in the member's README does.
	@# The comparison covers every output under the member (Dart, Swift and the C++ of windows/).
	@if [ -z "$(DART_ON)" ]; then $(call skip,codegen-pigeon-dart,no Dart package selected under apps/ or packages/); \
	else inputs=$$(git ls-files -- $(foreach p,$(DART_PKGS),'$(p)/pigeons/*.dart')) || exit 1; \
	  if [ -z "$$inputs" ]; then $(call skip,codegen-pigeon-dart,no Pigeon input under pigeons/); \
	  else $(FLUTTER) pub get --enforce-lockfile >/dev/null || exit 1; \
	    for f in $$inputs; do p=$${f%/pigeons/*}; \
	      $(CODEGEN_CHECK) --dir $$p -- sh -c 'dart run pigeon --input "$$1" && dart format lib/src/generated >/dev/null' pigeon "$${f#$$p/}" || exit 1; \
	    done; fi; fi
	@# drift of the Dart members with a drift build.yaml (brief M4-12): build_runner writes the
	@# database code and `drift_dev make-migrations` the stored schema file and the schema helpers
	@# of the migration tests; both must equal a fresh generation, so a changed table without a
	@# new schema version fails (docs/DECISIONS.md D-M4-12-7).
	@if [ -z "$(DART_ON)" ]; then $(call skip,codegen-drift-dart,no Dart package selected under apps/ or packages/); \
	else drift=""; for p in $(DART_PKGS); do ! grep -qs 'drift_dev:' $$p/build.yaml || drift="$$drift $$p"; done; \
	  if [ -z "$$drift" ]; then $(call skip,codegen-drift-dart,no Dart member with a drift build.yaml); \
	  else flutter pub get --enforce-lockfile >/dev/null || exit 1; \
	    for p in $$drift; do \
	      $(CODEGEN_CHECK) --dir $$p -- sh -c 'dart run build_runner build && dart run drift_dev make-migrations' || exit 1; \
	    done; fi; fi
	@# Riverpod providers of the Dart members with riverpod_generator (brief M4-18): build_runner
	@# writes the *.g.dart files of the providers, formatted as `make fmt` formats them; they must
	@# equal a fresh generation, so a changed provider without regeneration fails.
	@if [ -z "$(DART_ON)" ]; then $(call skip,codegen-riverpod-dart,no Dart package selected under apps/ or packages/); \
	else rp=""; for p in $(DART_PKGS); do ! grep -qs 'riverpod_generator:' $$p/pubspec.yaml || rp="$$rp $$p"; done; \
	  if [ -z "$$rp" ]; then $(call skip,codegen-riverpod-dart,no Dart member with riverpod_generator); \
	  else flutter pub get --enforce-lockfile >/dev/null || exit 1; \
	    for p in $$rp; do \
	      $(CODEGEN_CHECK) --dir $$p -- sh -c 'dart run build_runner build && dart format lib >/dev/null' || exit 1; \
	    done; fi; fi
	@# The tables of packages/domain: AppConfig from the configuration reference and the event
	@# catalogue from actions.yaml (docs/DECISIONS.md D-M4-08-3, D-M4-08-5). Python only, so
	@# they run for every component selection; a reference row added without regeneration fails.
	@if [ ! -f packages/domain/pubspec.yaml ]; then $(call skip,codegen-domain,packages/domain is not tracked); \
	else $(UVRUN) python tools/configdoc/app_config.py --check || exit 1; \
	  $(UVRUN) python tools/catalog/event_catalog_dart.py --check || exit 1; fi
	@# The copy of docs/api/ in the fake server of sync_engine, which the app's integration tests
	@# read inside the macOS app sandbox (docs/DECISIONS.md D-M4-18-17); Python only, like above.
	@if [ ! -f packages/sync_engine/pubspec.yaml ]; then $(call skip,codegen-contract,packages/sync_engine is not tracked); \
	else $(UVRUN) python tools/catalog/contract_documents_dart.py --check || exit 1; fi

arch: ## Module boundaries: Go modules (go-arch-lint, from the module register) and the Dart workspace (tools/arch-check)
	@# go-arch-lint does not follow a linked directory but attributes it by the path of the link,
	@# while the compiler builds the code behind it; so no symbolic link may sit below a module
	@# (docs/DECISIONS.md M1-05/24). Only editor lock files pass: dangling links named .#*, behind
	@# which nothing can be built. go-arch-lint stops at any dangling link below a ** component,
	@# so with lock files it checks a copy without them and names the files of the module (M1-05/27).
	@if [ -z "$(GO_ON)" ]; then $(call skip,arch,no Go module selected under services/); \
	else $(GO_PREREQS) arch || exit 1; gal=$(call tool,go-arch-lint) || exit 1; \
	  for m in $(GO_MODS); do \
	    [ -f $$m/.go-arch-lint.yml ] || { echo "arch: $$m/.go-arch-lint.yml is missing (generated from $$m/architecture/modules.yaml, docs/spec/02-architecture.md \"Automatic check of the rules\")" >&2; exit 1; }; \
	    links=$$(cd $$m && find . -type l \( ! -name '.#*' -o -exec test -e {} \; \) -print) || exit 1; \
	    [ -z "$$links" ] || { printf 'arch: %s has symbolic links; go-arch-lint does not follow them, the Go compiler does (docs/quality/gates.md):\n%s\n' "$$m" "$$links" >&2; exit 1; }; \
	    locks=$$(cd $$m && find . -type l -name '.#*' -print) || exit 1; \
	    if [ -z "$$locks" ]; then \
	      (cd $$m && "$$gal" check --project-path "$$(pwd -P)" --arch-file .go-arch-lint.yml --output-color=false) || exit 1; \
	    else \
	      printf 'arch: %s: checking a copy without the editor lock files:\n%s\n' "$$m" "$$locks" >&2; \
	      dir=$$(cd $$m && pwd -P) || exit 1; copy=$$(mktemp -d) || exit 1; \
	      { tar -cf "$$copy/m.tar" --exclude='.#*' -C "$$dir" . && mkdir "$$copy/m" && tar -xf "$$copy/m.tar" -C "$$copy/m" \
	        && view=$$(cd "$$copy/m" && pwd -P); } || { rm -rf "$$copy"; exit 1; }; \
	      out=$$(cd "$$view" && "$$gal" check --project-path "$$view" --arch-file .go-arch-lint.yml --output-color=false 2>&1) && rc=0 || rc=$$?; \
	      rm -rf "$$copy"; printf '%s\n' "$${out//"$$view"/$$dir}"; [ "$$rc" -eq 0 ] || exit 1; \
	    fi; \
	  done; fi
	@# The dependency rules of docs/spec/15-client-architecture.md 3.1 for every member of the Dart
	@# workspace, from tools/arch-check/rules.yaml (docs/DECISIONS.md D-M4-01-3).
	@if [ -z "$(DART_ON)" ]; then $(call skip,arch-dart,no Dart package selected under apps/ or packages/); \
	else $(UVRUN) python tools/arch-check/arch_check.py || exit 1; fi

# Repository-wide like catalog-check: no SKIP, a missing source fails (exit 2).
register-check: ## Module register equals 03, actions.yaml, the tables of 02, the component diagram and the module READMEs (ADR 0050)
	$(UVRUN) python tools/doccheck/register_check.py

migrations-lint: ## Migration rules: sequential numbers, locked checksums, STORED, NO TRANSACTION (ADR 0007)
	@if [ -z "$(GO_ON)" ]; then $(call skip,migrations-lint,no Go module selected under services/); \
	else $(GO_PREREQS) migrations-lint || exit 1; \
	  for m in $(GO_MODS); do \
	    if [ ! -d $$m/migrations ] && [ ! -e $$m/migrations.sha256 ]; then \
	      printf '  SKIP  %-14s %s\n' migrations-lint "$$m has no migrations/ directory yet"; \
	    else $(MIGRATIONS_LINT) check --dir $$m/migrations --sum $$m/migrations.sha256 || exit 1; fi; \
	  done; fi

# AZ-P12 (07 "Query pattern" rule 1): every reading sqlc query on a content table carries the
# visibility predicate or stands on the reviewed exception list tools/lint/sql_visibility.yaml.
sql-visibility: ## SQL visibility predicate on content tables (AZ-P12)
	@if [ -z "$(GO_ON)" ]; then $(call skip,sql-visibility,no Go module selected under services/); \
	else $(UVRUN) python tools/lint/sql_visibility.py || exit 1; fi

# Repository-wide like register-check: the server module decides, not COMPONENTS (gates.md
# "Skipping and failing"). ADR 0064: no registered release flag outlives its remove_by.
flag-check: ## Feature flag registry valid, no release flag past remove_by, configuration.md 7 equal (ADR 0064)
	@if ! git ls-files --error-unmatch services/server/go.mod >/dev/null 2>&1; then $(call skip,flag-check,no server module); \
	else (cd services/server && go run ./tools/flagcheck -version-file VERSION \
	  -config-doc ../../docs/operations/configuration.md -modules architecture/modules.yaml) || exit 1; fi

migrations-lock: ## Record the checksums of new migrations in <module>/migrations.sha256 (not a gate)
	@found=""; for m in $(GO_MODS); do \
	  if [ -d $$m/migrations ]; then found=1; \
	    $(MIGRATIONS_LINT) lock --dir $$m/migrations --sum $$m/migrations.sha256 || exit 1; fi; \
	done; [ -n "$$found" ] || echo "  migrations-lock: no Go module with a migrations/ directory"

perf: ## Load and chaos tests against the performance targets (end of a milestone and releases, not a gate; P-07)
	@if [ -z "$(GO_ON)" ]; then $(call skip,perf,no Go module selected under services/); \
	elif [ -z "$(PERF_MODS)" ]; then $(call skip,perf,no Go module with performance targets selected); \
	else $(GO_PREREQS) perf || exit 1; \
	  for m in $(PERF_MODS); do \
	    [ -d $$m/perf ] || { echo "perf: $$m/perf does not exist; load tests live there (docs/spec/02-architecture.md \"Performance targets\")" >&2; exit 1; }; \
	    echo "  perf: targets hold only on perf.reference_profile (mode match or emulated, ADR 0063)"; \
	    rm -f .cache/perf/results.json; \
	    rc=0; (cd $$m && go test -tags perf -count=1 -p 1 -timeout $(PERF_TIMEOUT) ./perf/... ./test/chaos/...) || rc=1; \
	    (cd $$m && go run ./perf/internal/report/perfreport) || rc=1; \
	    [ $$rc -eq 0 ] || exit 1; \
	  done; fi
	@# Dart members run their performance test in profile mode on macOS (`tool/perf_macos.sh`
	@# of the member, for example the live topology of bothy_viz, brief M6-12 design note 6);
	@# other hosts print a SKIP line, the CI job apps-macos is the evidence there. `make perf` is
	@# the run on reference hardware, so it holds the reference budget (D-M6-12-10).
	@if [ -z "$(DART_ON)" ]; then $(call skip,perf-dart,no Dart package selected); \
	else found=""; for p in $(DART_PKGS); do \
	    if [ -f $$p/tool/perf_macos.sh ]; then found="$$found $$p"; fi; done; \
	  if [ -z "$$found" ]; then $(call skip,perf-dart,no Dart member with tool/perf_macos.sh); \
	  elif [ "$(PERF_HOST)" != Darwin ]; then $(call skip,perf-dart,needs macOS (CI job apps-macos)); \
	  else for p in $$found; do BOTHY_PERF_BUDGET=reference sh $$p/tool/perf_macos.sh || exit 1; done; fi; fi

# ---- documentation --------------------------------------------------------------------
docs: ## Documentation site builds strictly with the API references and the Dart API pages; ADR index is current; problem codes registered; brief dependencies consistent; register IDs unique; risk ranking and the list behind its last rank follow the register ratings; no German text in tracked files (D-W-94); no external assets
	$(UVRUN) python tools/docs/language_check.py
	$(UVRUN) python tools/docs/adr_index.py --check
	git ls-files -z -- '*.md' docs/api/openapi.yaml docs/api/asyncapi.yaml | xargs -0 $(UVRUN) python tools/docs/problem_code_refs.py
	$(UVRUN) python tools/docs/brief_deps.py
	$(UVRUN) python tools/docs/register_ids.py
	$(UVRUN) python tools/docs/risk_ranking.py
	@# docs/reference/dart-api.md links the generated Dart API pages, so they are built for every
	@# component selection, not only for `apps` (docs/DECISIONS.md D-M4-01-6).
	@if [ -z "$(DART_PKGS)" ]; then $(call skip,dart-api-docs,no Dart package tracked under apps/ or packages/); \
	else $(UVRUN) python tools/docs/dart_api_docs.py --out docs/api/dart || exit 1; fi
	@# No --quiet: it hides the warnings that --strict turns into a failure.
	$(UVRUN) mkdocs build --strict --site-dir site
	@# The site bundles every script, stylesheet and font; nothing loads from another origin.
	$(UVRUN) python tools/docs/external_assets_check.py site

adr-index: ## Regenerate the ADR index in docs/adr/README.md
	$(UVRUN) python tools/docs/adr_index.py

config-reference: ## Regenerate section 4 of docs/operations/configuration.md from the schema and the chapter tables
	$(UVRUN) python tools/configdoc/configdoc.py

config-doc-check: ## Configuration reference matches schema, chapter tables, planned.yaml, retired.yaml and Compose (D1 to D10)
	@# Reads the committed services/server/architecture/config-schema.json, so no Go toolchain is
	@# needed; make codegen-check keeps that file equal to the schema in the code.
	$(UVRUN) python tools/configdoc/configdoc.py --check

catalog: ## Regenerate the tables of chapter 05 and the lists in docs/api/redocly.yaml from docs/spec/actions.yaml
	$(UVRUN) python tools/catalog/action_catalog.py generate

catalog-check: ## Action catalogue valid and its generated copies current (ADR 0049)
	$(UVRUN) python tools/catalog/action_catalog.py check

app-config: ## Regenerate the AppConfig key table of packages/domain from the configuration reference
	$(UVRUN) python tools/configdoc/app_config.py

event-catalog: ## Regenerate the event catalogue of packages/domain from docs/spec/actions.yaml
	$(UVRUN) python tools/catalog/event_catalog_dart.py

contract-documents: ## Regenerate the copy of docs/api/ in the fake server of packages/sync_engine
	$(UVRUN) python tools/catalog/contract_documents_dart.py

linkcheck: ## Local links and anchors of every tracked Markdown file (offline); no link to the placeholder repository
	@# docs/reference/dart-api.md links the generated Dart API pages; `make docs` builds them, and
	@# a run without them builds them first (docs/DECISIONS.md D-M4-01-6).
	@if [ -n "$(DART_PKGS)" ] && [ ! -d docs/api/dart ]; then $(UVRUN) python tools/docs/dart_api_docs.py --out docs/api/dart || exit 1; fi
	git ls-files -z -- '*.md' | xargs -0 $(call tool,lychee) --offline --include-fragments --no-progress --quiet
	git ls-files -z -- '*.md' | xargs -0 $(UVRUN) python tools/docs/placeholder_links.py

linkcheck-external: ## External links too (network, not a gate)
	git ls-files -z -- '*.md' | xargs -0 $(call tool,lychee) --include-fragments --no-progress --max-concurrency 8

diagrams: ## Every Mermaid diagram renders; .svg themed and readable; ERDs match the data model
	$(UVRUN) python tools/docs/render_diagrams.py

tokens: ## Design-token build check (contract: tools/design-tokens/Makefile target check)
	@if [ ! -d tools/design-tokens ]; then $(call skip,tokens,tools/design-tokens does not exist yet); \
	elif [ -f tools/design-tokens/Makefile ]; then $(MAKE) -C tools/design-tokens check; \
	else echo "tokens: tools/design-tokens exists but has no Makefile with target 'check'"; exit 1; fi

api-lint: ## Redocly lint of the OpenAPI and AsyncAPI documents
	@if [ ! -f docs/api/openapi.yaml ]; then $(call skip,api-lint,docs/api/openapi.yaml does not exist yet); \
	else cfg=""; [ -f docs/api/redocly.yaml ] && cfg="--config docs/api/redocly.yaml"; \
	  files="docs/api/openapi.yaml"; [ -f docs/api/asyncapi.yaml ] && files="$$files docs/api/asyncapi.yaml"; \
	  $(call need_node,redocly); REDOCLY_TELEMETRY=off $(NODE_BIN)/redocly lint $$cfg $$files; fi

api-checks: ## API contract copies equal their sources (06 G2, G3)
	$(UVRUN) python tools/api-checks/api_checks.py check

api-tables: ## Regenerate the generated tables of chapter 06 and webhooks.md
	$(UVRUN) python tools/api-checks/api_checks.py generate

api-breaking: ## G6: no breaking change of the API against the last server release (06 13)
	@# The API contract ships with the server (RELEASE_EXTRA_PATHS of tools/ci/components.py),
	@# and a change under docs/api/ selects every gate component, the server included.
	@if [ -z "$(call want,server)" ]; then $(call skip,api-breaking,component server not selected); \
	elif [ ! -f docs/api/openapi.yaml ]; then $(call skip,api-breaking,docs/api/openapi.yaml does not exist yet); \
	else $(UVRUN) python tools/api-checks/breaking.py check; fi

signal-check: ## Dashboards, rules and SLOs use only catalogue metrics and labels; SLO thresholds are bucket bounds (M1-68)
	$(UVRUN) python tools/signalcheck/signalcheck.py

ops-check: ## Operations configuration valid with the pinned upstream tools; derived values equal their sources
	@# Collector, Prometheus, Alertmanager, Loki, Tempo, Sloth and OpenTofu files exist, so there
	@# is no SKIP; a validator that cannot be fetched fails (tools/opscheck/README.md).
	$(UVRUN) python tools/opscheck/opscheck.py

# Client directories of gate G4 with their gate component (component contract "Generated
# client", docs/spec/23-quality.md section 3); a test replaces the list.
API_CLIENTS ?= sdk/python:sdk-python sdk/typescript:sdk-typescript packages/api_client:apps

api-clients: ## G4: generated API clients equal a fresh generation and compile (each client: `make -C <dir> generate-check`)
	@# Fails closed: a client with a tracked manifest but without the target is an error
	@# (docs/quality/gates.md "Skipping and failing", D-QA-22).
	@# git exports GIT_DIR to hooks (the pre-push hook runs `make check`); without the variables
	@# git finds each client's repository from its directory, as tools/ci/codegen_check.py does.
	@for entry in $(API_CLIENTS); do d=$${entry%%:*}; c=$${entry#*:}; m=""; \
	  if [ -d "$$d" ]; then m=$$(env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE -u GIT_COMMON_DIR -u GIT_PREFIX \
	    git -C "$$d" ls-files -- pyproject.toml package.json pubspec.yaml) || exit 1; fi; \
	  if [ -z "$$m" ]; then printf '  SKIP  %-14s %s\n' api-clients "$$d has no tracked manifest yet"; continue; fi; \
	  case " $(COMPONENTS) " in *" all "*|*" $$c "*) ;; \
	    *) printf '  SKIP  %-14s %s\n' api-clients "$$d: component $$c not selected"; continue ;; esac; \
	  if [ ! -f "$$d/Makefile" ] || ! grep -q '^generate-check:' "$$d/Makefile"; then \
	    echo "api-clients: $$d has a manifest but no Makefile with the target generate-check (component contract \"Generated client\", docs/spec/23-quality.md section 3)" >&2; exit 1; fi; \
	  $(MAKE) -C "$$d" generate-check || exit 1; \
	done

# ---- CI policy and supply chain -------------------------------------------------------
runners-check: ## Workflow policy: GitHub-hosted runners, triggers, SHA pinning, timeouts, permissions
	$(UVRUN) python tools/ci/check_runners.py .github/workflows

release-pins: ## Server version pinned by the product bundle: SemVer, no server pre-release in a stable product (docs/spec/18-release.md 2.1)
	$(UVRUN) python tools/ci/components.py pins

gitleaks: ## Secret scan of every commit reachable from HEAD (all refs: GITLEAKS_LOG_OPTS=--all)
	$(call tool,gitleaks) git --config .gitleaks.toml --redact --no-banner --log-opts="$(GITLEAKS_LOG_OPTS)" $(GITLEAKS_FLAGS) .

gitleaks-staged: ## Secret scan of the staged changes (pre-commit hook)
	$(call tool,gitleaks) git --pre-commit --staged --config .gitleaks.toml --redact --no-banner .

denylist: ## White-label scan with the private deny list (fails without the list); also PR_TITLE, PR_BODY, PR_BODY_FILE
	@base=""; \
	if git rev-parse --abbrev-ref '@{upstream}' >/dev/null 2>&1; then base='@{upstream}'; \
	elif git rev-parse --verify -q origin/main >/dev/null; then base=origin/main; fi; \
	range="$${DENYLIST_RANGE:-$${base:+$$base..HEAD}}"; \
	$(UVRUN) python tools/denylist/denylist_scan.py $${range:+--range "$$range"} $(DENYLIST_FLAGS)

denylist-history: ## White-label scan of every object and commit on every ref (public switch)
	$(UVRUN) python tools/denylist/denylist_scan.py --history $(DENYLIST_FLAGS)

semgrep: ## Project Semgrep rules (.semgrep/) and their rule tests
	@# Semgrep skips hidden directories when it looks for tests, so each rule is tested
	@# against its fixture explicitly (rule X.yaml, fixture X.<ext>).
	@for rule in .semgrep/*.yaml; do \
	  fixture=""; for f in "$${rule%.yaml}".*; do case "$$f" in *.yaml) ;; *) fixture=$$f ;; esac; done; \
	  [ -n "$$fixture" ] || { echo "semgrep: $$rule has no test fixture"; exit 1; }; \
	  $(SEMGREP) --test --metrics=off --disable-version-check --config "$$rule" "$$fixture" || exit 1; \
	done
	$(SEMGREP) scan --config .semgrep --exclude .semgrep --error --metrics=off --disable-version-check --quiet .

trivy: ## Trivy misconfiguration scan of Dockerfiles, Kubernetes, Helm and Terraform files
	@# Trivy walks a directory; it scans a copy of the committable files (tools/ci/tracked_tree.py),
	@# so ignored content such as other worktrees or caches cannot change the result.
	@files=$$(git ls-files -- '*Dockerfile*' '*.tf' '*Chart.yaml' 'infra/k8s/*'); \
	if [ -z "$$files" ]; then $(call skip,trivy,no Dockerfile or Helm chart or Kubernetes or Terraform file yet); \
	else trivy=$(call tool,trivy); tree=$$(mktemp -d); trap 'rm -rf "$$tree"' EXIT; \
	  $(UVRUN) python tools/ci/tracked_tree.py "$$tree/repo"; \
	  (cd "$$tree/repo" && "$$trivy" config --skip-check-update --exit-code 1 --severity HIGH,CRITICAL --quiet .); fi

trivy-image: ## Trivy vulnerability scan of a built image: make trivy-image IMAGE=<ref@digest>
	@[ -n "$${IMAGE:-}" ] || { echo "usage: make trivy-image IMAGE=<ref@digest>"; exit 2; }
	$(call tool,trivy) image --exit-code 1 --severity HIGH,CRITICAL --ignore-unfixed --quiet "$$IMAGE"

osv: ## Known vulnerabilities: every lock file (OSV-Scanner), Go call graphs (govulncheck)
	@locks=$$(git ls-files -- $(foreach n,$(LOCK_NAMES),'*$(n)')); \
	if [ -z "$$locks" ]; then $(call skip,osv,no lock file tracked); \
	else args=(); for l in $$locks; do args+=(-L "$$l"); done; \
	  $(call tool,osv-scanner) scan source --format table "$${args[@]}"; fi
	@if [ -z "$(GO_ON)" ]; then $(call skip,govulncheck,no Go module selected); \
	else for m in $(GO_MODS); do (cd $$m && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...) || exit 1; done; fi

licenses: ## License gate: LICENSE text, SPDX headers, dependency licenses of shipped components
	$(UVRUN) python tools/ci/license_check.py
	@$(allowed_licenses); \
	dirs="$(if $(GO_ON),$(GO_MODS)) $(if $(PY_ON),$(PY_PROJECTS)) $(if $(TS_ON),$(TS_PROJECTS))"; \
	args=(); for d in $$dirs; do for n in $(LOCK_NAMES); do \
	  for l in $$(git ls-files -- "$$d/*$$n"); do args+=(-L "$$l"); done; done; done; \
	if [ -n "$(DART_ON)" ]; then for l in $$(git ls-files -- pubspec.lock); do args+=(-L "$$l"); done; fi; \
	if [ $${#args[@]} -eq 0 ]; then $(call skip,dep-licenses,no lock file of a shipped component selected); \
	else $(call tool,osv-scanner) scan source --licenses="$$allowed" "$${args[@]}"; fi
	@# The module's own packages carry the repository license (checked above); go-licenses would
	@# look for a LICENSE only up to the module root, so it checks the dependencies alone.
	@if [ -n "$(GO_ON)" ]; then $(allowed_licenses); \
	  for m in $(GO_MODS); do \
	    mod=$$(awk '$$1 == "module" { gsub(/"/, "", $$2); print $$2; exit }' $$m/go.mod); \
	    [ -n "$$mod" ] || { echo "licenses: no module path in $$m/go.mod" >&2; exit 1; }; \
	    (cd $$m && go run github.com/google/go-licenses/v2@$(GO_LICENSES_VERSION) check ./... --allowed_licenses="$$allowed" --ignore "$$mod") || exit 1; \
	  done; fi

# ---- SonarQube ------------------------------------------------------------------------
sonar: test ## SonarQube analysis with the shared gate (local Community Build or SonarQube Cloud)
	@$(MAKE) --no-print-directory sonar-scan

# The scan alone, over the coverage reports that `make test` already wrote: the CI job sonar
# unpacks them from the gates job instead of running every test a second time.
sonar-scan: ## SonarQube analysis of the coverage reports already on disk (CI job sonar)
	@if [ -z "$(PRODUCT_SOURCES)" ]; then $(call skip,sonar,no product source code yet (tools are covered by ruff/mypy/bandit)); \
	elif [ "$${SONAR_SKIP:-0}" = 1 ]; then $(call skip,sonar,SONAR_SKIP=1 for fork contributors; maintainers and CI run it); \
	elif [ -z "$${SONAR_TOKEN:-}" ]; then echo "sonar: SONAR_TOKEN is not set (docs/quality/sonarqube-local.md)"; exit 1; \
	else $(call need_node,sonar-scanner-npm); $(NODE_BIN)/sonar-scanner-npm -Dsonar.host.url=$(SONAR_HOST_URL) -Dsonar.projectKey=$(SONAR_PROJECT_KEY) \
	  $(if $(SONAR_ORGANIZATION),-Dsonar.organization=$(SONAR_ORGANIZATION)) -Dsonar.qualitygate.wait=$(SONAR_QUALITYGATE_WAIT) \
	  || { SONAR_HOST_URL=$(SONAR_HOST_URL) SONAR_ORGANIZATION=$(SONAR_ORGANIZATION) $(UVRUN) python tools/sonarqube/quality_sync.py ce-error; exit 1; }; fi

sonar-up: ## Start the local SonarQube Community Build (tools/sonarqube/compose.yaml)
	@if [ ! -f tools/sonarqube/.env ]; then umask 077; \
	  printf 'SONARQUBE_DB_PASSWORD=%s\n' "$$(python3 -c 'import secrets; print(secrets.token_hex(24))')" > tools/sonarqube/.env; \
	  echo "  created tools/sonarqube/.env with a random database password"; fi
	docker compose --env-file tools/sonarqube/.env -f tools/sonarqube/compose.yaml up -d --wait
	@url="http://127.0.0.1:$${SONARQUBE_PORT:-9000}/api/system/status"; \
	for i in $$(seq 1 $(SONAR_STARTUP_TIMEOUT_S)); do \
	  if curl -fsS "$$url" 2>/dev/null | grep -q '"status":"UP"'; then echo "  SonarQube is up"; exit 0; fi; sleep 1; \
	done; echo "SonarQube did not report UP within $(SONAR_STARTUP_TIMEOUT_S)s"; exit 1

sonar-down: ## Stop the local SonarQube (data stays in its volumes)
	docker compose --env-file tools/sonarqube/.env -f tools/sonarqube/compose.yaml down

sonar-config: ## Apply quality gate and profiles (quality/sonar) to SONAR_HOST_URL
	SONAR_HOST_URL=$(SONAR_HOST_URL) SONAR_ORGANIZATION=$(SONAR_ORGANIZATION) SONAR_PROJECT_KEY=$(SONAR_PROJECT_KEY) \
	  $(UVRUN) python tools/sonarqube/quality_sync.py apply

sonar-config-check: ## Check that SONAR_HOST_URL enforces exactly quality/sonar
	SONAR_HOST_URL=$(SONAR_HOST_URL) SONAR_ORGANIZATION=$(SONAR_ORGANIZATION) SONAR_PROJECT_KEY=$(SONAR_PROJECT_KEY) \
	  $(UVRUN) python tools/sonarqube/quality_sync.py check

sonar-main-branch: ## Name the main branch of the SonarQube Cloud project main
	SONAR_HOST_URL=$(SONAR_HOST_URL) SONAR_ORGANIZATION=$(SONAR_ORGANIZATION) SONAR_PROJECT_KEY=$(SONAR_PROJECT_KEY) \
	  $(UVRUN) python tools/sonarqube/quality_sync.py main-branch

sonar-visibility: ## Make the SonarQube Cloud project public (a private one is capped in lines)
	SONAR_HOST_URL=$(SONAR_HOST_URL) SONAR_ORGANIZATION=$(SONAR_ORGANIZATION) SONAR_PROJECT_KEY=$(SONAR_PROJECT_KEY) \
	  $(UVRUN) python tools/sonarqube/quality_sync.py visibility

sonar-new-code: ## Give the SonarQube Cloud project the new code of the last 30 days unless set
	SONAR_HOST_URL=$(SONAR_HOST_URL) SONAR_ORGANIZATION=$(SONAR_ORGANIZATION) SONAR_PROJECT_KEY=$(SONAR_PROJECT_KEY) \
	  $(UVRUN) python tools/sonarqube/quality_sync.py new-code

# ---- release support ------------------------------------------------------------------
release-build: ## Product release bundle (tag v<version>): Compose installation, license, changelog
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-build VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	@[ -f infra/compose/compose.yaml ] || { echo "release-build: infra/compose/compose.yaml is missing"; exit 1; }
	@[ "$$(cat VERSION)" = "$$VERSION" ] || { echo "release-build: VERSION file says $$(cat VERSION), not $$VERSION"; exit 1; }
	mkdir -p "$$OUT"
	git archive --format=tar.gz --prefix="bothy-$$VERSION/" -o "$$OUT/bothy-compose-$$VERSION.tar.gz" HEAD \
	  infra/compose ':(exclude)infra/compose/test' LICENSE NOTICE CHANGELOG.md VERSION

release-prep: ## Prepare a component release (ADR 0060): make release-prep COMPONENT=<name> [CHANNEL=beta]
	@[ -n "$${COMPONENT:-}" ] || { echo "usage: make release-prep COMPONENT=<name> [CHANNEL=beta]"; exit 2; }
	@gc=$(call tool,git-cliff) || exit 1; \
	$(UVRUN) python tools/ci/release_prep.py --component "$$COMPONENT" --channel "$${CHANNEL:-stable}" --git-cliff "$$gc"

# Trial release (D-W-127): the release steps of a component against a local registry on
# 127.0.0.1, a throwaway cosign key pair and a temporary directory, then the verification an
# instance operator runs. Never under GitHub Actions; never a tag, a release or a remote push.
# TRIAL_PLATFORMS narrows the platforms (default: linux/amd64,linux/arm64).
TRIAL_PLATFORMS ?=
release-trial: ## Trial release against local throwaway targets: make release-trial COMPONENT=server
	@[ -n "$${COMPONENT:-}" ] || { echo "usage: make release-trial COMPONENT=server [TRIAL_PLATFORMS=linux/amd64,linux/arm64]"; exit 2; }
	@[ -z "$${GITHUB_ACTIONS:-}" ] || { echo "release-trial: refuses to run under GitHub Actions" >&2; exit 2; }
	@docker buildx version >/dev/null 2>&1 || { echo "release-trial: docker buildx is required (CONTRIBUTING.md)" >&2; exit 1; }
	@cosign=$(call tool,cosign) && syft=$(call tool,syft) || exit 1; \
	$(UVRUN) python tools/ci/release_trial.py --component "$$COMPONENT" --cosign "$$cosign" --syft "$$syft" \
	  $(if $(TRIAL_PLATFORMS),--platforms '$(TRIAL_PLATFORMS)')

# License and copyright scan of the tracked tree before every release (docs/quality/licenses.md
# section 4). ScanCode runs in a container built from tools/scancode/Dockerfile (base image by
# digest, release archive by sha256); its native parts exist for linux/amd64 only, so on an arm64
# host it runs emulated and slowly. Not a pull request gate: a run over the whole tree takes long.
SCANCODE_VERSION := $(shell sed -n 's/^ARG SCANCODE_VERSION=//p' tools/scancode/Dockerfile)
SCANCODE_IMAGE ?= bothy-scancode:$(SCANCODE_VERSION)-local
SCANCODE_OUT ?= .cache/scancode
# Parallel ScanCode processes; each needs up to about 2 GB on the largest files of the tree.
SCANCODE_PROCESSES ?= 4
scancode: ## ScanCode over the tracked tree with the license gate (release step): make scancode [SCANCODE_OUT=<dir>]
	@docker buildx version >/dev/null 2>&1 || { echo "scancode: docker buildx is required (CONTRIBUTING.md)" >&2; exit 1; }
	docker buildx build --platform linux/amd64 --load --tag '$(SCANCODE_IMAGE)' tools/scancode
	@mkdir -p '$(SCANCODE_OUT)' || exit 1; out=$$(cd '$(SCANCODE_OUT)' && pwd) || exit 1; \
	tree=$$(mktemp -d "$${TMPDIR:-/tmp}/bothy-scancode.XXXXXX") || exit 1; trap 'rm -rf "$$tree"' EXIT; \
	git archive HEAD | tar -x -C "$$tree" || exit 1; \
	docker run --rm --platform linux/amd64 --user "$$(id -u):$$(id -g)" --network none \
	  -v "$$tree:/scan/tree:ro" -v "$$out:/scan/out" '$(SCANCODE_IMAGE)' \
	  --license --copyright --strip-root --processes '$(SCANCODE_PROCESSES)' --timeout 600 --quiet \
	  --json-pp /scan/out/scancode.json /scan/tree || exit 1; \
	$(UVRUN) python tools/ci/scancode_policy.py --report "$$out/scancode.json"

# ---- server image (ADR 0012; not part of `gates`: needs Docker with buildx, see CONTRIBUTING.md) --
# The release (M7) and the Compose smoke test (M1-73) start from `make image`. The local tag is
# `<version>-local`, never `latest` (docs/spec/18-release.md "Channels" rule 3).
SERVER_VERSION := $(shell cat services/server/VERSION 2>/dev/null)
IMAGE_NAME ?= bothy-server
IMAGE_TAG ?= $(SERVER_VERSION)-local
IMAGE_PLATFORMS ?= linux/amd64,linux/arm64
IMAGE_OCI ?= .cache/image/bothy-server.oci.tar
# Empty: the current buildx builder. It must build several platforms (see `make image`).
IMAGE_BUILDER ?=
image_build = docker buildx build $(if $(IMAGE_BUILDER),--builder '$(IMAGE_BUILDER)') --file services/server/Dockerfile \
  --build-arg VERSION='$(SERVER_VERSION)' --build-arg REVISION="$$rev"

image: ## Build bothy-server for IMAGE_PLATFORMS into IMAGE_OCI and load the host platform as IMAGE_NAME:IMAGE_TAG
	@docker buildx version >/dev/null 2>&1 || { echo "image: docker buildx is required (Docker with the buildx plugin, CONTRIBUTING.md)" >&2; exit 1; }
	@[ -n "$(SERVER_VERSION)" ] || { echo "image: services/server/VERSION is empty" >&2; exit 1; }
	@[ "$(IMAGE_TAG)" != latest ] || { echo "image: the tag latest is not used (docs/spec/18-release.md \"Channels\" rule 3)" >&2; exit 1; }
	@mkdir -p "$(dir $(IMAGE_OCI))" || exit 1
	@rev=$$(git rev-parse HEAD) || exit 1; \
	log=$$(mktemp "$${TMPDIR:-/tmp}/bothy-image.XXXXXX") || exit 1; \
	rc=0; $(image_build) --platform '$(IMAGE_PLATFORMS)' --output 'type=oci,dest=$(IMAGE_OCI)' . 2>&1 | tee "$$log" || rc=$$?; \
	if [ "$$rc" -ne 0 ]; then \
	  if grep -qiE 'multi-?platform|multiple platforms' "$$log"; then \
	    echo "image: the buildx builder cannot build $(IMAGE_PLATFORMS). Create one and pass it:" >&2; \
	    echo "  docker buildx create --name <name> --driver docker-container" >&2; \
	    echo "  make image IMAGE_BUILDER=<name>" >&2; fi; \
	  rm -f "$$log"; exit 1; fi; \
	rm -f "$$log"; \
	$(image_build) --load --tag '$(IMAGE_NAME):$(IMAGE_TAG)' . || exit 1
	@echo "  image: $(IMAGE_NAME):$(IMAGE_TAG) loaded (host platform), $(IMAGE_PLATFORMS) in $(IMAGE_OCI)"

image-test: ## Structure tests of the image from make image (IMAGE=<ref> tests another loaded image)
	@docker info >/dev/null 2>&1 || { echo "image-test: no Docker runtime reachable; the image tests need one (P-09)" >&2; exit 1; }
	@[ -f "$(IMAGE_OCI)" ] || { echo "image-test: $(IMAGE_OCI) is missing; run make image first" >&2; exit 1; }
	@gl=$(call tool,golangci-lint) || exit 1; \
	cd services/server && "$$gl" run --build-tags imagetest ./test/image/... || exit 1; \
	BOTHY_TEST_IMAGE='$(or $(IMAGE),$(IMAGE_NAME):$(IMAGE_TAG))' BOTHY_TEST_IMAGE_OCI='$(abspath $(IMAGE_OCI))' \
	  go test -tags imagetest -count=1 ./test/image/... || exit 1

# ---- database image (brief M7-01; not part of `gates`: needs Docker with buildx) ------------------
# bothy-postgres is postgres:18 plus pgBackRest and bothy-backupd; its tag is pg18-<server version>
# (docs/spec/18-release.md "Server images" rule 4), locally with the suffix -local, never latest.
# No build step runs on the target platform, so one host builds both platforms natively.
POSTGRES_IMAGE_NAME ?= bothy-postgres
POSTGRES_IMAGE_TAG ?= pg18-$(SERVER_VERSION)-local
POSTGRES_IMAGE_OCI ?= .cache/image/bothy-postgres.oci.tar
postgres_image_build = docker buildx build $(if $(IMAGE_BUILDER),--builder '$(IMAGE_BUILDER)') \
  --file services/server/images/bothy-postgres/Dockerfile \
  --build-arg VERSION='$(SERVER_VERSION)' --build-arg REVISION="$$rev"

postgres-image: ## Build bothy-postgres for IMAGE_PLATFORMS into POSTGRES_IMAGE_OCI and load the host platform
	@docker buildx version >/dev/null 2>&1 || { echo "postgres-image: docker buildx is required (Docker with the buildx plugin, CONTRIBUTING.md)" >&2; exit 1; }
	@[ -n "$(SERVER_VERSION)" ] || { echo "postgres-image: services/server/VERSION is empty" >&2; exit 1; }
	@case '$(POSTGRES_IMAGE_TAG)' in latest|*-latest) echo "postgres-image: the tag latest is not used (docs/spec/18-release.md \"Channels\" rule 3)" >&2; exit 1;; esac
	@mkdir -p "$(dir $(POSTGRES_IMAGE_OCI))" || exit 1
	@rev=$$(git rev-parse HEAD) || exit 1; \
	$(postgres_image_build) --platform '$(IMAGE_PLATFORMS)' --output 'type=oci,dest=$(POSTGRES_IMAGE_OCI)' . || exit 1; \
	$(postgres_image_build) --load --tag '$(POSTGRES_IMAGE_NAME):$(POSTGRES_IMAGE_TAG)' . || exit 1
	@echo "  postgres-image: $(POSTGRES_IMAGE_NAME):$(POSTGRES_IMAGE_TAG) loaded (host platform), $(IMAGE_PLATFORMS) in $(POSTGRES_IMAGE_OCI)"

postgres-image-test: ## Two-container test of bothy-postgres: archive-push, full backup, PITR, verify (after make postgres-image)
	@docker info >/dev/null 2>&1 || { echo "postgres-image-test: no Docker runtime reachable; the image tests need one (P-09)" >&2; exit 1; }
	@[ -f "$(POSTGRES_IMAGE_OCI)" ] || { echo "postgres-image-test: $(POSTGRES_IMAGE_OCI) is missing; run make postgres-image first" >&2; exit 1; }
	@gl=$(call tool,golangci-lint) || exit 1; \
	cd services/server && "$$gl" run --build-tags imagetest ./test/postgresimage/... || exit 1; \
	BOTHY_TEST_IMAGE='$(or $(POSTGRES_IMAGE),$(POSTGRES_IMAGE_NAME):$(POSTGRES_IMAGE_TAG))' BOTHY_TEST_IMAGE_OCI='$(abspath $(POSTGRES_IMAGE_OCI))' \
	  go test -tags imagetest -count=1 -v -timeout 20m ./test/postgresimage/... || exit 1

portability: ## Compose smoke test with the server image (SMOKE_SERVER_IMAGE, default the tag of make image; infra/compose/test/README.md)
	@[ -x infra/compose/smoke.sh ] || { echo "portability: infra/compose/smoke.sh is missing (docs/quality/gates.md)"; exit 1; }
	infra/compose/smoke.sh

# ---- platform tests of the apps (not part of `gates`: they need a desktop session or a device) -
apps-platform: ## Platform tests of the members on this host and an attached Android device (platform-tests.json; CI jobs apps-macos, apps-windows, apps-android)
	$(UVRUN) python tools/ci/apps_platform.py

# ---- review and repository maintenance -------------------------------------------------
review: ## Local AI review before a push (CodeRabbit CLI only with BOTHY_CODERABBIT=1)
	@if [ "$${BOTHY_CODERABBIT:-0}" = 1 ]; then \
	  command -v coderabbit >/dev/null || { echo "coderabbit CLI not installed"; exit 1; }; \
	  coderabbit review --committed --base "$(REVIEW_BASE)"; \
	else echo "  CodeRabbit CLI off: it sends the diff to the CodeRabbit service (docs/quality/push-discipline.md)"; fi
	@mkdir -p .review
	@echo "  independent review: a fresh read-only reviewer checks the diff against $(REVIEW_BASE)"
	@echo "  (docs/quality/red-team-gate.md) and writes .review/$$(git rev-parse HEAD).md;"
	@echo "  the pre-push hook requires its last line to be 'Urteil: JA'. The report stays local;"
	@echo "  the pull request names the reviewed commit and the verdict in fresh words"
	$(UVRUN) python tools/hooks/review_gate.py || true

labels-check: ## Validate .github/labels.yml and the labels used by templates
	$(UVRUN) python tools/ci/labels.py check

labels-sync: ## Create or update the GitHub labels (needs gh login; maintainers)
	$(UVRUN) python tools/ci/labels.py sync

tools-lock: ## Re-pin a release binary: make tools-lock TOOL=<name> VERSION=<x.y.z>
	@[ -n "$${TOOL:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make tools-lock TOOL=<name> VERSION=<x.y.z>"; exit 2; }
	$(FETCH) refresh "$$TOOL" "$$VERSION"

clean: ## Remove build outputs (keeps the tool caches)
	rm -rf site .cache/coverage docs/api/dart
