# SPDX-License-Identifier: Apache-2.0
#
# TEST ONLY: the binaries of the host service container of phase 3 of make console-e2e (brief
# M6-18 Design note 5). tools/console-e2e/console_e2e.py builds it with the context services/host
# and exports the files with the local exporter; the job of the console E2E test has Docker but no
# Go. The builds are those of BuildHostBinaries of the terminal suite of M2-48
# (services/server/test/terminal/stack/host.go): bothy-host with the mark of the platform suite,
# because the host container is a disposable test machine (docs/DECISIONS.md D-M2-42-11 and
# D-M2-48-5), and the Fake-claude with its start log. Never part of a release; the Go image is the
# one of services/server/Dockerfile (a test holds both equal).
FROM --platform=$BUILDPLATFORM golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
# A transient error of the module proxy is retried (docs/DECISIONS.md PL/33).
RUN for attempt in 1 2 3; do \
      GOTOOLCHAIN=local go mod download && break; \
      [ "$attempt" = 3 ] && exit 1; \
      echo "go mod download failed (attempt $attempt of 3), retrying" >&2; sleep 10; \
    done
COPY . .
RUN CGO_ENABLED=0 GOTOOLCHAIN=local GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
        -ldflags "-X github.com/OWNER/bothy/services/host/internal/buildinfo.PlatformSuite=1" \
        -o /out/bothy-host ./cmd/bothy-host \
    && CGO_ENABLED=0 GOTOOLCHAIN=local GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
        -ldflags "-X main.startLog=/var/tmp/termsuite-claude-starts.log" \
        -o /out/claude ./testdata/fakeclaude

FROM scratch
COPY --from=build /out/ /
# The stage is only exported as files (--output type=local) and never runs; the user keeps the
# misconfiguration scan of every tracked Dockerfile clean (trivy DS-0002).
USER 65532:65532
