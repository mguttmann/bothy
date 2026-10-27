#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Measures the performance targets client.perf.* of the phone apps on an attached physical
# iPhone or Android phone (brief M5-11 Design note 5; docs/spec/15-client-architecture.md 19 and
# 20). `make perf` calls it at the close of a milestone and before a release.
#
# Profile builds of the phone apps run only on devices (Flutter builds the iOS simulator in debug
# only, and an emulator is no phone of 15 19), so the script looks for an attached physical
# device in `flutter devices --machine` (field `emulator` false, platform ios or android). With
# one it runs `flutter drive --profile` with perf_test/phone/mobile_perf_test.dart and
# test_driver/mobile_perf_driver.dart on it; the app measures against the fake server of M4-17
# inside the app, so no stack is needed. The driver writes the summary, the script prints it and
# ends with its result. Only a reference device of 15 19 (the fixed list in
# perf_test/phone/mobile_perf_summary.dart, decided in the app from the model the platform
# reports) can pass; the targets are compared with the values of AppConfig.
#
# Usage: apps/client/tool/mobile_perf.sh
#
# Inputs:
#   MOBILE_PERF_DEVICE   the Flutter device ID of the phone; default the first attached physical
#                        phone
#   MOBILE_PERF_SUMMARY  the summary file; default build/perf/mobile-summary.json of the app
#
# Output: exactly one line `SKIP perf-mobile: no physical reference device attached (targets not
# proven)` without an attached physical phone (also without flutter or dart, with the reason on
# stderr); otherwise the progress on stderr and the report of the summary on stdout.
# Exit codes: 0 a reference device met every target, or the SKIP line; 1 a target missed on a
# reference device, a failed run or no summary; 2 MOBILE_PERF_DEVICE names a device that is not
# among the attached physical phones, or the device list failed; 3 values from a device that is
# not a reference device (not proven).
# Compatible with bash 3.2.

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
client="$(cd "$here/.." && pwd)"
cli="$client/perf_test/phone/mobile_perf_cli.dart"

say() { printf 'perf-mobile: %s\n' "$*" >&2; }

skip='SKIP perf-mobile: no physical reference device attached (targets not proven)'

# Without the tools no phone can be found or measured: the same SKIP line, the reason on stderr.
if ! command -v flutter >/dev/null 2>&1 || ! command -v dart >/dev/null 2>&1; then
	say "flutter or dart is not on PATH, so no phone can be found"
	echo "$skip"
	exit 0
fi

# The warnings of the tool (the startup lock) go to stderr; stdout is the JSON list.
if ! listing="$(cd "$client" && flutter devices --machine 2>/dev/null)"; then
	say "flutter devices --machine failed"
	exit 2
fi
phones="$(printf '%s\n' "$listing" | dart "$cli" phones)"
if [[ -z "$phones" ]]; then
	echo "$skip"
	exit 0
fi

wanted="${MOBILE_PERF_DEVICE:-}"
id="" platform="" name=""
while IFS=$'\t' read -r p_id p_platform p_name; do
	if [[ -z "$wanted" || "$p_id" = "$wanted" ]]; then
		id="$p_id" platform="$p_platform" name="$p_name"
		break
	fi
done <<<"$phones"
if [[ -z "$id" ]]; then
	say "MOBILE_PERF_DEVICE=$wanted is no attached physical phone; attached:"
	printf '%s\n' "$phones" >&2
	exit 2
fi

summary="${MOBILE_PERF_SUMMARY:-$client/build/perf/mobile-summary.json}"
case "$summary" in
/*) ;;
*) summary="$PWD/$summary" ;;
esac
# The driver runs in the app's folder and finds the file through this variable.
export MOBILE_PERF_SUMMARY="$summary"
mkdir -p "$(dirname "$summary")"
# A fresh summary: one of an earlier run never counts.
rm -f "$summary"

say "measuring on $name ($platform, $id)"
cd "$client"
rc=0
# --no-start-paused: `flutter drive` starts the app paused until the driver connects by
# default; the cold start counts from the start of the process to `main`, so the app must not
# wait for the driver.
flutter drive --profile --no-start-paused -d "$id" \
	--driver=test_driver/mobile_perf_driver.dart \
	--target=perf_test/phone/mobile_perf_test.dart || rc=$?

result=0
dart "$cli" report "$summary" || result=$?
if [[ "$rc" -ne 0 ]]; then
	say "the measurement failed (flutter drive ended with $rc)"
	exit 1
fi
case "$result" in
0) say "pass on a reference device of 15 19" ;;
3) say "not a reference device of 15 19: the values are reported, the targets are not proven" ;;
*) say "the targets are not met (see $summary)" ;;
esac
exit "$result"
