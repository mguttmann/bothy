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

.PHONY: check test lint typecheck format-check generate generate-check build

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

build: ## Production bundle in dist/ (unsigned; the release ZIP is M7-26)
	npm run build
