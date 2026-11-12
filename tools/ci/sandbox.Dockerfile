# SPDX-License-Identifier: Apache-2.0
#
# The container of tools/ci/sandbox.sh: a command of the checkout runs in it as a user without
# root, for code that no maintainer has read (the newest upstream commit of Hermes Agent in
# .github/workflows/hermes-pin.yml, docs/decisions.d/AUD-misc-ci-gates-01.md). It holds only what
# the targets of a Python component need: git and make, the certificates for the pinned
# downloads, `ps`, with which the recording harness reads the state of the processes it
# started, and a Python on the search path, as a runner has one: the provisioning tests of the
# adapter lock a throwaway project offline with it. The Python of a component still comes from
# uv. uv is no part of the image: sandbox.sh mounts the uv of the caller, so the container runs
# the version every job runs.
#
# The user gets the ids of the caller (build arguments), so the caller can read what the
# container wrote, and an entry in the account database, which the recording harness reads.
# `--non-unique`: the ids of a developer's host can exist in the base image already.
#
# The base image is pinned by digest (SEC-SC-4, `make lint`); Renovate updates the digest.

FROM debian:trixie-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f
ARG SANDBOX_UID=1000
ARG SANDBOX_GID=1000
RUN apt-get update -qq \
    && DEBIAN_FRONTEND=noninteractive apt-get install -qq -y --no-install-recommends ca-certificates git make procps python3 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --non-unique --gid "$SANDBOX_GID" sandbox \
    && useradd --non-unique --uid "$SANDBOX_UID" --gid "$SANDBOX_GID" --no-create-home --home-dir /home/sandbox sandbox
USER sandbox
WORKDIR /repo
