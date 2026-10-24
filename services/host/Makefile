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

.PHONY: help release-build recordings platform-bundle platform-test

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


# The platform suite (brief M2-42 design note 19, brief M2-48 design note 11): only on a disposable
# machine with administrator rights, never on a workstation. The test binary itself refuses to run
# without the marker file /etc/termsuite-disposable (Windows %ProgramData%\termsuite-disposable,
# test/platform/guard.go); platform-test also wants BOTHY_PLATFORM_DISPOSABLE=1 before it builds.
MODULE := github.com/OWNER/bothy/services/host
PLATFORM_OUT ?= $(CURDIR)/.platform

# run.sh of a platform bundle: the environment platform-test sets, for a machine without Go and
# without the checkout. @GOOS@ and @COMMIT@ are filled in when the bundle is built. "$${@+"$$@"}"
# keeps set -u quiet on the bash 3.2 of macOS.
define PLATFORM_RUN_SH
#!/usr/bin/env bash
# Runs the platform bundle in this folder (make -C services/host platform-bundle, brief M2-48
# design note 11) with the environment make platform-test sets. Only on a disposable test machine,
# as root or Administrator for suite: the suite creates and deletes users, services and ACLs.
#   run.sh [suite|units] [test binary flags]
#   suite  TestSessionCgroup (Linux) and the platform suite; the result file goes to
#          BOTHY_PLATFORM_RESULT or result-<goos>.json in this folder
#   units  the package tests of internal/pty, internal/session, internal/profiles (not on
#          Windows) and internal/confirm, as the user who runs this script
set -euo pipefail
mode="$${1:-suite}"
[ $$# -gt 0 ] && shift
here="$$(cd "$$(dirname "$$0")" && { pwd -W 2>/dev/null || pwd; })"
goos=@GOOS@
ext=""
[ "$$goos" = windows ] && ext=.exe
bin="$$here/bin"
case "$$mode" in
suite)
  if [ "$$goos" = windows ]; then marker="$${ProgramData:-}\\termsuite-disposable"; else marker=/etc/termsuite-disposable; fi
  [ -f "$$marker" ] || { echo "platform: not a disposable test machine (marker file missing)" >&2; exit 2; }
  # Linux: the per-session cgroup needs root and a cgroup it may write (M2-45 TestSessionCgroup).
  if [ "$$goos" = linux ]; then "$$bin/pty.test" -test.count=1 -test.timeout=10m -test.run '^TestSessionCgroup$$'; fi
  export BOTHY_HOST_BIN="$$bin/bothy-host$$ext" BOTHY_FAKECLAUDE="$$bin/claude$$ext" BOTHY_FAKENODE="$$bin/node$$ext"
  export BOTHY_SUITE_FAKECLAUDE="$$bin/suite-claude$$ext" BOTHY_BIGHOME="$$bin/bighome$$ext"
  export BOTHY_PLATFORM_RESULT="$${BOTHY_PLATFORM_RESULT:-$$here/result-$$goos.json}" BOTHY_COMMIT=@COMMIT@
  exec "$$bin/platform.test$$ext" -test.count=1 -test.timeout=60m $${@+"$$@"}
  ;;
units)
  # TestSessionCgroup needs root (suite); TestNoInboundNetworkListener reads the source tree,
  # which a bundle does not carry (make check runs it). internal/profiles simulates Unix paths.
  rc=0
  "$$bin/pty.test$$ext" -test.count=1 -test.timeout=10m -test.skip '^TestSessionCgroup$$' $${@+"$$@"} || rc=1
  "$$bin/session.test$$ext" -test.count=1 -test.timeout=10m -test.skip '^TestNoInboundNetworkListener$$' $${@+"$$@"} || rc=1
  if [ "$$goos" != windows ]; then "$$bin/profiles.test$$ext" -test.count=1 -test.timeout=10m $${@+"$$@"} || rc=1; fi
  "$$bin/confirm.test$$ext" -test.count=1 -test.timeout=10m $${@+"$$@"} || rc=1
  exit "$$rc"
  ;;
*)
  echo "usage: run.sh [suite|units] [test binary flags]" >&2
  exit 2
  ;;
esac
endef

# run.ps1 of a Windows bundle: the same as run.sh for a Windows machine without bash.
define PLATFORM_RUN_PS1
# Runs the platform bundle in this folder on Windows without bash, as run.sh does (make -C
# services/host platform-bundle, brief M2-48 design note 11). Only on a disposable test machine,
# elevated for suite:
#   powershell -NoProfile -ExecutionPolicy Bypass -File run.ps1 [suite|units] [test binary flags]
$$mode = if ($$args.Count -gt 0) { $$args[0] } else { 'suite' }
$$rest = @($$args | Select-Object -Skip 1)
$$bin = Join-Path $$PSScriptRoot 'bin'
switch ($$mode) {
  'suite' {
    if (-not $$env:ProgramData -or -not (Test-Path -LiteralPath (Join-Path $$env:ProgramData 'termsuite-disposable') -PathType Leaf)) {
      [Console]::Error.WriteLine('platform: not a disposable test machine (marker file missing)')
      exit 2
    }
    $$env:BOTHY_HOST_BIN = Join-Path $$bin 'bothy-host.exe'
    $$env:BOTHY_FAKECLAUDE = Join-Path $$bin 'claude.exe'
    $$env:BOTHY_FAKENODE = Join-Path $$bin 'node.exe'
    $$env:BOTHY_SUITE_FAKECLAUDE = Join-Path $$bin 'suite-claude.exe'
    $$env:BOTHY_BIGHOME = Join-Path $$bin 'bighome.exe'
    if (-not $$env:BOTHY_PLATFORM_RESULT) { $$env:BOTHY_PLATFORM_RESULT = Join-Path $$PSScriptRoot 'result-windows.json' }
    $$env:BOTHY_COMMIT = '@COMMIT@'
    & (Join-Path $$bin 'platform.test.exe') '-test.count=1' '-test.timeout=60m' @rest
    exit $$LASTEXITCODE
  }
  'units' {
    $$rc = 0
    & (Join-Path $$bin 'pty.test.exe') '-test.count=1' '-test.timeout=10m' '-test.skip=^TestSessionCgroup$$' @rest
    if ($$LASTEXITCODE -ne 0) { $$rc = 1 }
    & (Join-Path $$bin 'session.test.exe') '-test.count=1' '-test.timeout=10m' '-test.skip=^TestNoInboundNetworkListener$$' @rest
    if ($$LASTEXITCODE -ne 0) { $$rc = 1 }
    & (Join-Path $$bin 'confirm.test.exe') '-test.count=1' '-test.timeout=10m' @rest
    if ($$LASTEXITCODE -ne 0) { $$rc = 1 }
    exit $$rc
  }
  default {
    [Console]::Error.WriteLine('usage: run.ps1 [suite|units] [test binary flags]')
    exit 2
  }
}
endef
export PLATFORM_RUN_SH PLATFORM_RUN_PS1

# The bundle holds every file a platform run needs, built as platform-test built it before
# M2-48: bothy-host without cgo with the PlatformSuite mark, the fake claude and its node build,
# the fake the suite installs for itself (suite-claude), the generator bighome, the test binaries
# of the suite and of M2-45's packages, run.sh (run.ps1 on Windows) and bundle.json with the
# SHA-256 of each file. The fake programs log to a path linked into them: the folder of the
# bundle for platform-test (PLATFORM_LOGDIR), otherwise the fixed folder of the suite
# (/opt/bothy-platform, Windows %ProgramFiles%\bothy-platform), which exists on every test machine
# whatever folder the bundle is copied to. No codesign here: bothy-host install signs the binary
# with the hardened runtime on macOS (internal/install), so a bundle may be cross-built.
platform-bundle: ## Build, without running, the platform suite of one platform: make platform-bundle OUT=<dir> [GOOS=<os> GOARCH=<arch>]
	@[ -n "$${OUT:-}" ] || { echo "usage: make platform-bundle OUT=<dir> [GOOS=<os> GOARCH=<arch>]"; exit 2; }
	@export GOTOOLCHAIN=local; \
	  goos="$$(go env GOOS)"; goarch="$$(go env GOARCH)"; ext=""; \
	  if [ "$$goos" = windows ]; then ext=.exe; sep='\'; root="$${ProgramFiles:-C:\Program Files}\bothy-platform"; \
	  else sep=/; root=/opt/bothy-platform; fi; \
	  if [ -n "$${PLATFORM_LOGDIR:-}" ]; then startlog="$$PLATFORM_LOGDIR/claude-starts.log"; else startlog="$$root$${sep}claude-starts.log"; fi; \
	  suitelog="$$root$${sep}fake$${sep}claude-starts.log"; \
	  mkdir -p "$$OUT/bin"; out="$$(cd "$$OUT" && pwd)"; \
	  commit="$$(git rev-parse HEAD)"; gover="$$(go env GOVERSION)"; \
	  echo "  platform bundle $$goos/$$goarch in $$out"; \
	  CGO_ENABLED=0 go build -trimpath -ldflags "-X $(MODULE)/internal/buildinfo.PlatformSuite=1" -o "$$out/bin/bothy-host$$ext" ./cmd/bothy-host; \
	  go build -ldflags "-X 'main.startLog=$$startlog'" -o "$$out/bin/claude$$ext" ./testdata/fakeclaude; \
	  go build -ldflags "-X 'main.startLog=$$startlog'" -o "$$out/bin/node$$ext" ./testdata/fakeclaude; \
	  go build -ldflags "-X 'main.startLog=$$suitelog'" -o "$$out/bin/suite-claude$$ext" ./testdata/fakeclaude; \
	  go build -tags platform -o "$$out/bin/bighome$$ext" ./test/platform/bighome; \
	  go test -c -tags platform -o "$$out/bin/platform.test$$ext" ./test/platform; \
	  go test -c -tags platform -o "$$out/bin/pty.test$$ext" ./internal/pty; \
	  for p in session profiles confirm; do go test -c -o "$$out/bin/$$p.test$$ext" "./internal/$$p"; done; \
	  files=""; for b in bothy-host claude node suite-claude bighome platform.test pty.test session.test profiles.test confirm.test; do \
	    files="$$files bin/$$b$$ext"; done; \
	  printf '%s\n' "$$PLATFORM_RUN_SH" | sed -e "s|@GOOS@|$$goos|" -e "s|@COMMIT@|$$commit|" > "$$out/run.sh"; \
	  chmod 755 "$$out/run.sh"; files="$$files run.sh"; \
	  if [ "$$goos" = windows ]; then \
	    printf '%s\n' "$$PLATFORM_RUN_PS1" | sed -e "s|@COMMIT@|$$commit|" > "$$out/run.ps1"; files="$$files run.ps1"; fi; \
	  sum() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$$1"; else shasum -a 256 "$$1"; fi | cut -d' ' -f1; }; \
	  { printf '{\n  "format": 1,\n  "commit": "%s",\n  "go_version": "%s",\n  "goos": "%s",\n  "goarch": "%s",\n  "files": [' \
	      "$$commit" "$$gover" "$$goos" "$$goarch"; \
	    comma=""; for f in $$files; do printf '%s\n    {"path": "%s", "sha256": "%s"}' "$$comma" "$$f" "$$(sum "$$out/$$f")"; comma=","; done; \
	    printf '\n  ]\n}\n'; } > "$$out/bundle.json"

# platform-test builds the bundle of the build host into PLATFORM_OUT and runs it, so a run here
# and a run of a copied bundle execute the same files.
platform-test: ## Platform suite on a disposable machine: make platform-test (needs administrator rights)
	@[ "$${BOTHY_PLATFORM_DISPOSABLE:-}" = 1 ] || { echo "platform-test: set BOTHY_PLATFORM_DISPOSABLE=1 only on a disposable machine"; exit 2; }
	@$(MAKE) platform-bundle OUT="$(PLATFORM_OUT)" PLATFORM_LOGDIR="$(PLATFORM_OUT)"
	@bash "$(PLATFORM_OUT)/run.sh" suite
