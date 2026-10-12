# SPDX-License-Identifier: Apache-2.0
#
# Generated types of the TypeScript SDK (component contract "Generierter Client":
# docs/spec/23-qualitaet.md section 3). The root target `make api-clients` (gate G4,
# docs/spec/06-api.md 15) calls `generate-check`. The generator openapi-typescript is pinned
# exactly in devDependencies of package.json and locked in package-lock.json
# (docs/DECISIONS.md M2-23/2); `make bootstrap-ci` installs it with `npm ci`. Format, lint,
# types and tests: root `make fmt-check`, `make lint`, `make typecheck` and `make test`.

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := generate-check
MAKEFLAGS += --no-builtin-rules --no-print-directory

# Overridable, so that the drift test can run on a copy of the spec and of this directory.
SPEC ?= ../../docs/api/openapi.yaml
TOOLS ?= ../../tools
UV ?= uv
OUT_DIR := src/generated
OUT := $(OUT_DIR)/openapi.ts
GENERATOR := node_modules/.bin/openapi-typescript

.PHONY: generate generate-check

generate: ## Regenerate $(OUT) from $(SPEC) with standard settings (never edit it by hand)
	@[ -x $(GENERATOR) ] || { echo "generate: $(GENERATOR) not found; run \`make bootstrap-ci\` in the repository root" >&2; exit 2; }
	@# The directory holds only generator output: emptying it first makes a checked-in file the
	@# generator does not write show up as drift (`removed:`, red team M2-23).
	rm -rf $(OUT_DIR)
	@mkdir -p $(OUT_DIR)
	@# The generator prints progress lines with the absolute spec path; only a failure is shown.
	@log=$$(mktemp "$${TMPDIR:-/tmp}/bothy-sdk-typescript-generate.XXXXXX") || exit 1; \
	if ! $(GENERATOR) "$(SPEC)" --output $(OUT) >"$$log" 2>&1; then cat "$$log" >&2; rm -f "$$log"; exit 1; fi; \
	rm -f "$$log"

generate-check: ## G4: a fresh generation equals the checked-in types, which compile with tsc --strict
	$(UV) run --project "$(TOOLS)" --locked --quiet python "$(TOOLS)/ci/codegen_check.py" --dir . -- $(MAKE) generate
	npm run --silent typecheck
