# SPDX-License-Identifier: Apache-2.0
#
# ScanCode Toolkit for the release license scan (docs/quality/licenses.md section 4, brief
# M7-10). ScanCode publishes no container image, so this file builds one from the release
# archive of the project: the base image is pinned by digest (SEC-SC-4, `make lint`) and the
# archive by its sha256 (`ADD --checksum`), so the image is fully determined by this file.
# The archive bundles every Python dependency as a wheel and installs offline; its native
# wheels exist only for linux/amd64, so the image is linux/amd64 (on an arm64 host the scan
# runs emulated and slower; the release job runs on an amd64 runner).
#
#   docker buildx build --platform linux/amd64 --load --tag bothy-scancode:32.5.0-local tools/scancode
#
# `make scancode` builds and runs it. Renovate updates the base digest; a new ScanCode version
# changes SCANCODE_VERSION and SCANCODE_SHA256 together (sha256 of the GitHub release asset).

FROM python:3.13-slim-trixie@sha256:70729b46c69b4f1e97c4822c1af3df53a1476cf5ddc6c087c0c10bc3a5678c2f
ARG SCANCODE_VERSION=32.5.0
ARG SCANCODE_SHA256=f9f83ffcf90577a74b61a30ebc4632a1fc89f026b438f597b18206babb964e28
ADD --checksum=sha256:${SCANCODE_SHA256} \
    https://github.com/aboutcode-org/scancode-toolkit/releases/download/v${SCANCODE_VERSION}/scancode-toolkit-v${SCANCODE_VERSION}_py3.13-linux.tar.gz \
    /tmp/scancode.tar.gz
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
# libgomp1 and libpopt0 are runtime libraries of the bundled native wheels (license matching and
# archive extraction); configure installs only from the bundled wheels, without the network.
RUN apt-get update -qq \
    && apt-get install -qq -y --no-install-recommends libgomp1 libpopt0 bzip2 xz-utils \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt \
    && tar -xzf /tmp/scancode.tar.gz -C /opt \
    && mv "/opt/scancode-toolkit-v${SCANCODE_VERSION}" /opt/scancode-toolkit \
    && rm /tmp/scancode.tar.gz \
    && cd /opt/scancode-toolkit \
    && ./configure \
    && ./scancode --version
# The license index is built on first use and cached in SCANCODE_CACHE: a first scan builds it
# here, so the unprivileged user of the scan only reads it. HOME is writable for that user.
ENV SCANCODE_CACHE=/opt/scancode-cache SCANCODE_TEMP=/tmp HOME=/tmp
RUN mkdir /tmp/warmup \
    && echo "SPDX-License-Identifier: Apache-2.0" > /tmp/warmup/a.txt \
    && /opt/scancode-toolkit/scancode --license --quiet --json /tmp/warmup.json /tmp/warmup \
    && rm -rf /tmp/warmup /tmp/warmup.json \
    && chmod -R a+rX /opt/scancode-toolkit /opt/scancode-cache
USER 65532:65532
WORKDIR /scan
ENTRYPOINT ["/opt/scancode-toolkit/scancode"]
