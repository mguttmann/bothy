#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the Android tests of the app on the one attached device or emulator (platform-tests.json,
# brief M5-03 Tests; tools/ci/apps_platform.py has already checked that `adb devices` lists
# exactly one): the phone integration tests (integration_test/phone_suite_test.dart, the same
# files as on the iOS simulator) against the fake server inside the app, then the instrumented
# tests of the runner (android/app/src/androidTest: channels, backup rules, recent apps). The
# fake binds 127.0.0.1 on the device itself, so no port of the host is forwarded.
set -eu
cd "$(dirname "$0")/.."
serial=$(adb devices | awk 'NR > 1 && $2 == "device" { print $1; exit }')
if [ -z "$serial" ]; then
  echo "client android: no device in state 'device' (adb devices)" >&2
  exit 1
fi
flutter test --no-dds integration_test/phone_suite_test.dart -d "$serial"
cd android
ANDROID_SERIAL="$serial" ./gradlew --no-daemon --console=plain :app:connectedDebugAndroidTest
