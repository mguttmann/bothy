# SPDX-License-Identifier: Apache-2.0
#
# Component Makefile of the client app (docs/spec/23-quality.md "Component contract",
# docs/spec/18-release.md 8). The gates run from the root Makefile; this file holds the release
# builds of the desktop app with the publisher values of DISTRIBUTION_FILE (brief M7-12,
# ADR 0037) and their trials with a test publisher (D-W-127). The steps are
# tools/distribution/apple.py and windows.py; apps/distribution/README.md explains the values.

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
# GNU make 3.81 (macOS) ignores .SHELLFLAGS; the flags go into SHELL there, as in the root
# Makefile (docs/DECISIONS.md M1-01/8).
ifeq ($(filter oneshell,$(.FEATURES)),)
SHELL := bash -euo pipefail
endif
.DEFAULT_GOAL := help
MAKEFLAGS += --no-builtin-rules --no-print-directory

ROOT := $(abspath ../..)
UVRUN := uv run --project $(ROOT)/tools --locked --quiet python
# The publisher values (an instance keeps its own copy in its ops repository) and the build
# branding (name, URL scheme, default server; the bundle identifier comes from the
# distribution).
DISTRIBUTION_FILE ?= $(ROOT)/apps/distribution/distribution.yaml
BRANDING_FILE ?= $(abspath branding.example.yaml)

.PHONY: help release-build release-apple release-windows trial-apple trial-windows

help: ## Show the targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# release.yml runs `release-build` of every component on Linux; the client's artifacts come from
# the jobs build-apple and build-windows on their own runners, so there is nothing to build here.
release-build: ## Nothing on Linux: the desktop builds are release-apple and release-windows
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-build VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	@echo "release-build: the client is built by release-apple (macOS) and release-windows (Windows)"

release-apple: ## macOS app, Developer ID, notarized, Sparkle archive and appcast: make release-apple VERSION=<x.y.z> OUT=<dir> [APPCAST=<published appcast>]
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-apple VERSION=<x.y.z> OUT=<dir> [APPCAST=<file>]"; exit 2; }
	$(UVRUN) $(ROOT)/tools/distribution/apple.py release --version "$$VERSION" --out "$(abspath $(OUT))" \
	  --distribution "$(DISTRIBUTION_FILE)" --branding "$(BRANDING_FILE)" $(if $(APPCAST),--appcast "$(abspath $(APPCAST))")

release-windows: ## Windows MSIX, signed, with beta.appinstaller: make release-windows VERSION=<x.y.z> OUT=<dir>
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-windows VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	$(UVRUN) $(ROOT)/tools/distribution/windows.py release --version "$$VERSION" --out "$(abspath $(OUT))" \
	  --distribution "$(DISTRIBUTION_FILE)" --branding "$(BRANDING_FILE)"

trial-apple: ## The macOS chain with a test publisher and the Sparkle update test (macOS with Xcode; sudo for the trust of the test certificate)
	$(UVRUN) $(ROOT)/tools/distribution/apple.py trial --out "$(abspath $(or $(OUT),build/trial-apple))" --branding "$(BRANDING_FILE)"

trial-windows: ## The Windows chain with a test publisher and the App Installer update test (Windows, administrator)
	$(UVRUN) $(ROOT)/tools/distribution/windows.py trial --out "$(abspath $(or $(OUT),build/trial-windows))" --branding "$(BRANDING_FILE)"
