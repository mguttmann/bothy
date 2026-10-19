# SPDX-License-Identifier: Apache-2.0
#
# Component Makefile of the host service bothy-host (docs/spec/23-quality.md
# "Component contract", docs/spec/18-release.md "Components" row host). The gates run from
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

# The five platforms of docs/spec/18-release.md "Components", row host.
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

.PHONY: help release-build recordings platform-test

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
	  CGO_ENABLED=0 GOTOOLCHAIN=local GOOS=$$os GOARCH=$$arch go build -trimpath \
	    -ldflags "-X github.com/OWNER/bothy/services/host/internal/buildinfo.Version=$$VERSION" -o "$$out" ./cmd/bothy-host || exit 1; \
	done

recordings: ## Rewrite testdata/noise/recordings/go.json, testdata/noise/fingerprint.json and testdata/control/host_frames.json
	UPDATE_RECORDINGS=1 GOTOOLCHAIN=local go test -count=1 -run 'TestRecordingsCurrent|TestFingerprintFileCurrent|TestHostFramesRecordingCurrent' ./internal/noise ./internal/fingerprint ./internal/control

# The platform suite (brief M2-42 design note 19): only on a disposable machine with administrator
# rights (a GitHub-hosted runner of the workflow host-platform.yml), never on a workstation. The
# build marks itself with buildinfo.PlatformSuite, so the peer check is measured, not assumed.
PLATFORM_OUT ?= $(CURDIR)/.platform
platform-test: ## Platform suite on a disposable machine: make platform-test (needs administrator rights)
	@[ "$${BOTHY_PLATFORM_DISPOSABLE:-}" = 1 ] || { echo "platform-test: set BOTHY_PLATFORM_DISPOSABLE=1 only on a disposable machine"; exit 2; }
	@mkdir -p "$(PLATFORM_OUT)"
	@ext=""; [ "$$(go env GOOS)" = windows ] && ext=.exe; \
	  CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -ldflags "-X github.com/OWNER/bothy/services/host/internal/buildinfo.PlatformSuite=1" \
	    -o "$(PLATFORM_OUT)/bothy-host$$ext" ./cmd/bothy-host && \
	  GOTOOLCHAIN=local go build -ldflags "-X main.startLog=$(PLATFORM_OUT)/claude-starts.log" -o "$(PLATFORM_OUT)/claude$$ext" ./testdata/fakeclaude && \
	  GOTOOLCHAIN=local go build -ldflags "-X main.startLog=$(PLATFORM_OUT)/claude-starts.log" -o "$(PLATFORM_OUT)/node$$ext" ./testdata/fakeclaude && \
	  BOTHY_HOST_BIN="$(PLATFORM_OUT)/bothy-host$$ext" BOTHY_FAKECLAUDE="$(PLATFORM_OUT)/claude$$ext" BOTHY_FAKENODE="$(PLATFORM_OUT)/node$$ext" \
	  BOTHY_PLATFORM_RESULT="$(PLATFORM_OUT)/result-$$(go env GOOS).json" BOTHY_COMMIT="$$(git rev-parse HEAD)" \
	  GOTOOLCHAIN=local go test -tags platform -count=1 -timeout 60m ./test/platform/...
