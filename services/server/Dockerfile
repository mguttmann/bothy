# SPDX-License-Identifier: Apache-2.0
#
# Image bothy-server (ADR 0012, docs/spec/18-release.md "Server images"). Build context is the
# repository root; services/server/Dockerfile.dockerignore keeps it to the server sources:
#
#   docker buildx build --file services/server/Dockerfile \
#     --build-arg VERSION="$(cat services/server/VERSION)" --build-arg REVISION="$(git rev-parse HEAD)" .
#
# `make image` and the release workflow call it this way. The builder runs on the build platform
# and cross compiles for the target platform; the final stage only copies, so no stage runs
# under emulation. Base images are pinned by digest (SEC-SC-4, `make lint`); Renovate updates
# the digests, and the golang tag follows the toolchain of services/server/go.mod (a test in
# tools/ci/tests/test_dockerfile_pins.py holds both).

FROM --platform=$BUILDPLATFORM golang:1.27.3-trixie@sha256:1896654cfc5c2b5ad74c07f29cb36f5d2d1cbed42e3054070fbfe444e521e83c AS build
ARG TARGETOS TARGETARCH
WORKDIR /src/services/server
COPY services/server/go.mod services/server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    GOTOOLCHAIN=local GOFLAGS=-mod=readonly go mod download
# Declared after the download, so a new version does not discard the module cache layer.
ARG VERSION
COPY services/server/VERSION ./
# Labels and binary must never disagree: the binary embeds VERSION through buildinfo (a test
# holds the constant equal to the file), the labels take the build argument.
RUN want="$(tr -d '[:space:]' < VERSION)"; \
    if [ -z "$VERSION" ]; then echo "build argument VERSION is required (services/server/VERSION says $want)" >&2; exit 1; fi; \
    if [ "$VERSION" != "$want" ]; then echo "build argument VERSION=$VERSION differs from services/server/VERSION ($want)" >&2; exit 1; fi
COPY services/server/api ./api
COPY services/server/cmd ./cmd
COPY services/server/internal ./internal
COPY services/server/migrations ./migrations
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOTOOLCHAIN=local GOFLAGS=-mod=readonly \
    go build -trimpath -buildvcs=false -ldflags "-s -w" -o /out/bothy ./cmd/bothy
# Empty key store of secrets.signing_key_dir (18 "Server images" rule 3, ADR 0066) and empty root
# file.fs.root of the file system driver (M3-03): a named volume mounted there takes owner 65532
# and mode 0700 from the image on first use, which the start check of the driver requires.
RUN mkdir -p /out/signing-keys /out/files

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
ARG VERSION REVISION
ARG SOURCE_URL=https://github.com/OWNER/bothy
LABEL org.opencontainers.image.title="bothy-server" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.licenses="Apache-2.0"
# Owned by root and not writable, so the running user cannot replace the binary.
COPY --from=build --chown=0:0 --chmod=0555 /out/bothy /usr/local/bin/bothy
COPY --from=build --chown=65532:65532 --chmod=0700 /out/signing-keys /var/lib/bothy/signing-keys
COPY --from=build --chown=65532:65532 --chmod=0700 /out/files /var/lib/bothy/files
# Numeric, so runtimes that enforce "run as non-root" can verify it without /etc/passwd.
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/bothy"]
CMD ["serve"]
# Same command and timings as x-server-healthcheck in infra/compose/compose.yaml; the image has
# no shell and no curl. 8081 is the default of server.internal_listen_addr.
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=4 \
    CMD ["/usr/local/bin/bothy", "healthcheck", "--url", "http://127.0.0.1:8081/readyz"]
