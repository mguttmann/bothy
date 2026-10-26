#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the phone integration tests (integration_test/phone_suite_test.dart) on an iOS simulator
# (platform-tests.json, brief M5-02 Design note 7). The first available iPhone of
# `xcrun simctl list` is used, so no device name is written down here; a simulator this script
# booted is shut down again by its UDID. Without Xcode there is no simulator: the script prints a
# SKIP line and exits 0; the apps-ios CI job (macos-15 with Xcode) runs it for real (PL/44).
set -eu
cd "$(dirname "$0")/.."
if ! xcrun --find simctl >/dev/null 2>&1 || ! xcrun --find xcodebuild >/dev/null 2>&1; then
  echo "SKIP client ios-simulator: Xcode with the iOS simulator is not installed on this host"
  exit 0
fi
udid=$(xcrun simctl list devices available | sed -n 's/^ *iPhone[^(]*(\([0-9A-F-]\{36\}\)).*/\1/p' | head -n 1)
if [ -z "$udid" ]; then
  echo "client ios-simulator: no available iPhone simulator (xcrun simctl list devices available)" >&2
  exit 1
fi
booted=0
if ! xcrun simctl list devices | grep "$udid" | grep -q "(Booted)"; then
  xcrun simctl boot "$udid"
  booted=1
fi
trap 'if [ "$booted" = 1 ]; then xcrun simctl shutdown "$udid"; fi' EXIT
xcrun simctl bootstatus "$udid" -b >/dev/null
# The voice test (brief M5-07) records with the simulator's audio input only when
# CLIENT_VOICE_REAL_RECORDER is set (the audio input of a hosted runner is not dependable) and
# the microphone is allowed before the run, since no test can answer the system dialog;
# otherwise the test prints its SKIP line and uses the fake recorder.
bundle=$(sed -n 's/^BRANDING_BUNDLE_ID *= *//p' ios/Runner/Configs/Branding.xcconfig)
if [ -n "$bundle" ]; then
  xcrun simctl privacy "$udid" grant microphone "$bundle" ||
    echo "client ios-simulator: the microphone could not be granted to $bundle before the run"
fi
# One line with the define always present (empty means false): the PL/44 check
# (tools/ci/tests/test_flutter_device_tests_one_file.py) reads each `flutter test` line on its own.
flutter test integration_test/phone_suite_test.dart -d "$udid" --dart-define=CLIENT_VOICE_REAL_RECORDER="${CLIENT_VOICE_REAL_RECORDER:+true}"
