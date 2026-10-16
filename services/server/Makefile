# SPDX-License-Identifier: Apache-2.0
#
# Component Makefile of the server (docs/spec/23-quality.md "Component contract",
# docs/spec/18-release.md "Components" row server). The gates run from the root Makefile, and so
# do the image builds (`make image`, `make postgres-image`); this file holds the release build of
# the binary `bothy` that release.yml and `make release-trial` call.

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
# GNU make 3.81 (macOS) ignores .SHELLFLAGS; the flags go into SHELL there, as in the root
# Makefile (docs/DECISIONS.md M1-01/8).
ifeq ($(filter oneshell,$(.FEATURES)),)
SHELL := bash -euo pipefail
endif
.DEFAULT_GOAL := help
MAKEFLAGS += --no-builtin-rules --no-print-directory

# The platforms of the server images (docs/spec/18-release.md "Server images" rule 1).
PLATFORMS := linux/amd64 linux/arm64

.PHONY: help release-build

help: ## Show the targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Static binaries without cgo, built like the binary in services/server/Dockerfile (same flags),
# so the release binary and the one in the image come from the same build rule. release.yml
# signs them and adds SBOMs and checksums (docs/spec/18-release.md section 6).
release-build: ## Server binaries (cmd/bothy) for linux/amd64 and linux/arm64: make release-build VERSION=<x.y.z> OUT=<dir>
	@[ -n "$${OUT:-}" ] && [ -n "$${VERSION:-}" ] || { echo "usage: make release-build VERSION=<x.y.z> OUT=<dir>"; exit 2; }
	@[ "$$(tr -d '[:space:]' < VERSION)" = "$$VERSION" ] || { echo "release-build: VERSION file says $$(tr -d '[:space:]' < VERSION), not $$VERSION"; exit 1; }
	@mkdir -p "$$OUT" || exit 1
	@for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; out="$$OUT/bothy_$${VERSION}_$${os}_$${arch}"; \
	  echo "  build $$out"; \
	  CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly GOOS=$$os GOARCH=$$arch \
	    go build -trimpath -buildvcs=false -ldflags "-s -w" -o "$$out" ./cmd/bothy || exit 1; \
	done
