#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs a command of the checkout in a container without root, for code that no maintainer has
# read (docs/decisions.d/AUD-misc-ci-gates-01.md). The job `record` of hermes-pin.yml records and
# tests the newest upstream commit of Hermes Agent with it. On a GitHub-hosted runner a step
# runs as the user that owns the runner's files, with sudo and the Docker socket: code in a
# step can read the credentials of the run (the cache of main among them) and change the steps
# after it. The container takes that reach away:
#
#   * its own process, mount and network namespaces: no process and no file of the runner is
#     visible, only the mounts below; the network is open for the pinned downloads;
#   * the ids of the caller, never root; no capability (`--cap-drop ALL`) and no way to a new
#     one (`no-new-privileges`), so a setuid program gains nothing; no Docker socket;
#   * the checkout is mounted read-only, with `.git`; the command can write only the folders
#     named with --write and its own home. Nothing the caller runs later comes from a place
#     the command could write: this script, its Dockerfile and uv are outside those folders.
#
# Before the command starts, the container checks these conditions from the inside and ends
# with exit code 70 when one does not hold, so a changed Docker default cannot weaken them
# unseen. What the command writes into a --write folder is untrusted for the caller: read it
# as data, never run it.
#
# Usage: sh tools/ci/sandbox.sh [--write <folder of the checkout>]... -- <command> [argument ...]
# Environment:
#   SANDBOX_HOME    home of the container user, kept between calls (uv cache, Python);
#                   default: a new temporary folder
#   SANDBOX_UV      the uv program for Linux to mount; default: uv on PATH
#   SANDBOX_DOCKER  the docker program; tests name a recorder
set -eu

usage() {
  echo "usage: sh tools/ci/sandbox.sh [--write <folder>]... -- <command> [argument ...]" >&2
  exit 2
}

here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo=$(CDPATH='' cd -- "$here/../.." && pwd)
docker=${SANDBOX_DOCKER-docker}
image=bothy-sandbox:local

# The mounts of the --write folders, one argument per line (a folder name has no line break).
writable=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --write)
      [ "$#" -ge 2 ] || usage
      folder=${2%/}
      case "/$folder/" in
        //* | */../* | */./* | *"
"*) echo "sandbox: --write takes a folder inside the checkout, got '$2'" >&2; exit 2 ;;
      esac
      [ -d "$repo/$folder" ] && [ ! -L "$repo/$folder" ] || {
        echo "sandbox: --write $folder is no folder of the checkout" >&2
        exit 2
      }
      writable="$writable--volume
$repo/$folder:/repo/$folder
"
      shift 2
      ;;
    --)
      shift
      break
      ;;
    *) usage ;;
  esac
done
[ "$#" -gt 0 ] || usage

uv=${SANDBOX_UV-$(command -v uv || true)}
[ -n "$uv" ] && [ -f "$uv" ] || { echo "sandbox: uv not found (SANDBOX_UV or PATH)" >&2; exit 2; }
home=${SANDBOX_HOME-}
[ -n "$home" ] || home=$(mktemp -d)
mkdir -p "$home"

"$docker" build --tag "$image" \
  --build-arg "SANDBOX_UID=$(id -u)" --build-arg "SANDBOX_GID=$(id -g)" \
  - < "$here/sandbox.Dockerfile" >&2

# Runs inside the container, before the command: each condition the header promises.
# shellcheck disable=SC2016 # the text of a script for the shell in the container, expanded there
check='
fail() { echo "sandbox: $1; the command did not start" >&2; exit 70; }
[ "$(id -u)" != 0 ] || fail "the container runs as root"
grep -Eq "^NoNewPrivs:[[:space:]]+1$" /proc/self/status || fail "new privileges are possible"
grep -Eq "^CapBnd:[[:space:]]+0+$" /proc/self/status || fail "capabilities are left"
[ ! -e /var/run/docker.sock ] || fail "the Docker socket is mounted"
if touch /repo/.sandbox-probe 2>/dev/null; then
  rm -f /repo/.sandbox-probe
  fail "the checkout is writable"
fi
exec "$@"
'

# The --write mounts come after the read-only checkout, so they overlay their folders.
set -- "$image" sh -c "$check" sandbox "$@"
old_ifs=$IFS
IFS='
'
set -f
# shellcheck disable=SC2086 # one mount argument per line, split on purpose
set -- $writable "$@"
set +f
IFS=$old_ifs
exec "$docker" run --rm --init \
  --user "$(id -u):$(id -g)" \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --volume "$repo:/repo:ro" \
  --volume "$uv:/usr/local/bin/uv:ro" \
  --volume "$home:/home/sandbox" \
  --env HOME=/home/sandbox \
  --workdir /repo \
  "$@"
