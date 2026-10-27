# SPDX-License-Identifier: Apache-2.0
#
# Component Makefile of the client app (docs/spec/23-quality.md "Component contract",
# docs/spec/18-release.md 8). The gates run from the root Makefile; this file holds the release
# builds of the desktop apps (brief M7-12) and the phone apps (brief M7-13) with the publisher
# values of DISTRIBUTION_FILE (ADR 0037) and their trials with a test publisher (D-W-127). The
# steps are tools/distribution/apple.py, windows.py, ios.py and android.py;
# apps/distribution/README.md explains the values.

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

.PHONY: help release-build release-apple release-windows release-ios release-android \
  trial-apple trial-windows trial-ios trial-android

help: ## Show the targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# release.yml runs `release-build` of every component on Linux; the client's artifacts come from
# the jobs build-apple and build-windows on their own runners, so there is nothing to build here.
release-build: ## Nothing on Linux: the app builds are release-apple, release-windows, release-ios and release-android
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-build VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	@echo "release-build: the client is built by release-apple, release-windows, release-ios and release-android"

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

release-ios: ## iOS app, Apple Distribution signature, IPA uploaded to TestFlight (group beta): make release-ios VERSION=<x.y.z> OUT=<dir>
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-ios VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	$(UVRUN) $(ROOT)/tools/distribution/ios.py release --version "$$VERSION" --out "$(abspath $(OUT))" \
	  --distribution "$(DISTRIBUTION_FILE)" --branding "$(BRANDING_FILE)"

release-android: ## Android App Bundle, signed with the upload key, on the Play internal track: make release-android VERSION=<x.y.z> OUT=<dir>
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-android VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	$(UVRUN) $(ROOT)/tools/distribution/android.py release --version "$$VERSION" --out "$(abspath $(OUT))" \
	  --distribution "$(DISTRIBUTION_FILE)" --branding "$(BRANDING_FILE)"

trial-ios: ## The unsigned iOS archive of a test publisher, its structure and entitlements checked (macOS with Xcode)
	$(UVRUN) $(ROOT)/tools/distribution/ios.py trial --out "$(abspath $(or $(OUT),build/trial-ios))" --branding "$(BRANDING_FILE)"

trial-android: ## The App Bundle signed with a throwaway upload key, validated and verified (Android SDK, Java 17)
	$(UVRUN) $(ROOT)/tools/distribution/android.py trial --out "$(abspath $(or $(OUT),build/trial-android))" --branding "$(BRANDING_FILE)"
