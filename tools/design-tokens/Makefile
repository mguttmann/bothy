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

.PHONY: check test gate

check: test gate ## Unit and gate tests, then the token gate on the repository state

test: ## Unit tests and end-to-end tests of the gate (tests/)
	$(PYTHON) -W error -m unittest discover -s tests -t .

gate: ## Structure, contrast, palette guards, generated files current, vendor/ matches MANIFEST.json
	$(PYTHON) tokens.py check
