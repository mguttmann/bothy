#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Runs the desktop E2E suite or the performance level of the app against the stack that
# infra/compose/e2e.sh prepared (brief M4-34; docs/spec/15-client-architecture.md 19 and 20).
# `make e2e-desktop` and `make perf-desktop` call it as the command of e2e.sh; it never starts a
# stack itself.
#
# Usage: apps/client/tool/e2e_suite.sh e2e|perf|terminal|terminal-perf
#   e2e    the suite e2e_test/desktop_e2e_test.dart: on macOS or Windows through flutter drive
#          with test_driver/e2e_driver.dart (flutter test runs a file outside integration_test/ on
#          flutter-tester whatever device is named), headless with flutter test on flutter-tester
#   perf   flutter drive --profile with test_driver/perf_driver.dart and perf_test/perf_test.dart;
#          the driver writes a fresh build/perf/summary.json and fails on a missed target
#   terminal       the terminal E2E e2e_test/terminal_e2e_test.dart against the client stack of
#                  the terminal (brief M4-38; tools/ci/terminal_stack.py --mode e2e), like e2e
#   terminal-perf  flutter drive --profile with the timing hooks of terminal_ui
#                  (TERMINAL_PERF_HOOKS=true) and perf_test/terminal_perf_test.dart
#                  (tools/ci/terminal_stack.py --mode perf); with PERF_SUMMARY_MERGE=1 the driver
#                  adds its results to build/perf/summary.json
#
# Inputs:
#   E2E_CONFIG, E2E_CA   files of e2e.sh (e2e.json and the PEM of the test CA); required for e2e
#                        and perf
#   TERMINAL_STACK       the connection file of the terminal's client stack; required for
#                        terminal and terminal-perf
#   E2E_DEVICE           the Flutter device: default macos on macOS and windows on Windows;
#                        flutter-tester runs an E2E suite headless (no profile build, so not
#                        for perf or terminal-perf)
#   PERF_DEVICE_LABEL    the name of the measuring device in summary.json (perf modes only)
#   PERF_REFERENCE=1     the device is a reference device of 15 19 (perf modes only)
#
# The world, the CA and the terminal's connection data reach the build as --dart-define values in
# base64 (the test app runs in its own sandbox, e2e_test/support/e2e_world.dart and
# terminal_stack.dart). Exit codes: the code of flutter, 2 for a missing input. Compatible with
# bash 3.2.

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
client="$(cd "$here/.." && pwd)"

say() { printf 'e2e-suite: %s\n' "$*" >&2; }

mode="${1:-}"
case "$mode" in
e2e | perf) inputs="E2E_CONFIG E2E_CA" runner="infra/compose/e2e.sh" ;;
terminal | terminal-perf) inputs="TERMINAL_STACK" runner="tools/ci/terminal_stack.py" ;;
*) say "usage: e2e_suite.sh e2e|perf|terminal|terminal-perf"; exit 2 ;;
esac
for name in $inputs; do
	file="${!name:-}"
	[[ -n "$file" && -r "$file" ]] || { say "$name is not a readable file; run through $runner (make e2e-desktop or make perf-desktop)"; exit 2; }
done

default_device() {
	case "$(uname -s)" in
	Darwin) printf 'macos' ;;
	MINGW* | MSYS* | CYGWIN* | Windows_NT) printf 'windows' ;;
	*) printf 'linux' ;;
	esac
}
device="${E2E_DEVICE:-$(default_device)}"

# One line of base64 on every system (GNU base64 wraps at 76 columns).
b64() {
	local file="$1"
	base64 <"$file" | tr -d '\n\r'
}
case "$mode" in
e2e | perf)
	defines=(--dart-define="E2E_CONFIG=$(b64 "$E2E_CONFIG")" --dart-define="E2E_CA=$(b64 "$E2E_CA")")
	suite=e2e_test/desktop_e2e_test.dart perf=perf_test/perf_test.dart
	;;
*)
	defines=(--dart-define="TERMINAL_STACK=$(b64 "$TERMINAL_STACK")")
	suite=e2e_test/terminal_e2e_test.dart perf=perf_test/terminal_perf_test.dart
	;;
esac

cd "$client"
if [[ "$mode" = e2e || "$mode" = terminal ]]; then
	say "E2E suite $suite on $device"
	if [[ "$device" = flutter-tester ]]; then
		exec flutter test "$suite" "${defines[@]}"
	fi
	exec flutter drive -d "$device" --driver=test_driver/e2e_driver.dart \
		--target="$suite" "${defines[@]}"
fi

[[ "$device" != flutter-tester ]] || { say "the performance level needs a profile build on macos or windows"; exit 2; }
label="${PERF_DEVICE_LABEL:-$(uname -s) $(uname -m)}"
say "performance level $perf on $device ($label)"
mkdir -p build/perf
if [[ "$mode" = perf ]]; then
	# A fresh summary: the terminal part of make perf-desktop adds to it, never to an old one.
	rm -f build/perf/summary.json
else
	# The timing hooks of terminal_ui exist only with this define (brief M4-38).
	defines+=(--dart-define=TERMINAL_PERF_HOOKS=true)
fi
exec flutter drive --profile -d "$device" \
	--driver=test_driver/perf_driver.dart --target="$perf" \
	"${defines[@]}" --dart-define="PERF_DEVICE_LABEL=$label" --dart-define="PERF_REFERENCE=${PERF_REFERENCE:-0}"
