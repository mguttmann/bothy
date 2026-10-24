# SPDX-License-Identifier: Apache-2.0
#
# Image bothy-postgres (docs/spec/16-operations.md 2, docs/spec/18-release.md "Server images" rules
# 2 and 4, briefs M7-01 and M7-02): the official postgres:18 plus pgBackRest, restic and
# bothy-backupd. One image
# serves the Compose services postgres and backup; entrypoint.sh picks the role. Build context is
# the repository root; Dockerfile.dockerignore next to this file keeps it to what the image needs:
#
#   docker buildx build --file services/server/images/bothy-postgres/Dockerfile \
#     --build-arg VERSION="$(cat services/server/VERSION)" --build-arg REVISION="$(git rev-parse HEAD)" .
#
# `make postgres-image` calls it this way. No step runs on the target platform, so both platforms
# build natively on any host, without emulation (18 "Server images" rule 2, docs/DECISIONS.md
# D-M7-01-2): the Go builder cross compiles bothy-backupd, the package stage downloads the
# packages of the target architecture on the build platform and unpacks them, and the final
# stage only copies. Base images are pinned by digest (SEC-SC-4, `make lint`); Renovate updates
# the digests, and the golang tag follows the toolchain of services/server/go.mod.

FROM --platform=$BUILDPLATFORM golang:1.27.3-trixie@sha256:1896654cfc5c2b5ad74c07f29cb36f5d2d1cbed42e3054070fbfe444e521e83c AS build
ARG TARGETOS TARGETARCH
WORKDIR /src/services/server
COPY services/server/go.mod services/server/go.sum ./
# A transient error of the module proxy is retried (docs/DECISIONS.md PL/33).
RUN --mount=type=cache,target=/go/pkg/mod \
    for attempt in 1 2 3; do \
      GOTOOLCHAIN=local GOFLAGS=-mod=readonly go mod download && break; \
      [ "$attempt" = 3 ] && exit 1; \
      echo "go mod download failed (attempt $attempt of 3), retrying" >&2; sleep 10; \
    done
ARG VERSION
COPY services/server/VERSION ./
# Labels and binary must never disagree: bothy-backupd reports VERSION through buildinfo.
RUN want="$(tr -d '[:space:]' < VERSION)"; \
    if [ -z "$VERSION" ]; then echo "build argument VERSION is required (services/server/VERSION says $want)" >&2; exit 1; fi; \
    if [ "$VERSION" != "$want" ]; then echo "build argument VERSION=$VERSION differs from services/server/VERSION ($want)" >&2; exit 1; fi
COPY services/server/cmd/bothy-backupd ./cmd/bothy-backupd
COPY services/server/internal ./internal
# bothy-backupd reads the configuration schema of the server (internal/wire), which embeds the
# API routes and the migrations.
COPY services/server/api ./api
COPY services/server/migrations ./migrations
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOTOOLCHAIN=local GOFLAGS=-mod=readonly \
    go build -trimpath -buildvcs=false -ldflags "-s -w" -o /out/bothy-backupd ./cmd/bothy-backupd

# The packages come from the PostgreSQL project's Debian repository for the Debian release of the
# base image (trixie-pgdg), which the base image already trusts; apt verifies every index against
# the repository key, and this stage checks that key by its fingerprint first. The pgBackRest
# version is exact; libssh2 is its one dependency the base image lacks and comes from the signed
# Debian archive of the same release (docs/DECISIONS.md D-M7-01-2).
FROM --platform=$BUILDPLATFORM postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336 AS packages
ARG TARGETARCH
# renovate: datasource=deb depName=pgbackrest registryUrl=https://apt.postgresql.org/pub/repos/apt?suite=trixie-pgdg&components=main&binaryArch=amd64
ARG PGBACKREST_VERSION=2.59.3-1.pgdg13+1
# Fingerprint of the signing key of apt.postgresql.org (https://www.postgresql.org/download/linux/debian/).
ARG PGDG_FINGERPRINT=B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8
COPY --chmod=0555 services/server/images/bothy-postgres/check-pgbackrest-version.sh /usr/local/bin/
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
# DS-0017 expects "apt-get install" after "apt-get update"; this stage only downloads the packages of
# the target architecture with "apt-get download" and never installs into a final image layer.
# trivy:ignore:DS-0017
RUN set -eu; \
    keyring=/usr/local/share/keyrings/postgres.gpg.asc; \
    grep -q "signed-by=$keyring" /etc/apt/sources.list.d/pgdg.list \
      || { echo "the PGDG source of the base image no longer uses $keyring" >&2; exit 1; }; \
    primaries=$(gpg --show-keys --with-colons "$keyring" | awk -F: '$1 == "pub" { want = 1; next } want && $1 == "fpr" { print $10; want = 0 }'); \
    [ "$primaries" = "$PGDG_FINGERPRINT" ] \
      || { echo "PGDG keyring holds $primaries, want exactly $PGDG_FINGERPRINT" >&2; exit 1; }; \
    check-pgbackrest-version.sh --reported "pgBackRest ${PGBACKREST_VERSION%%-*}"; \
    dpkg --add-architecture "$TARGETARCH"; \
    apt-get update -qq; \
    mkdir /debs; cd /debs; \
    apt-get download -qq "pgbackrest:$TARGETARCH=$PGBACKREST_VERSION" "libssh2-1t64:$TARGETARCH"; \
    for deb in ./*.deb; do \
      [ "$(dpkg-deb --field "$deb" Architecture)" = "$TARGETARCH" ] || { echo "$deb is not for $TARGETARCH" >&2; exit 1; }; \
      dpkg-deb --extract "$deb" /rootfs; \
      # The package database of distroless-style images, so image scanners see both packages.
      mkdir -p /rootfs/var/lib/dpkg/status.d; \
      { dpkg-deb --field "$deb"; echo "Status: install ok installed"; } > "/rootfs/var/lib/dpkg/status.d/$(dpkg-deb --field "$deb" Package)"; \
    done; \
    [ "$(dpkg-deb --field pgbackrest_*.deb Version)" = "$PGBACKREST_VERSION" ]; \
    # Only /usr is copied (plus the package records): /lib and /bin are symbolic links in the base
    # image, so a package file below them would be lost; /etc holds only a sample configuration.
    extra=$(find /rootfs -mindepth 1 -maxdepth 1 ! -name usr ! -name etc ! -name var); \
    [ -z "$extra" ] || { echo "unexpected top-level paths in the packages: $extra" >&2; exit 1; }; \
    rm -rf /rootfs/usr/share/doc /rootfs/usr/share/man /rootfs/usr/share/lintian; \
    # Directories of pgBackRest owned by the user postgres of the base image (uid and gid 999): the
    # repository and the log. A named volume mounted on /var/lib/pgbackrest takes owner and mode
    # from here on first use. The configuration the entry point writes lives in /tmp, and
    # /etc/pgbackrest is a link owned by root to it, so no copied path below /etc is writable by
    # the user (docs/DECISIONS.md D-M7-01-9).
    # bothy-backupd writes its scratch files (the pg_dump file of a logical run) below
    # /var/lib/bothy-backup, the volume backup-scratch (brief M7-02).
    mkdir -p /owned/var/lib/pgbackrest /owned/var/log/pgbackrest /owned/var/lib/bothy-backup /link/etc; \
    ln -s /tmp/bothy-pgbackrest /link/etc/pgbackrest

# restic (BSD-2-Clause) for the logical and the object backup (docs/spec/16-operations.md 8.1,
# brief M7-02): the release binary of the target architecture, checked against the SHA-256 of
# that architecture pinned here (from the release's SHA256SUMS), so a changed download fails the
# build; a transient error of the release host is retried (docs/DECISIONS.md PL/33). A version
# update changes RESTIC_VERSION and both checksums together. The binary gets the
# file capability cap_dac_read_search (restic's documented way to back up as a user that does not
# own the files): the volume files belongs to the server's user 65532 with mode 0700, and the
# service backup runs as 999 with cap_add DAC_READ_SEARCH (infra/compose/compose.yaml); without
# that capability in the container the kernel refuses to start restic (docs/decisions.d/M7-02.md
# D-M7-02-10).
FROM --platform=$BUILDPLATFORM postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336 AS restic
ARG TARGETARCH
ARG RESTIC_VERSION=0.19.1
ARG RESTIC_SHA256_AMD64=f415415624dcc452f2a02b8c33641791a8c6d6d3b65bbb3543fcf9a25151585c
ARG RESTIC_SHA256_ARM64=a5f64aaab53d51e311fa3829124c5b703f2d14cf187d8640b6be3b2b49376465
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
# Only the downloader and bzip2 of the build platform are installed; this stage never ends in a
# final image layer.
RUN set -eu; \
    case "$TARGETARCH" in \
      amd64) want="$RESTIC_SHA256_AMD64" ;; \
      arm64) want="$RESTIC_SHA256_ARM64" ;; \
      *) echo "no restic checksum for $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    apt-get update -qq; \
    apt-get install -y -qq --no-install-recommends curl ca-certificates bzip2 libcap2-bin >/dev/null; \
    curl -fsSL --proto '=https' --retry 5 --retry-all-errors -o /tmp/restic.bz2 \
      "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH}.bz2"; \
    echo "$want  /tmp/restic.bz2" | sha256sum -c -; \
    bunzip2 /tmp/restic.bz2; \
    install -D -m 0555 /tmp/restic /out/restic; \
    setcap cap_dac_read_search=+ep /out/restic; \
    getcap /out/restic | grep -q cap_dac_read_search

FROM postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336
ARG VERSION REVISION
ARG SOURCE_URL=https://github.com/OWNER/bothy
LABEL org.opencontainers.image.title="bothy-postgres" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.base.name="docker.io/library/postgres:18.6-trixie"
COPY --from=packages /rootfs/usr/ /usr/
COPY --from=packages /rootfs/var/lib/dpkg/status.d/ /var/lib/dpkg/status.d/
COPY --from=packages --chown=999:999 --chmod=0750 /owned/var/lib/pgbackrest /var/lib/pgbackrest
COPY --from=packages --chown=999:999 --chmod=0750 /owned/var/log/pgbackrest /var/log/pgbackrest
COPY --from=packages /link/etc/ /etc/
COPY --from=packages --chown=999:999 --chmod=0700 /owned/var/lib/bothy-backup /var/lib/bothy-backup
COPY --from=build --chown=0:0 --chmod=0555 /out/bothy-backupd /usr/local/bin/bothy-backupd
COPY --from=restic --chown=0:0 --chmod=0555 /out/restic /usr/local/bin/restic
COPY --chown=0:0 --chmod=0555 services/server/images/bothy-postgres/check-pgbackrest-version.sh /usr/local/bin/check-pgbackrest-version.sh
COPY --chown=0:0 --chmod=0555 services/server/images/bothy-postgres/entrypoint.sh /usr/local/bin/bothy-postgres-entrypoint
# Numeric, so runtimes that enforce "run as non-root" can verify it: the user postgres of the base
# image. Every role runs as this user; the entry point never needs root.
USER 999:999
ENTRYPOINT ["bothy-postgres-entrypoint"]
CMD ["postgres"]
