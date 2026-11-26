#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Runs the end-to-end test of bothy-computer-mcp (e2e_test.go, build tag computere2e) in a
# throwaway Debian container with Xvfb, python3-xlib, Pillow and Tk, as an unprivileged user.
# The test binary is built here for Linux without cgo and copied into the container together
# with services/host/test (no bind mount, so any Docker-compatible runtime works); the container
# is removed at the end. Without a Docker-compatible runtime it fails.
#
#   services/host/test/computermcp/run.sh
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
host="$(cd "$here/../.." && pwd)"
image="${COMPUTER_E2E_IMAGE:-debian:trixie-slim}"
command -v docker >/dev/null || { echo "run.sh: no docker CLI" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "run.sh: no Docker-compatible runtime is reachable" >&2; exit 1; }
arch="$(docker info --format '{{.Architecture}}')"
case "$arch" in
  x86_64 | amd64) goarch=amd64 ;;
  aarch64 | arm64) goarch=arm64 ;;
  *) echo "run.sh: unknown architecture $arch" >&2; exit 1 ;;
esac
out="$(mktemp -d)"
cid=""
cleanup() {
  [ -z "$cid" ] || docker rm -f "$cid" >/dev/null 2>&1 || true
  rm -rf -- "$out"
}
trap cleanup EXIT
(cd "$host" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -tags computere2e -o "$out/e2e.test" ./test/computermcp/)
cid="$(docker create "$image" bash -euo pipefail -c '
  apt-get update -qq >/dev/null
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends xvfb python3 python3-xlib python3-pil python3-tk >/dev/null
  useradd -m -u 1000 alex
  chmod 0755 /usr/local/bin/e2e.test
  chmod -R a+rX /opt/host-test
  cd /opt/host-test/computermcp
  exec setpriv --reuid=1000 --regid=1000 --init-groups env HOME=/home/alex TMPDIR=/tmp /usr/local/bin/e2e.test -test.v -test.count=1
')"
docker cp "$out/e2e.test" "$cid:/usr/local/bin/e2e.test"
docker cp "$host/test" "$cid:/opt/host-test"
docker start -a "$cid"
