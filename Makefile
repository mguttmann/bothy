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
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
MAKEFLAGS += --no-builtin-rules --no-print-directory

# ---- configuration (named values: docs/spec/23-qualitaet.md) -------------------------
COMPONENTS ?= all
COVERAGE_GO_MIN ?= 80
COVERAGE_DART_MIN ?= 80
SONAR_HOST_URL ?= http://127.0.0.1:9000
SONAR_ORGANIZATION ?=
SONAR_PROJECT_KEY ?= bothy
SONAR_STARTUP_TIMEOUT_S ?= 300
# renovate: datasource=go depName=github.com/google/go-licenses/v2
GO_LICENSES_VERSION := v2.0.1
# renovate: datasource=go depName=golang.org/x/vuln
GOVULNCHECK_VERSION := v1.8.0
# renovate: datasource=pypi depName=semgrep
SEMGREP_VERSION := 1.180.0
REVIEW_BASE ?= main
DENYLIST_FLAGS ?=
GITLEAKS_FLAGS ?=
GITLEAKS_LOG_OPTS ?= HEAD

UV := uv
UVRUN := $(UV) run --project tools --locked --quiet
NODE_BIN := tools/node/node_modules/.bin
FETCH := $(UVRUN) python tools/bin/fetch_tools.py
SEMGREP := $(UV)x --quiet --python 3.12 --from semgrep==$(SEMGREP_VERSION) semgrep
# Path of a pinned release binary, resolved (and fetched on first use) inside a recipe.
tool = "$$($(FETCH) path $(1))"
skip = printf '  SKIP  %-14s %s\n' '$(1)' '$(2)'
want = $(if $(filter all $(1),$(COMPONENTS)),yes,)

# ---- component discovery (tracked files only, like CI) --------------------------------
GO_MODS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'services/*go.mod' 2>/dev/null))))
DART_PKGS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'apps/*pubspec.yaml' 'packages/*pubspec.yaml' 2>/dev/null))))
PY_PROJECTS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'sdk/python/pyproject.toml' 'plugins/*pyproject.toml' 2>/dev/null))))
TS_PROJECTS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'sdk/typescript/package.json' 'plugins/*package.json' 2>/dev/null))))
# Lock files OSV-Scanner reads; named explicitly instead of a directory walk, so that
# only tracked files count and .gitignore rules of enclosing checkouts cannot hide them.
LOCK_NAMES := uv.lock package-lock.json pnpm-lock.yaml pubspec.lock go.mod
PRODUCT_SOURCES := $(shell git ls-files -- 'services/*.go' 'apps/*.dart' 'packages/*.dart' 'sdk/*.py' 'sdk/*.ts' 'plugins/*.py' 'plugins/*.ts' 2>/dev/null | head -n 1)
GO_ON := $(and $(GO_MODS),$(call want,server))
DART_ON := $(and $(DART_PKGS),$(call want,apps))
PY_ON := $(and $(PY_PROJECTS),$(or $(call want,sdk-python),$(call want,plugins)))
TS_ON := $(and $(TS_PROJECTS),$(or $(call want,sdk-typescript),$(call want,plugins)))

.PHONY: help bootstrap bootstrap-ci check gates fmt fmt-check lint typecheck test coverage \
        sonar sonar-up sonar-down sonar-config sonar-config-check semgrep trivy osv gitleaks \
        gitleaks-staged denylist denylist-history licenses docs adr-index config-reference linkcheck \
        trivy-image portability linkcheck-external diagrams tokens api-lint api-clients runners-check commitlint \
        review labels-check labels-sync tools-lock release-build clean

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
	@docker info >/dev/null 2>&1 && echo "  docker: ok (needed for make sonar-up)" || \
	  echo "  docker: not reachable; make sonar-up needs a Docker-compatible runtime"

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

# ---- aggregate ------------------------------------------------------------------------
check: gates denylist sonar ## Every gate, identical to CI (before every push)
	@echo "check: all gates passed"

gates: ## All gates except denylist and sonar (CI job "gates")
gates: fmt-check lint typecheck test coverage commitlint docs linkcheck diagrams api-lint \
       api-clients tokens runners-check labels-check gitleaks semgrep trivy osv licenses

# ---- code quality ---------------------------------------------------------------------
fmt: ## Format all code in place
	$(UVRUN) ruff format tools
	$(UVRUN) ruff check --fix tools
	@if [ -n "$(GO_ON)" ]; then for m in $(GO_MODS); do (cd $$m && gofmt -w .); done; fi
	@if [ -n "$(DART_ON)" ]; then for p in $(DART_PKGS); do (cd $$p && dart format .); done; fi
	@if [ -n "$(PY_ON)" ]; then for p in $(PY_PROJECTS); do $(UV) run --project $$p --locked ruff format $$p; done; fi
	@if [ -n "$(TS_ON)" ]; then for p in $(TS_PROJECTS); do npm --prefix $$p run format; done; fi

fmt-check: ## Formatting is clean (no changes)
	$(UVRUN) ruff format --check tools
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-fmt,no Go module selected under services/); \
	else for m in $(GO_MODS); do out=$$(cd $$m && gofmt -l .); [ -z "$$out" ] || { echo "$$out"; exit 1; }; done; fi
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-format,no Dart package selected under apps/ or packages/); \
	else for p in $(DART_PKGS); do (cd $$p && dart format --output=none --set-exit-if-changed .); done; fi
	@if [ -z "$(PY_ON)" ]; then $(call skip,py-format,no Python project selected under sdk/ or plugins/); \
	else for p in $(PY_PROJECTS); do $(UV) run --project $$p --locked ruff format --check $$p; done; fi
	@if [ -z "$(TS_ON)" ]; then $(call skip,ts-format,no TypeScript project selected); \
	else for p in $(TS_PROJECTS); do npm --prefix $$p run format:check; done; fi

lint: ## Linters: file hygiene hooks, ruff, bandit, actionlint, zizmor, component linters
	SKIP=ruff-format,ruff-check,gitleaks-staged $(UVRUN) pre-commit run --all-files --hook-stage pre-commit
	$(UVRUN) ruff check tools
	$(UVRUN) bandit -q -c tools/pyproject.toml -r tools/bin tools/ci tools/denylist tools/docs tools/hooks tools/sonarqube
	$(UVRUN) check-jsonschema --builtin-schema vendor.github-issue-forms $(filter-out %/config.yml,$(wildcard .github/ISSUE_TEMPLATE/*.yml))
	$(UVRUN) check-jsonschema --builtin-schema vendor.github-issue-config .github/ISSUE_TEMPLATE/config.yml
	$(UVRUN) check-jsonschema --builtin-schema vendor.renovate renovate.json
	$(UVRUN) check-jsonschema --schemafile https://coderabbit.ai/integrations/schema.v2.json .coderabbit.yaml
	$(call tool,actionlint) -shellcheck= -pyflakes= .github/workflows/*.yml
	@if [ -n "$${GH_TOKEN:-}" ]; then $(UVRUN) zizmor --min-severity low .github/workflows; \
	else echo "  zizmor: offline (set GH_TOKEN for the online audits)"; $(UVRUN) zizmor --offline --min-severity low .github/workflows; fi
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-lint,no Go module selected); \
	else for m in $(GO_MODS); do (cd $$m && go vet ./... && $(call tool,golangci-lint) run ./...); done; fi
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-lint,no Dart package selected); \
	else for p in $(DART_PKGS); do (cd $$p && flutter analyze --fatal-infos && \
	  if grep -q '^ *custom_lint:' pubspec.yaml; then dart run custom_lint; fi); done; fi
	@if [ -z "$(PY_ON)" ]; then $(call skip,py-lint,no Python project selected); \
	else for p in $(PY_PROJECTS); do $(UV) run --project $$p --locked ruff check $$p && \
	  $(UVRUN) bandit -q -r $$p -x $$p/tests; done; fi
	@if [ -z "$(TS_ON)" ]; then $(call skip,ts-lint,no TypeScript project selected); \
	else for p in $(TS_PROJECTS); do npm --prefix $$p run lint; done; fi

typecheck: ## Type checks: mypy (strict), go build, flutter analyze, tsc
	cd tools && uv run --locked --quiet mypy
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-build,no Go module selected); \
	else for m in $(GO_MODS); do (cd $$m && go build ./...); done; fi
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-types,no Dart package selected); \
	else echo "  dart types: covered by flutter analyze in make lint"; fi
	@if [ -z "$(PY_ON)" ]; then $(call skip,py-types,no Python project selected); \
	else for p in $(PY_PROJECTS); do $(UV) run --project $$p --locked mypy; done; fi
	@if [ -z "$(TS_ON)" ]; then $(call skip,ts-types,no TypeScript project selected); \
	else for p in $(TS_PROJECTS); do npm --prefix $$p run typecheck; done; fi

test: ## All tests with coverage data
	@mkdir -p .cache/coverage
	cd tools && uv run --locked --quiet pytest --cov --cov-report=xml:../.cache/coverage/tools.xml --cov-report=term
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-test,no Go module selected); \
	else for m in $(GO_MODS); do (cd $$m && go test -race -covermode=atomic -coverprofile=coverage.out ./...); done; fi
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-test,no Dart package selected); \
	else for p in $(DART_PKGS); do (cd $$p && if [ -d test ]; then flutter test --coverage; fi); done; fi
	@if [ -z "$(PY_ON)" ]; then $(call skip,py-test,no Python project selected); \
	else for p in $(PY_PROJECTS); do (cd $$p && $(UV) run --locked pytest --cov --cov-report=xml:coverage.xml); done; fi
	@if [ -z "$(TS_ON)" ]; then $(call skip,ts-test,no TypeScript project selected); \
	else for p in $(TS_PROJECTS); do npm --prefix $$p test; done; fi

coverage: test ## Coverage thresholds (named values quality.coverage.*)
	cd tools && uv run --locked --quiet coverage report >/dev/null && echo "  tools coverage: ok (fail_under in tools/pyproject.toml)"
	@if [ -z "$(GO_ON)" ]; then $(call skip,go-coverage,no Go module selected); \
	else for m in $(GO_MODS); do total=$$(cd $$m && go tool cover -func=coverage.out | awk '/^total:/ {sub("%","",$$3); print $$3}'); \
	  awk -v t="$$total" -v min=$(COVERAGE_GO_MIN) -v m=$$m 'BEGIN { if (t+0 < min) { printf "%s: coverage %s%% < %s%%\n", m, t, min; exit 1 } }'; done; fi
	@if [ -z "$(DART_ON)" ]; then $(call skip,dart-coverage,no Dart package selected); \
	else for p in $(DART_PKGS); do [ -f $$p/coverage/lcov.info ] || continue; \
	  awk -F: -v min=$(COVERAGE_DART_MIN) -v p=$$p '/^LF:/ {lf+=$$2} /^LH:/ {lh+=$$2} END { pct = lf ? 100*lh/lf : 100; if (pct < min) { printf "%s: coverage %.1f%% < %s%%\n", p, pct, min; exit 1 } }' $$p/coverage/lcov.info; done; fi
	@[ -z "$(PY_ON)$(TS_ON)" ] || echo "  python/typescript components enforce their threshold in their own test config"

commitlint: ## Conventional Commits and DCO sign-off for commits not yet on the remote main
	@base=""; \
	if git rev-parse --abbrev-ref '@{upstream}' >/dev/null 2>&1; then base='@{upstream}'; \
	elif git rev-parse --verify -q origin/main >/dev/null; then base=origin/main; fi; \
	range="$${COMMIT_RANGE:-$${base:+$$base..HEAD}}"; \
	if [ -z "$$range" ]; then $(call skip,commitlint,no remote main yet; the commit-msg hook checks each commit); \
	else $(UVRUN) python tools/hooks/commit_msg.py --range "$$range" --require-signoff; fi
	@# The pull request title becomes the squash commit on main (CI sets PR_TITLE).
	@if [ -n "$${PR_TITLE:-}" ]; then f=$$(mktemp); printf '%s\n' "$$PR_TITLE" > "$$f"; \
	  rc=0; $(UVRUN) python tools/hooks/commit_msg.py "$$f" || rc=$$?; rm -f "$$f"; exit $$rc; fi

# ---- documentation --------------------------------------------------------------------
docs: ## Documentation site builds strictly; ADR index and configuration reference are current
	$(UVRUN) python tools/docs/adr_index.py --check
	$(UVRUN) python tools/docs/config_reference.py --check
	@# No --quiet: it hides the warnings that --strict turns into a failure.
	$(UVRUN) mkdocs build --strict --site-dir site

adr-index: ## Regenerate the ADR index in docs/adr/README.md
	$(UVRUN) python tools/docs/adr_index.py

config-reference: ## Regenerate section 4 of docs/operations/configuration.md from the chapter tables
	$(UVRUN) python tools/docs/config_reference.py

linkcheck: ## Local links and anchors of every tracked Markdown file (offline)
	git ls-files -z -- '*.md' | xargs -0 $(call tool,lychee) --offline --include-fragments --no-progress --quiet

linkcheck-external: ## External links too (network, not a gate)
	git ls-files -z -- '*.md' | xargs -0 $(call tool,lychee) --include-fragments --no-progress --max-concurrency 8

diagrams: ## Every Mermaid diagram renders; every .mmd has its .svg
	$(UVRUN) python tools/docs/render_diagrams.py

tokens: ## Design-token build check (contract: tools/design-tokens/Makefile target check)
	@if [ ! -d tools/design-tokens ]; then $(call skip,tokens,tools/design-tokens does not exist yet); \
	elif [ -f tools/design-tokens/Makefile ]; then $(MAKE) -C tools/design-tokens check; \
	else echo "tokens: tools/design-tokens exists but has no Makefile with target 'check'"; exit 1; fi

api-lint: ## Redocly lint of the OpenAPI and AsyncAPI documents
	@if [ ! -f docs/api/openapi.yaml ]; then $(call skip,api-lint,docs/api/openapi.yaml does not exist yet); \
	else cfg=""; [ -f docs/api/redocly.yaml ] && cfg="--config docs/api/redocly.yaml"; \
	  files="docs/api/openapi.yaml"; [ -f docs/api/asyncapi.yaml ] && files="$$files docs/api/asyncapi.yaml"; \
	  REDOCLY_TELEMETRY=off $(NODE_BIN)/redocly lint $$cfg $$files; fi

api-clients: ## Generated API clients compile (contract: each SDK/app provides `make -C <dir> generate-check`)
	@found=""; for d in sdk/python sdk/typescript packages/api_client; do \
	  if [ -f $$d/Makefile ] && grep -q '^generate-check:' $$d/Makefile; then found=1; $(MAKE) -C $$d generate-check; fi; done; \
	[ -n "$$found" ] || $(call skip,api-clients,no client package with a generate-check target yet)

# ---- CI policy and supply chain -------------------------------------------------------
runners-check: ## Workflow policy: runner switch, triggers, SHA pinning, timeouts, permissions
	$(UVRUN) python tools/ci/check_runners.py .github/workflows

gitleaks: ## Secret scan of every commit reachable from HEAD (all refs: GITLEAKS_LOG_OPTS=--all)
	$(call tool,gitleaks) git --config .gitleaks.toml --redact --no-banner --log-opts="$(GITLEAKS_LOG_OPTS)" $(GITLEAKS_FLAGS) .

gitleaks-staged: ## Secret scan of the staged changes (pre-commit hook)
	$(call tool,gitleaks) git --pre-commit --staged --config .gitleaks.toml --redact --no-banner .

denylist: ## White-label scan with the private deny list (fails without the list)
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
	  $(SEMGREP) --test --metrics=off --disable-version-check --config "$$rule" "$$fixture"; \
	done
	$(SEMGREP) scan --config .semgrep --exclude .semgrep --error --metrics=off --disable-version-check --quiet .

trivy: ## Trivy misconfiguration scan of Dockerfiles, Kubernetes, Helm and Terraform files
	@files=$$(git ls-files -- '*Dockerfile*' '*.tf' '*Chart.yaml' 'infra/k8s/*'); \
	if [ -z "$$files" ]; then $(call skip,trivy,no Dockerfile or Helm chart or Kubernetes or Terraform file yet); \
	else $(call tool,trivy) config --skip-check-update --exit-code 1 --severity HIGH,CRITICAL --quiet .; fi

trivy-image: ## Trivy vulnerability scan of a built image: make trivy-image IMAGE=<ref@digest>
	@[ -n "$${IMAGE:-}" ] || { echo "usage: make trivy-image IMAGE=<ref@digest>"; exit 2; }
	$(call tool,trivy) image --exit-code 1 --severity HIGH,CRITICAL --ignore-unfixed --quiet "$$IMAGE"

osv: ## Known vulnerabilities: every lock file (OSV-Scanner), Go call graphs (govulncheck)
	@locks=$$(git ls-files -- $(foreach n,$(LOCK_NAMES),'*$(n)')); \
	if [ -z "$$locks" ]; then $(call skip,osv,no lock file tracked); \
	else args=(); for l in $$locks; do args+=(-L "$$l"); done; \
	  $(call tool,osv-scanner) scan source --format table "$${args[@]}"; fi
	@if [ -z "$(GO_ON)" ]; then $(call skip,govulncheck,no Go module selected); \
	else for m in $(GO_MODS); do (cd $$m && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...); done; fi

licenses: ## License gate: LICENSE text, SPDX headers, dependency licenses of shipped components
	$(UVRUN) python tools/ci/license_check.py
	@allowed=$$(grep -v '^#' quality/licenses/allowed.txt | grep . | paste -sd, -); \
	dirs="$(if $(GO_ON),$(GO_MODS)) $(if $(DART_ON),$(DART_PKGS)) $(if $(PY_ON),$(PY_PROJECTS)) $(if $(TS_ON),$(TS_PROJECTS))"; \
	args=(); for d in $$dirs; do for n in $(LOCK_NAMES); do \
	  for l in $$(git ls-files -- "$$d/*$$n"); do args+=(-L "$$l"); done; done; done; \
	if [ $${#args[@]} -eq 0 ]; then $(call skip,dep-licenses,no lock file of a shipped component selected); \
	else $(call tool,osv-scanner) scan source --licenses="$$allowed" "$${args[@]}"; fi
	@if [ -n "$(GO_ON)" ]; then allowed=$$(grep -v '^#' quality/licenses/allowed.txt | grep . | paste -sd, -); \
	  for m in $(GO_MODS); do (cd $$m && go run github.com/google/go-licenses/v2@$(GO_LICENSES_VERSION) check ./... --allowed_licenses="$$allowed"); done; fi

# ---- SonarQube ------------------------------------------------------------------------
sonar: test ## SonarQube analysis with the shared gate (local Community Build or SonarQube Cloud)
	@if [ -z "$(PRODUCT_SOURCES)" ]; then $(call skip,sonar,no product source code yet (tools are covered by ruff/mypy/bandit)); \
	elif [ "$${SONAR_SKIP:-0}" = 1 ]; then $(call skip,sonar,SONAR_SKIP=1 for fork contributors; maintainers and CI run it); \
	elif [ -z "$${SONAR_TOKEN:-}" ]; then echo "sonar: SONAR_TOKEN is not set (docs/quality/sonarqube-local.md)"; exit 1; \
	else $(NODE_BIN)/sonar-scanner-npm -Dsonar.host.url=$(SONAR_HOST_URL) -Dsonar.projectKey=$(SONAR_PROJECT_KEY) \
	  $(if $(SONAR_ORGANIZATION),-Dsonar.organization=$(SONAR_ORGANIZATION)) -Dsonar.qualitygate.wait=true; fi

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
	SONAR_HOST_URL=$(SONAR_HOST_URL) SONAR_ORGANIZATION=$(SONAR_ORGANIZATION) $(UVRUN) python tools/sonarqube/quality_sync.py apply

sonar-config-check: ## Check that SONAR_HOST_URL enforces exactly quality/sonar
	SONAR_HOST_URL=$(SONAR_HOST_URL) SONAR_ORGANIZATION=$(SONAR_ORGANIZATION) $(UVRUN) python tools/sonarqube/quality_sync.py check

# ---- release support ------------------------------------------------------------------
release-build: ## Product release bundle (tag v<version>): Compose installation, license, changelog
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-build VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	@[ -f infra/compose/compose.yaml ] || { echo "release-build: infra/compose/compose.yaml is missing"; exit 1; }
	@[ "$$(cat VERSION)" = "$$VERSION" ] || { echo "release-build: VERSION file says $$(cat VERSION), not $$VERSION"; exit 1; }
	mkdir -p "$$OUT"
	git archive --format=tar.gz --prefix="bothy-$$VERSION/" -o "$$OUT/bothy-compose-$$VERSION.tar.gz" HEAD \
	  infra/compose LICENSE NOTICE CHANGELOG.md VERSION

portability: ## Full stack from images on this machine plus smoke test (contract: infra/compose/smoke.sh)
	@[ -x infra/compose/smoke.sh ] || { echo "portability: infra/compose/smoke.sh is missing (docs/quality/gates.md)"; exit 1; }
	infra/compose/smoke.sh

# ---- review and repository maintenance -------------------------------------------------
review: ## Local AI review before a push (CodeRabbit CLI only with BOTHY_CODERABBIT=1)
	@if [ "$${BOTHY_CODERABBIT:-0}" = 1 ]; then \
	  command -v coderabbit >/dev/null || { echo "coderabbit CLI not installed"; exit 1; }; \
	  coderabbit review --committed --base "$(REVIEW_BASE)"; \
	else echo "  CodeRabbit CLI off: it sends the diff to the CodeRabbit service (docs/quality/push-discipline.md)"; fi
	@echo "  red team: run the independent read-only review of docs/quality/red-team-gate.md"

labels-check: ## Validate .github/labels.yml and the labels used by templates
	$(UVRUN) python tools/ci/labels.py check

labels-sync: ## Create or update the GitHub labels (needs gh login; maintainers)
	$(UVRUN) python tools/ci/labels.py sync

tools-lock: ## Re-pin a release binary: make tools-lock TOOL=<name> VERSION=<x.y.z>
	@[ -n "$${TOOL:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make tools-lock TOOL=<name> VERSION=<x.y.z>"; exit 2; }
	$(FETCH) refresh "$$TOOL" "$$VERSION"

clean: ## Remove build outputs (keeps the tool caches)
	rm -rf site .cache/coverage
