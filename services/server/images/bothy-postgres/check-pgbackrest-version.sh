#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Version guard of bothy-postgres (docs/spec/16-operations.md 8.1 rationale 4, brief M7-01):
# pgBackRest below 2.59.3 can produce weak keys and salts, so the image never ships it and never
# starts it.
#
#   check-pgbackrest-version.sh [BINARY]          runs "BINARY version" (default pgbackrest)
#   check-pgbackrest-version.sh --reported TEXT   checks TEXT, the output of "pgbackrest version"
#
# The Dockerfile checks the pinned package version with --reported, because the binary of the
# target platform does not run on the build platform; the entry point checks the installed
# binary on every start. Exit codes: 0 the version is 2.59.3 or later, 1 it is older or unreadable.
set -eu

minimum=2.59.3

fail() {
	echo "check-pgbackrest-version: $1; bothy-postgres needs pgBackRest $minimum or later" >&2
	exit 1
}

if [ "${1:-}" = --reported ]; then
	reported=${2:-}
else
	bin=${1:-pgbackrest}
	reported=$("$bin" version 2>/dev/null) || fail "\"$bin version\" failed"
fi

version=${reported#pgBackRest }
case $version in
'' | *[!0-9.]* | .* | *. | *..*) fail "cannot read a release version from \"$reported\"" ;;
*) ;;
esac

# Compare major, minor and patch as numbers; a missing part counts as 0.
IFS=. read -r have_major have_minor have_patch <<EOF
$version
EOF
IFS=. read -r want_major want_minor want_patch <<EOF
$minimum
EOF
older=$(awk -v a="${have_major:-0}" -v b="${have_minor:-0}" -v c="${have_patch:-0}" \
	-v x="$want_major" -v y="$want_minor" -v z="$want_patch" \
	'BEGIN { print ((a < x) || (a == x && b < y) || (a == x && b == y && c < z)) ? 1 : 0 }')
[ "$older" = 0 ] || fail "found $version"
echo "pgBackRest $version (at least $minimum)"
