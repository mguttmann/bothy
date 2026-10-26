# SPDX-License-Identifier: Apache-2.0
#
# Component targets of the panel plugin bothy-live-panel (component contract of plugins/,
# docs/quality/gates.md). The root gates call the npm scripts of package.json for format, lint,
# types and tests (`make fmt-check lint typecheck test` with COMPONENTS=plugins); the root target
# `make signal-check` holds queries.yaml to the signal catalogue. Dependencies: `npm ci` in this
# directory (root `make bootstrap-ci` runs it).

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := check
MAKEFLAGS += --no-builtin-rules --no-print-directory

.PHONY: check test lint typecheck format-check generate generate-check build release-build stack-install

UV ?= uv

check: generate-check format-check lint typecheck test ## Every component target except build

test: ## Unit tests of the views, the generator, the colour lint and the repository ties, with coverage
	npm test

lint: ## ESLint, with the rule bothy/no-color-literal on src/
	npm run lint

typecheck: ## tsc --noEmit over src/, test/ and scripts/
	npm run typecheck

format-check: ## Prettier finds nothing to change
	npm run format:check

generate: ## Write src/queries.gen.ts from queries.yaml (never edit it by hand)
	npm run --silent generate

generate-check: ## src/queries.gen.ts equals a fresh generation from queries.yaml
	npm run --silent generate:check

build: ## Production bundle in dist/ (unsigned, ruling T11)
	npm run build

# Release ZIP of the component grafana-bothy-live (docs/spec/18-release.md section 2,
# docs/spec/25-visualization.md section 8.3): the production bundle, then
# tools/ci/plugin_release.py packs dist/ into bothy-live-panel-<version>.zip with the top folder
# bothy-live-panel/. release.yml signs it like every release file; the product release-build
# puts the same ZIP into the bundle. OUT is an absolute path or relative to this directory.
release-build: ## Release ZIP bothy-live-panel-<version>.zip (unsigned plugin): make release-build VERSION=<x.y.z> OUT=<dir>
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-build VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	@[ "$$(tr -d '[:space:]' < VERSION)" = "$$VERSION" ] || { echo "release-build: VERSION file says $$(tr -d '[:space:]' < VERSION), not $$VERSION"; exit 1; }
	npm run build
	$(UV) run --project ../../tools --locked --quiet python ../../tools/ci/plugin_release.py pack --dist dist --version "$$VERSION" --out "$$OUT"

# The plugin for the Compose stack of a checkout: the release ZIP of VERSION, unpacked into the
# folder compose.yaml mounts into Grafana (infra/observability/grafana/plugins/bothy-live-panel,
# ignored by git). An installation from the release bundle needs no such step.
stack-install: ## Release ZIP of VERSION unpacked into the plugin folder of the Compose stack (checkout only)
	@tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/bothy-plugin.XXXXXX") || exit 1; trap 'rm -rf "$$tmp"' EXIT; \
	version=$$(tr -d '[:space:]' < VERSION); \
	$(MAKE) release-build VERSION="$$version" OUT="$$tmp" || exit 1; \
	$(UV) run --project ../../tools --locked --quiet python ../../tools/ci/plugin_release.py unpack \
	  --zip "$$tmp/bothy-live-panel-$$version.zip" --version "$$version" \
	  --dest ../../infra/observability/grafana/plugins/bothy-live-panel
