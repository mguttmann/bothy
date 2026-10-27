#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the mobile E2E suites (e2e_test/phone, brief M5-11 Design note 2, and the terminal suite of
# brief M5-13) on one iOS simulator or one Android emulator against the stack the runner
# prepared. `make e2e-mobile` (tools/e2e/mobile_e2e.py) calls it as the command of
# infra/compose/e2e.sh for the suites of M5-11, after it booted the device and started the push
# relay, and with `terminal` against the terminal stack of the Go helper
# services/server/test/terminal/clientstack (mode mobile); this script never starts a stack, a
# device or the relay.
#
# Usage: tool/mobile_e2e_suite.sh ios|android [login|push|share|offline|terminal ...]
#   Without suites it runs login, push, share and offline, in that order; `terminal` runs only
#   when named.
# Inputs: for login, push, share and offline E2E_CONFIG and E2E_CA (files) and E2E_SERVER_URL
# (https://localhost:<port>) from e2e.sh; for terminal TERMINAL_STACK (the connection file of the
# helper) and on Android MOBILE_E2E_TERMINAL_PORTS (the published port of the TLS front and the
# port of the control endpoint, both on 127.0.0.1); MOBILE_E2E_UDID (ios) or MOBILE_E2E_SERIAL
# (android), the booted device.
#
# The device is prepared first:
#   ios      the test CA becomes a trusted root of the simulator (the Share Extension and the
#            Notification Service Extension talk to the TLS front with the system's trust), the
#            sample photo goes to Photos, and the app is built with the simulator groups and the
#            push entitlement (CLIENTKIT_SIMULATOR_GROUPS=PUSH, docs/decisions.d/M5-11.md), so
#            the simulator registers it for remote notifications.
#   android  `adb reverse` makes localhost:<port> of the emulator the server's port on this
#            host, the sample image goes to the downloads, and the debug build gets the E2E
#            push backend and the trust of the test CA (Gradle properties e2ePush and
#            e2eCa, brief M5-11 part B). For terminal `adb reverse` maps the two ports of
#            MOBILE_E2E_TERMINAL_PORTS the same way, and the build is the plain debug build (the
#            suite trusts the stack's CA inside the app, e2e_test/support/e2e_network.dart). Each suite starts from a fresh install, so the
#            notification permission is asked again; the push suite needs API 33 or later,
#            where the app asks for it (below, notifications are allowed without asking).
# Each suite is one `patrol test` run of its file (its own install and app process); every
# named suite runs and is reported, and the exit code is that of the first suite that failed
# (patrol's), 0 when all passed, 2 for a missing input. The world reaches the test build as
# defines only; nothing the suites exchange with the server is written by this script. What
# this script added to an emulator (the port reverse, the sample, the app) is removed at the end.
set -eu
cd "$(dirname "$0")/.."
patrol_cli_version=4.8.0
sample=integration_test/share/assets/share-sample.jpg

usage() {
  echo "usage: tool/mobile_e2e_suite.sh ios|android [login|push|share|offline|terminal ...]" >&2
  exit 2
}

platform="${1:-}"
case "$platform" in
ios | android) shift ;;
*) usage ;;
esac
if [ "$#" -eq 0 ]; then
  set -- login push share offline
fi
# world: a suite of M5-11 against the stack of e2e.sh; terminal: the terminal suite of M5-13.
world=0
terminal=0
for suite in "$@"; do
  case "$suite" in
  login | push | share | offline) world=1 ;;
  terminal) terminal=1 ;;
  *)
    echo "mobile-e2e: unknown suite '$suite'" >&2
    usage
    ;;
  esac
done

b64() { base64 <"$1" | tr -d '\n\r'; }
port=""
ca=""
if [ "$world" -eq 1 ]; then
  for file in "${E2E_CONFIG:-}" "${E2E_CA:-}"; do
    if [ -z "$file" ] || [ ! -r "$file" ]; then
      echo "mobile-e2e: E2E_CONFIG or E2E_CA is not a readable file; run through make e2e-mobile" >&2
      exit 2
    fi
  done
  port="${E2E_SERVER_URL:-}"
  port="${port#https://localhost:}"
  port="${port%/}"
  case "$port" in
  '' | *[!0-9]*)
    echo "mobile-e2e: E2E_SERVER_URL is not https://localhost:<port>; run through make e2e-mobile" >&2
    exit 2
    ;;
  esac
  # The Gradle property names a file the build reads, so it is an absolute path.
  ca="$(cd "$(dirname "$E2E_CA")" && pwd)/$(basename "$E2E_CA")"
  config_b64=$(b64 "$E2E_CONFIG")
  ca_b64=$(b64 "$ca")
fi
terminal_ports=""
if [ "$terminal" -eq 1 ]; then
  if [ -z "${TERMINAL_STACK:-}" ] || [ ! -r "$TERMINAL_STACK" ]; then
    echo "mobile-e2e: TERMINAL_STACK is not a readable file; run through make e2e-mobile" >&2
    exit 2
  fi
  stack_b64=$(b64 "$TERMINAL_STACK")
  if [ "$platform" = android ]; then
    terminal_ports="${MOBILE_E2E_TERMINAL_PORTS:-}"
    words() { echo "$#"; }
    # shellcheck disable=SC2086 # two port numbers, split on purpose
    count=$(words $terminal_ports)
    case "$terminal_ports" in
    *[!0-9\ ]*) count=0 ;;
    esac
    if [ "$count" -ne 2 ]; then
      echo "mobile-e2e: MOBILE_E2E_TERMINAL_PORTS must be two port numbers; run through make e2e-mobile" >&2
      exit 2
    fi
  fi
fi

patrol_cmd() {
  if command -v patrol >/dev/null 2>&1; then
    patrol "$@"
  else
    dart pub global activate patrol_cli "$patrol_cli_version" >/dev/null
    dart pub global run patrol_cli:main "$@"
  fi
}

# run_suite runs the suite $1 with the device arguments that follow and records its result.
failed=0
results=""
# The host ports of patrol's servers: the runner picks free ones (MOBILE_E2E_PATROL_PORTS, "test
# app"), because patrol's defaults are reserved by the host rules of the build host.
patrol_ports=""
if [ -n "${MOBILE_E2E_PATROL_PORTS:-}" ]; then
  set -f
  # shellcheck disable=SC2086 # two port numbers, split on purpose
  set -- $MOBILE_E2E_PATROL_PORTS "$@"
  set +f
  case "$1$2" in
  '' | *[!0-9]*)
    echo "mobile-e2e: MOBILE_E2E_PATROL_PORTS must be two port numbers" >&2
    exit 2
    ;;
  esac
  patrol_ports="--test-server-port $1 --app-server-port $2"
  shift 2
fi
run_suite() {
  suite=$1
  shift
  rc=0
  if [ "$suite" = terminal ]; then
    # The plain debug build: the terminal suite needs neither the push backend nor the trust of
    # a CA in the platform.
    (
      unset ORG_GRADLE_PROJECT_e2ePush ORG_GRADLE_PROJECT_e2eCa
      # shellcheck disable=SC2086 # $patrol_ports is empty or four words without spaces inside
      patrol_cmd test "$@" $patrol_ports \
        --target "e2e_test/phone/terminal_e2e_test.dart" \
        --dart-define "TERMINAL_STACK=$stack_b64"
    ) || rc=$?
  else
    # shellcheck disable=SC2086 # $patrol_ports is empty or four words without spaces inside
    patrol_cmd test "$@" $patrol_ports \
      --target "e2e_test/phone/${suite}_e2e_test.dart" \
      --dart-define "E2E_CONFIG=$config_b64" \
      --dart-define "E2E_CA=$ca_b64" || rc=$?
  fi
  if [ "$rc" -eq 0 ]; then
    results="$results
mobile-e2e $platform $suite: PASS"
  else
    results="$results
mobile-e2e $platform $suite: FAIL (exit $rc)"
    if [ "$failed" -eq 0 ]; then failed=$rc; fi
  fi
}

case "$platform" in
ios)
  udid="${MOBILE_E2E_UDID:-}"
  if [ -z "$udid" ]; then
    echo "mobile-e2e: MOBILE_E2E_UDID names no simulator; run through make e2e-mobile" >&2
    exit 2
  fi
  if ! xcrun --find simctl >/dev/null 2>&1; then
    echo "mobile-e2e: Xcode with the iOS simulator is needed" >&2
    exit 2
  fi
  xcrun simctl bootstatus "$udid" -b >/dev/null
  if [ "$world" -eq 1 ]; then
    xcrun simctl keychain "$udid" add-root-cert "$ca"
    xcrun simctl addmedia "$udid" "$sample"
  fi
  export CLIENTKIT_SIMULATOR_GROUPS=PUSH
  for suite in "$@"; do
    # Photos starts afresh, so a share sheet left open by an earlier run hides nothing.
    xcrun simctl terminate "$udid" com.apple.mobileslideshow >/dev/null 2>&1 || true
    run_suite "$suite" -d "$udid"
  done
  ;;
android)
  serial="${MOBILE_E2E_SERIAL:-}"
  if [ -z "$serial" ]; then
    echo "mobile-e2e: MOBILE_E2E_SERIAL names no emulator; run through make e2e-mobile" >&2
    exit 2
  fi
  adb=adb
  if ! command -v adb >/dev/null 2>&1; then
    adb="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}/platform-tools/adb"
    if [ ! -x "$adb" ]; then
      echo "mobile-e2e: adb is needed (PATH, ANDROID_HOME or ANDROID_SDK_ROOT)" >&2
      exit 2
    fi
    # patrol finds adb on PATH only.
    PATH="$(dirname "$adb"):$PATH"
    export PATH
  fi
  if [ "$("$adb" -s "$serial" get-state 2>/dev/null || true)" != device ]; then
    echo "mobile-e2e: $serial is not a device in state 'device' (adb devices)" >&2
    exit 2
  fi
  app_id=$(sed -n 's/^applicationId=//p' android/branding.properties)
  name=$(basename "$sample")
  # set +e: under errexit the first failing step (the app is gone already) would end the trap,
  # skip the steps after it and turn the result of the suites into 1.
  # The ports `adb reverse` maps: the server's of e2e.sh, the two of the terminal stack.
  reversed="$port $terminal_ports"
  trap 'set +e
    for p in $reversed; do
      "$adb" -s "$serial" reverse --remove "tcp:$p" >/dev/null 2>&1
    done
    [ "$world" -eq 0 ] || "$adb" -s "$serial" shell rm -f "/sdcard/Download/$name" >/dev/null 2>&1
    "$adb" -s "$serial" uninstall "$app_id" >/dev/null 2>&1
    "$adb" -s "$serial" uninstall "$app_id.test" >/dev/null 2>&1
    true' EXIT
  for p in $reversed; do
    "$adb" -s "$serial" reverse "tcp:$p" "tcp:$p" >/dev/null
  done
  if [ "$world" -eq 1 ]; then
    "$adb" -s "$serial" push "$sample" /sdcard/Download/ >/dev/null
    "$adb" -s "$serial" shell am broadcast -a android.intent.action.MEDIA_SCANNER_SCAN_FILE \
      -d "file:///sdcard/Download/$name" >/dev/null
    export ORG_GRADLE_PROJECT_e2ePush=true ORG_GRADLE_PROJECT_e2eCa="$ca"
  fi
  for suite in "$@"; do
    # A fresh install per suite: the notification permission is asked again, and nothing of an
    # earlier suite's app is left. The file app lists the downloads as it last loaded them, so
    # it starts afresh too.
    "$adb" -s "$serial" uninstall "$app_id" >/dev/null 2>&1 || true
    "$adb" -s "$serial" uninstall "$app_id.test" >/dev/null 2>&1 || true
    "$adb" -s "$serial" shell am force-stop com.google.android.documentsui >/dev/null 2>&1 || true
    "$adb" -s "$serial" shell am force-stop com.android.documentsui >/dev/null 2>&1 || true
    # --no-uninstall: the Gradle test run removes the app on some API levels already, and a
    # second removal by patrol would fail the run; the loop and the trap remove it instead.
    run_suite "$suite" -d "$serial" --no-uninstall
  done
  ;;
esac

printf '%s\n' "$results" | sed '/^$/d'
exit "$failed"
