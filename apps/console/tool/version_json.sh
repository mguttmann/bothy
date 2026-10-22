#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Writes version.json of the console build (brief M6-02 Design note 5,
# docs/DECISIONS.md D-M6-02-6): spec_version is info.version of the OpenAPI document the
# console's api_client was generated from, server_version the content of services/server/VERSION.
# The console compares spec_version with Discovery.spec_version of the server that serves it.
#
# Usage: version_json.sh <openapi.yaml> <server VERSION file> <output file>
# Run after `flutter build web`, which writes a version.json of its own that this one replaces.
set -eu
if [ "$#" -ne 3 ]; then
  echo "usage: $0 <openapi.yaml> <server VERSION file> <output file>" >&2
  exit 2
fi
spec=$(awk '/^info:/ { inside = 1; next } inside && /^[^ ]/ { exit } inside && /^  version:/ { print $2; exit }' "$1")
server=$(tr -d '[:space:]' < "$2")
case "$spec" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "$0: no SemVer info.version in $1" >&2; exit 1 ;;
esac
case "$server" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "$0: no SemVer version in $2" >&2; exit 1 ;;
esac
printf '{"spec_version":"%s","server_version":"%s"}\n' "$spec" "$server" > "$3"
