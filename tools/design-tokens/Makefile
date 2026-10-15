# SPDX-License-Identifier: Apache-2.0
#
# Gate of the design-token generator (component contract: docs/spec/23-qualitaet.md,
# section 3). The root target `make tokens` calls `check`. The generator uses only the
# Python standard library; it runs in the pinned tool environment of the repository
# (tools/pyproject.toml), like every other gate. Lint and format: root `make lint` and
# `make fmt-check` (ruff reads ruff.toml in this folder).

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := check
MAKEFLAGS += --no-builtin-rules --no-print-directory

UV ?= uv
PYTHON := $(UV) run --project ../../tools --locked --quiet python
# The TypeScript of sdk/typescript/package-lock.json (installed by `make bootstrap-ci`);
# docs/DECISIONS.md D-M7-24-2.
TSC := ../../sdk/typescript/node_modules/.bin/tsc
TSC_FLAGS := --strict --noEmit --target es2022 --module es2022 --lib es2022

.PHONY: check test gate ts

check: test gate ts ## Unit and gate tests, the token gate on the repository state, the TypeScript compile check

test: ## Unit tests and end-to-end tests of the gate (tests/)
	$(PYTHON) -W error -m unittest discover -s tests -t .

gate: ## Structure, contrast, palette guards, generated files current, vendor/ matches MANIFEST.json
	$(PYTHON) tokens.py check

ts: ## tsc --strict --noEmit over design/build/ts/tokens.gen.ts (flags as emit_ts.TSC_FLAGS)
	@[ -x $(TSC) ] || { echo "ts: $(TSC) not found; run \`make bootstrap\`" >&2; exit 2; }
	$(TSC) $(TSC_FLAGS) ../../design/build/ts/tokens.gen.ts
