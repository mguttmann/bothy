# SPDX-License-Identifier: Apache-2.0
#
# Generated core of the Python SDK (component contract "Generierter Client":
# docs/spec/23-qualitaet.md section 3). The root target `make api-clients` (gate G4,
# docs/spec/06-api.md 15) calls `generate-check`. The generator openapi-python-client is
# pinned in the dev group of pyproject.toml and locked in uv.lock (docs/DECISIONS.md
# M2-23/2). Lint, format, types and tests: root `make lint`, `make fmt-check`,
# `make typecheck` and `make test`.

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := generate-check
MAKEFLAGS += --no-builtin-rules --no-print-directory

# Overridable, so that the drift test can run on a copy of the spec and of this directory.
SPEC ?= ../../docs/api/openapi.yaml
TOOLS ?= ../../tools
UV ?= uv
OUT := src/bothy_sdk/generated

.PHONY: generate generate-check

generate: ## Regenerate the core in $(OUT) from $(SPEC) (never edit it by hand)
	@# The generator reports the responses it cannot type (06 14) on every run; its output is
	@# shown only when it fails. test_g4_every_operation_is_generated catches a skipped operation.
	@# The generator replaces only the files it writes; emptying the directory first makes a
	@# checked-in file it does not produce show up as drift (`removed:`, red team M2-23).
	rm -rf $(OUT)
	@log=$$(mktemp "$${TMPDIR:-/tmp}/bothy-sdk-python-generate.XXXXXX") || exit 1; \
	if ! $(UV) run --project . --locked --quiet openapi-python-client generate --path "$(SPEC)" \
	  --meta none --overwrite --config openapi-python-client.yaml --output-path $(OUT) >"$$log" 2>&1; then \
	  cat "$$log" >&2; rm -f "$$log"; exit 1; fi; \
	rm -f "$$log"
	@# The templates of the pinned generator write em dashes, which the text-rules hook rejects;
	@# a deterministic rewrite to a hyphen is the last step (docs/DECISIONS.md M2-23/5).
	find $(OUT) -name '*.py' -exec perl -CSD -pi -e 's/[\x{2013}\x{2014}]/-/g' {} +

generate-check: ## G4: a fresh generation equals the checked-in core, which compiles and type checks
	$(UV) run --project "$(TOOLS)" --locked --quiet python "$(TOOLS)/ci/codegen_check.py" --dir . -- $(MAKE) generate
	$(UV) run --project . --locked --quiet python -m compileall -q src/bothy_sdk
	$(UV) run --project . --locked --quiet mypy
