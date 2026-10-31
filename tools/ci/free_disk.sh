#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Frees disk space on a GitHub-hosted ubuntu runner before a heavy job (FIX-main-red-2026-10-31,
# docs/quality/gates.md "CI jobs"). The runner image ships toolchains that no job of this
# repository uses; with them the disk ran full in `gates`, `test (tools)` and `console-e2e`
# ("No space left on device"). This removes .NET, GHC, Swift, Boost and the CodeQL bundles of
# the tool cache, the Android SDK unless --keep-android, and every Docker image, then prints
# the free space of / before and after. Missing folders and a missing docker are no error.
# Every heavy Linux job of ci.yml runs it right after checkout, with --keep-android when it
# installs a JDK, the Android SDK, Flutter or an emulator (tools/ci/tests/test_free_disk.py).
#
# Usage: sh tools/ci/free_disk.sh [--keep-android]
# Tests set FREE_DISK_ROOT (a fake root), FREE_DISK_SUDO (empty: no sudo) and FREE_DISK_DOCKER.
set -eu

keep_android=0
case "${1-}" in
  "") ;;
  --keep-android) keep_android=1 ;;
  *)
    echo "usage: sh tools/ci/free_disk.sh [--keep-android]" >&2
    exit 2
    ;;
esac

root=${FREE_DISK_ROOT-}
sudo=${FREE_DISK_SUDO-sudo}
docker=${FREE_DISK_DOCKER-docker}

free() {
  echo "free disk space $1:"
  df -h "${root:-/}" | sed -n '2p'
}

free before
folders="$root/usr/share/dotnet $root/opt/ghc $root/usr/local/.ghcup $root/usr/share/swift"
folders="$folders $root/usr/local/share/boost $root/opt/hostedtoolcache/CodeQL"
if [ "$keep_android" = 0 ]; then
  folders="$folders $root/usr/local/lib/android"
fi
# The folders hold hundreds of thousands of files: remove them at the same time.
for folder in $folders; do
  $sudo rm -rf "$folder" &
done
wait
if command -v "$docker" >/dev/null 2>&1; then
  "$docker" image prune --all --force >/dev/null
fi
free after
