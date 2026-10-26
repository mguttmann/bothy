#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs ID-T16 (e2e_test/share/id_t16_test.dart, brief M5-06) on an iOS simulator against the
# stack that infra/compose/e2e.sh prepared; tools/e2e/id_t16.sh calls it as the command of e2e.sh
# and it never starts a stack itself. Inputs from e2e.sh: E2E_CONFIG and E2E_CA (files), and
# ID_T16_ACCESS_TOKEN_TTL_S and ID_T16_CLOCK_LEEWAY_S from tools/e2e/id_t16.sh. The first available iPhone simulator is
# used: the test CA becomes a trusted root there (the Share Extension talks to the TLS front
# with the system's trust), the sample photo goes to Photos, and the app is built with the
# simulator groups (CLIENTKIT_SIMULATOR_GROUPS=YES, docs/decisions.d/M5-06.md). A simulator this
# script booted is shut down again by its UDID. Exit codes: patrol's, 2 for a missing input.
set -eu
cd "$(dirname "$0")/.."
patrol_cli_version=4.8.0
for name in E2E_CONFIG E2E_CA; do
  eval "file=\${$name:-}"
  if [ -z "$file" ] || [ ! -r "$file" ]; then
    echo "id_t16: $name is not a readable file; run tools/e2e/id_t16.sh" >&2
    exit 2
  fi
done
for value in "${ID_T16_ACCESS_TOKEN_TTL_S:-}" "${ID_T16_CLOCK_LEEWAY_S:-}"; do
  case "$value" in
  '' | *[!0-9]*) echo "id_t16: ID_T16_ACCESS_TOKEN_TTL_S or ID_T16_CLOCK_LEEWAY_S is missing; run tools/e2e/id_t16.sh" >&2; exit 2 ;;
  esac
done
if ! xcrun --find simctl >/dev/null 2>&1; then
  echo "id_t16: Xcode with the iOS simulator is needed" >&2
  exit 2
fi
udid=$(xcrun simctl list devices available | sed -n 's/^ *iPhone[^(]*(\([0-9A-F-]\{36\}\)).*/\1/p' | head -n 1)
if [ -z "$udid" ]; then
  echo "id_t16: no available iPhone simulator (xcrun simctl list devices available)" >&2
  exit 2
fi
booted=0
if ! xcrun simctl list devices | grep "$udid" | grep -q "(Booted)"; then
  xcrun simctl boot "$udid"
  booted=1
fi
trap 'if [ "$booted" = 1 ]; then xcrun simctl shutdown "$udid"; fi' EXIT
xcrun simctl bootstatus "$udid" -b >/dev/null
xcrun simctl keychain "$udid" add-root-cert "$E2E_CA"
xcrun simctl addmedia "$udid" integration_test/share/assets/share-sample.jpg
xcrun simctl terminate "$udid" com.apple.mobileslideshow >/dev/null 2>&1 || true
b64() { base64 <"$1" | tr -d '\n\r'; }
if command -v patrol >/dev/null 2>&1; then
  set -- patrol
else
  dart pub global activate patrol_cli "$patrol_cli_version" >/dev/null
  set -- dart pub global run patrol_cli:main
fi
CLIENTKIT_SIMULATOR_GROUPS=YES "$@" test -d "$udid" \
  --target e2e_test/share/id_t16_test.dart \
  --dart-define "E2E_CONFIG=$(b64 "$E2E_CONFIG")" \
  --dart-define "E2E_CA=$(b64 "$E2E_CA")" \
  --dart-define "ID_T16_ACCESS_TOKEN_TTL_S=$ID_T16_ACCESS_TOKEN_TTL_S" \
  --dart-define "ID_T16_CLOCK_LEEWAY_S=$ID_T16_CLOCK_LEEWAY_S"
