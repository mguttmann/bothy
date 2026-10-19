# SPDX-License-Identifier: Apache-2.0
#
# Image bothy-server (ADR 0012, docs/spec/18-release.md "Server-Images"). Build context is the
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

FROM --platform=$BUILDPLATFORM golang:1.27.2-trixie@sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d AS build
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
# Empty key store of secrets.signing_key_dir (18 "Server-Images" rule 3, ADR 0066) and empty root
# file.fs.root of the file system driver (M3-03): a named volume mounted there takes owner 65532
# and mode 0700 from the image on first use, which the start check of the driver requires.
RUN mkdir -p /out/signing-keys /out/files

# The web console (brief M6-02, docs/spec/18-release.md 7 rule 7): built with the pinned Flutter
# of the workspace (pubspec.yaml) on the build platform, because its output is platform
# independent; the final stage copies build/web to the path BOTHY_CONSOLE__ASSETS_DIR names.
# No Flutter image exists for the pinned version, so the stage clones the SDK at the tag and
# checks the revision; the SDK fetches its Dart SDK and web artifacts by the engine hash of that
# revision. The base is the pinned builder image of the server.
FROM --platform=$BUILDPLATFORM golang:1.27.2-trixie@sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d AS console
ARG FLUTTER_VERSION=3.47.7
ARG FLUTTER_REVISION=abaf9c523780a608bd46686fd5e53740a07077f8
RUN apt-get update -qq \
    && apt-get install -y -qq --no-install-recommends unzip xz-utils \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --quiet --depth 1 --branch "$FLUTTER_VERSION" https://github.com/flutter/flutter.git /opt/flutter \
    && test "$(git -C /opt/flutter rev-parse HEAD)" = "$FLUTTER_REVISION"
ENV PATH=/opt/flutter/bin:$PATH
# The build fetches only the web artifacts it needs, by the engine hash of the revision.
RUN flutter --disable-analytics >/dev/null
WORKDIR /src
# The root resolution of the pub workspace needs every member pubspec.yaml; the lock file is
# enforced, so the image builds exactly the packages the tests ran with.
COPY pubspec.yaml pubspec.lock analysis_options.yaml ./
COPY apps/client/pubspec.yaml apps/client/
COPY apps/console/pubspec.yaml apps/console/
COPY packages/admin_ui/pubspec.yaml packages/admin_ui/
COPY packages/api_client/pubspec.yaml packages/api_client/
COPY packages/auth/pubspec.yaml packages/auth/
COPY packages/bothy_noise/pubspec.yaml packages/bothy_noise/
COPY packages/bothy_viz/pubspec.yaml packages/bothy_viz/
COPY packages/bothy_viz/harness/pubspec.yaml packages/bothy_viz/harness/
COPY packages/design_system/pubspec.yaml packages/design_system/
COPY packages/device_key/pubspec.yaml packages/device_key/
COPY packages/domain/pubspec.yaml packages/domain/
COPY packages/l10n/pubspec.yaml packages/l10n/
COPY packages/local_store/pubspec.yaml packages/local_store/
COPY packages/sync_engine/pubspec.yaml packages/sync_engine/
COPY packages/terminal_ui/pubspec.yaml packages/terminal_ui/
RUN flutter pub get --enforce-lockfile >/dev/null
COPY packages/api_client/lib packages/api_client/lib
COPY packages/auth/lib packages/auth/lib
COPY packages/design_system/lib packages/design_system/lib
COPY packages/design_system/assets packages/design_system/assets
COPY packages/design_system/fonts packages/design_system/fonts
COPY packages/domain/lib packages/domain/lib
COPY packages/l10n/lib packages/l10n/lib
COPY apps/console ./apps/console
COPY docs/api/openapi.yaml docs/api/
COPY services/server/VERSION services/server/
WORKDIR /src/apps/console
RUN flutter build web --release --base-href /console/ --no-web-resources-cdn --no-pub --no-wasm-dry-run \
    && sh tool/version_json.sh ../../docs/api/openapi.yaml ../../services/server/VERSION build/web/version.json

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
# The console build, owned by root and read-only for the running user (console.assets_dir).
COPY --from=console --chown=0:0 --chmod=0555 /src/apps/console/build/web /usr/share/bothy/console
ENV BOTHY_CONSOLE__ASSETS_DIR=/usr/share/bothy/console
# Numeric, so runtimes that enforce "run as non-root" can verify it without /etc/passwd.
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/bothy"]
CMD ["serve"]
# Same command and timings as x-server-healthcheck in infra/compose/compose.yaml; the image has
# no shell and no curl. 8081 is the default of server.internal_listen_addr.
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=4 \
    CMD ["/usr/local/bin/bothy", "healthcheck", "--url", "http://127.0.0.1:8081/readyz"]
