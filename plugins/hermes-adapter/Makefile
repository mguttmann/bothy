# SPDX-License-Identifier: Apache-2.0
#
# Component targets of plugins/hermes-adapter (component contract: docs/spec/23-qualitaet.md,
# section 3). The root gates (`make fmt-check lint typecheck test coverage licenses`) run this
# project's ruff, mypy and pytest through PY_PROJECTS; `record` writes the fixtures of the live
# recording against the pinned Hermes Agent version (tests/hermes_pin.toml). It needs network for
# the pinned downloads and replaces tests/fixtures/recording only after a complete, clean run.

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := test
MAKEFLAGS += --no-builtin-rules --no-print-directory

UV ?= uv

.PHONY: test record

test: ## The component's default test run (unit, fixture and live recording tests)
	$(UV) run --locked pytest

record: ## Record again: run every scenario and rewrite tests/fixtures/recording
	$(UV) run --locked pytest tests/recording --record
