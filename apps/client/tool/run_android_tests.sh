#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the Android tests of the app on the one attached device or emulator (platform-tests.json,
# briefs M5-03 and M5-05 Tests; tools/ci/apps_platform.py has already checked that `adb devices`
# lists exactly one): the phone integration tests (integration_test/phone_suite_test.dart, the
# same files as on the iOS simulator) against the fake server inside the app, then the JVM tests
# and the instrumented tests of the runner (android/app/src/test and src/androidTest: channels,
# backup rules, recent apps, the push service with its TLS fake server on the device's
# loopback). The push tests run a second time in a build with the fake Firebase configuration of
# android/fixtures (D-M5-05-5), which compiles the FCM service and its test; the first run is
# the build without one, where the service is disabled. The fakes bind 127.0.0.1 on the device
# itself, so no port of the host is forwarded, and no request leaves the device.
set -eu
cd "$(dirname "$0")/.."
serial=$(adb devices | awk 'NR > 1 && $2 == "device" { print $1; exit }')
if [ -z "$serial" ]; then
  echo "client android: no device in state 'device' (adb devices)" >&2
  exit 1
fi
# The phone tests run without a system keyboard (FIX-main-red-2026-10-31): the text input probe
# of integration_test/phone/dictation_test.dart edits the composer as a dictation keyboard does,
# and a bound Gboard rewrote the same composing state after each insert, so the test failed at
# random. Every enabled input method is disabled for the phone tests and enabled again
# afterwards, with the default one selected, also when they fail (API 26 has no `ime reset`).
# `ime disable` alone does not hold: Android enables its default input methods again on its
# own (a package event does so on API 26, and Gboard was bound again in CI during the tests).
# So the packages of the input methods are disabled too (`pm disable-user`, which no such reset
# undoes) and enabled again afterwards; a package the shell may not disable, or one that was
# disabled before, stays as it is.
imes=$(adb -s "$serial" shell ime list -s | tr -d '\r')
default_ime=$(adb -s "$serial" shell settings get secure default_input_method | tr -d '\r')
ime_packages=$(adb -s "$serial" shell ime list -a | tr -d '\r' | sed -n 's|^ *mId=\([^/]*\)/.*|\1|p' | sort -u)
disabled_before=$(adb -s "$serial" shell pm list packages -d | tr -d '\r' | sed -n 's|^package:||p')
disabled_packages=""
restore_ime() {
  for package in $disabled_packages; do
    adb -s "$serial" shell pm enable --user 0 "$package" >/dev/null || true
  done
  for ime in $imes; do
    adb -s "$serial" shell ime enable "$ime" >/dev/null || true
  done
  case "$default_ime" in
    "" | null) ;;
    *) adb -s "$serial" shell ime set "$default_ime" >/dev/null || true ;;
  esac
}
trap restore_ime EXIT
for ime in $imes; do
  adb -s "$serial" shell ime disable "$ime" >/dev/null
done
for package in $ime_packages; do
  if printf '%s\n' "$disabled_before" | grep -qxF "$package"; then
    continue
  fi
  if adb -s "$serial" shell pm disable-user --user 0 "$package" | grep -q "new state: disabled-user"; then
    disabled_packages="$disabled_packages $package"
  fi
done
flutter test integration_test/phone_suite_test.dart -d "$serial"
# Evidence in the job log: no input method may have come back during the tests.
echo "client android: enabled input methods after the phone tests: [$(adb -s "$serial" shell ime list -s | tr -d '\r' | tr '\n' ' ')]"
restore_ime
trap - EXIT
cd android
# MainActivityTest is the bridge of the patrol device tests and runs only under `patrol test`;
# AccessibilityChecksTest needs the build of the audit entry (tool/run_android_a11y_checks.sh).
ANDROID_SERIAL="$serial" ./gradlew --no-daemon --console=plain :app:testDebugUnitTest :app:connectedDebugAndroidTest \
  -Pandroid.testInstrumentationRunnerArguments.notClass=com.example.client.MainActivityTest,com.example.client.AccessibilityChecksTest
# A build configuration with its own Firebase file was tested above; never replace that file.
if [ -e app/google-services.json ]; then
  exit 0
fi
# The fake configuration names the branding's application ID, so a changed one still matches.
app_id=$(sed -n 's/^applicationId=//p' branding.properties)
sed "s/\"package_name\": \"com.example.chat\"/\"package_name\": \"$app_id\"/" \
  fixtures/google-services.json > app/google-services.json
trap 'rm -f app/google-services.json' EXIT
ANDROID_SERIAL="$serial" ./gradlew --no-daemon --console=plain :app:connectedDebugAndroidTest \
  -Pandroid.testInstrumentationRunnerArguments.package=com.example.client.push
