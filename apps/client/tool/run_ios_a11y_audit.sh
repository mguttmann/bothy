#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the accessibility audit of Xcode on the phone screens (brief M5-10 Design note 3;
# platform-tests.json): builds the app with the audit entry
# integration_test/a11y_audit/audit_app.dart for the first available iPhone simulator and runs
# the UI tests ios/RunnerUITests/AccessibilityAuditTests.swift, which start that build once per
# screen and call `performAccessibilityAudit`. A simulator this script booted is shut down
# again by its UDID. Without Xcode there is no simulator: the script prints a SKIP line and
# exits 0; the apps-ios CI job (macos-15 with Xcode) runs it for real.
set -eu
cd "$(dirname "$0")/.."
if ! xcrun --find simctl >/dev/null 2>&1 || ! xcrun --find xcodebuild >/dev/null 2>&1; then
  echo "SKIP client ios-a11y-audit: Xcode with the iOS simulator is not installed on this host"
  exit 0
fi
udid=$(xcrun simctl list devices available | sed -n 's/^ *iPhone[^(]*(\([0-9A-F-]\{36\}\)).*/\1/p' | head -n 1)
if [ -z "$udid" ]; then
  echo "client ios-a11y-audit: no available iPhone simulator (xcrun simctl list devices available)" >&2
  exit 1
fi
booted=0
if ! xcrun simctl list devices | grep "$udid" | grep -q "(Booted)"; then
  xcrun simctl boot "$udid"
  booted=1
fi
# Under `set -e` a failing command ends the trap with its status, so the shutdown ignores its own.
trap 'status=$?; if [ "$booted" = 1 ]; then xcrun simctl shutdown "$udid" || true; fi; exit $status' EXIT
xcrun simctl bootstatus "$udid" -b >/dev/null
# Only the configuration: ios/Flutter/Generated.xcconfig gets the audit entry as FLUTTER_TARGET, so
# the one build of the test run below compiles that entry, not lib/main.dart.
flutter build ios --config-only --simulator --debug --target integration_test/a11y_audit/audit_app.dart
# The target also holds the bridge of the patrol device tests (RunnerUITests.m), whose macro
# needs the two settings `patrol test` passes; this run selects only the audit.
xcodebuild test \
  -workspace ios/Runner.xcworkspace \
  -scheme Runner \
  -configuration Debug \
  -destination "id=$udid" \
  -only-testing:RunnerUITests/AccessibilityAuditTests \
  "GCC_PREPROCESSOR_DEFINITIONS=\$(inherited) FULL_ISOLATION=0 CLEAR_PERMISSIONS=0"
