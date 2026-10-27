#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the checks of the Accessibility Test Framework on the phone screens (brief M5-10 Design
# note 3; platform-tests.json) on the one attached device or emulator (or ANDROID_SERIAL): the
# instrumented test android/app/src/androidTest/.../AccessibilityChecksTest.kt in a build of
# the audit entry integration_test/a11y_audit/audit_app.dart, which shows the screen the test
# names as its initial route. The app and its test APK are removed again at the end. Without a
# device it prints a SKIP line.
set -eu
cd "$(dirname "$0")/.."
serial=${ANDROID_SERIAL:-$(adb devices | awk 'NR > 1 && $2 == "device" { print $1; exit }')}
if [ -z "$serial" ]; then
  echo "SKIP client android-a11y-checks: no device in state 'device' (adb devices)"
  exit 0
fi
entry="$(pwd)/integration_test/a11y_audit/audit_app.dart"
app_id=$(sed -n 's/^applicationId=//p' android/branding.properties)
# Under `set -e` a failing command ends the trap with its status, so each one ignores its own.
trap 'status=$?; adb -s "$serial" uninstall "$app_id" >/dev/null 2>&1 || true; adb -s "$serial" uninstall "$app_id.test" >/dev/null 2>&1 || true; exit $status' EXIT
cd android
ANDROID_SERIAL="$serial" ./gradlew --no-daemon --console=plain :app:connectedDebugAndroidTest \
  -Ptarget="$entry" \
  -Pandroid.testInstrumentationRunnerArguments.class=com.example.client.AccessibilityChecksTest
