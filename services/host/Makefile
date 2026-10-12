# SPDX-License-Identifier: Apache-2.0
#
# Component Makefile of the host service bothy-host (docs/spec/23-qualitaet.md
# "Komponenten-Vertrag", docs/spec/18-release.md "Komponenten" row host). The gates run from
# the root Makefile; this file holds the release build and the regeneration of the recordings.

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
# GNU make 3.81 (macOS) ignores .SHELLFLAGS; the flags go into SHELL there, as in the root
# Makefile (docs/DECISIONS.md M1-01/8).
ifeq ($(filter oneshell,$(.FEATURES)),)
SHELL := bash -euo pipefail
endif
.DEFAULT_GOAL := help
MAKEFLAGS += --no-builtin-rules --no-print-directory

# The five platforms of docs/spec/18-release.md "Komponenten", row host.
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

.PHONY: help release-build recordings

help: ## Show the targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Unsigned binaries without cgo (docs/DECISIONS.md D-M2-38-6); M7-23 adds signature, SBOM,
# checksums and installation packages.
release-build: ## Unsigned bothy-host binaries for every platform: make release-build VERSION=<x.y.z> OUT=<dir>
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-build VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	@[ "$$(cat VERSION)" = "$$VERSION" ] || { echo "release-build: VERSION file says $$(cat VERSION), not $$VERSION"; exit 1; }
	@mkdir -p "$$OUT" || exit 1
	@for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; ext=""; [ "$$os" = windows ] && ext=.exe; \
	  out="$$OUT/bothy-host_$${VERSION}_$${os}_$${arch}$$ext"; \
	  echo "  build $$out"; \
	  CGO_ENABLED=0 GOTOOLCHAIN=local GOOS=$$os GOARCH=$$arch go build -trimpath -o "$$out" ./cmd/bothy-host || exit 1; \
	done

recordings: ## Rewrite testdata/noise/recordings/go.json and testdata/noise/fingerprint.json
	UPDATE_RECORDINGS=1 GOTOOLCHAIN=local go test -count=1 -run 'TestRecordingsCurrent|TestFingerprintFileCurrent' ./internal/noise ./internal/fingerprint
