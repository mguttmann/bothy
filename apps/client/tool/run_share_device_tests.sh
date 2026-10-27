#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the patrol device tests of sharing (brief M5-06 Tests; platform-tests.json) against the
# fake server of M4-17 inside the app:
#   ios      on the first available iPhone simulator: the sample photo is added to Photos, the
#            app is built with CLIENTKIT_SIMULATOR_GROUPS=YES (the App Group and keychain group
#            of an unsigned simulator build, docs/decisions.d/M5-06.md) and
#            integration_test/share/share_image_from_photos_test.dart and share_offline_test.dart
#            run through the real Share Extension.
#   android  on the one attached device or emulator (or ANDROID_SERIAL): the samples go to
#            the downloads and share_image_from_photos_test.dart and share_android_mime_test.dart run through the
#            file app and the share chooser.
# patrol_cli comes from `dart pub global activate` in the version below when it is missing. A
# simulator this script booted is shut down again by its UDID; the samples it pushed are deleted
# by their exact paths. Without Xcode (ios) or without a device (android) it prints a SKIP line.
set -eu
cd "$(dirname "$0")/.."
patrol_cli_version=4.8.0
platform="${1:-}"
assets=integration_test/share/assets

patrol_cmd() {
  if command -v patrol >/dev/null 2>&1; then
    patrol "$@"
  else
    dart pub global activate patrol_cli "$patrol_cli_version" >/dev/null
    dart pub global run patrol_cli:main "$@"
  fi
}

case "$platform" in
ios)
  if ! xcrun --find simctl >/dev/null 2>&1 || ! xcrun --find xcodebuild >/dev/null 2>&1; then
    echo "SKIP client share ios: Xcode with the iOS simulator is not installed on this host"
    exit 0
  fi
  udid=$(xcrun simctl list devices available | sed -n 's/^ *iPhone[^(]*(\([0-9A-F-]\{36\}\)).*/\1/p' | head -n 1)
  if [ -z "$udid" ]; then
    echo "client share ios: no available iPhone simulator (xcrun simctl list devices available)" >&2
    exit 1
  fi
  booted=0
  if ! xcrun simctl list devices | grep "$udid" | grep -q "(Booted)"; then
    xcrun simctl boot "$udid"
    booted=1
  fi
  trap 'if [ "$booted" = 1 ]; then xcrun simctl shutdown "$udid"; fi' EXIT
  xcrun simctl bootstatus "$udid" -b >/dev/null
  xcrun simctl addmedia "$udid" "$assets/share-sample.jpg"
  # One patrol run per test file: each starts the app afresh. Photos starts afresh too, so a
  # share sheet left open by an earlier run hides nothing. --verbose: without it patrol prints
  # only "** TEST BUILD FAILED **" of a failed xcodebuild, not its errors (xcodebuild runs with
  # -quiet, so the build adds only its warnings and errors).
  for test in share_image_from_photos_test share_offline_test; do
    xcrun simctl terminate "$udid" com.apple.mobileslideshow >/dev/null 2>&1 || true
    CLIENTKIT_SIMULATOR_GROUPS=YES patrol_cmd --verbose test -d "$udid" \
      --target "integration_test/share/$test.dart"
  done
  ;;
android)
  serial=${ANDROID_SERIAL:-$(adb devices | awk 'NR > 1 && $2 == "device" { print $1; exit }')}
  if [ -z "$serial" ]; then
    echo "SKIP client share android: no device in state 'device' (adb devices)"
    exit 0
  fi
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/share-samples.XXXXXX")
  app_id=$(sed -n 's/^applicationId=//p' android/branding.properties)
  trap 'rm -rf "$tmp"; for f in share-sample.jpg share-sample.txt share-sample.pdf share-sample.bin; do adb -s "$serial" shell rm -f "/sdcard/Download/$f"; done; adb -s "$serial" uninstall "$app_id" >/dev/null 2>&1; adb -s "$serial" uninstall "$app_id.test" >/dev/null 2>&1; true' EXIT
  printf 'Share sample text\n' >"$tmp/share-sample.txt"
  printf 'not a known type\n' >"$tmp/share-sample.bin"
  for f in "$assets/share-sample.jpg" "$assets/share-sample.pdf" "$tmp/share-sample.txt" "$tmp/share-sample.bin"; do
    adb -s "$serial" push "$f" /sdcard/Download/ >/dev/null
    name=$(basename "$f")
    adb -s "$serial" shell am broadcast -a android.intent.action.MEDIA_SCANNER_SCAN_FILE \
      -d "file:///sdcard/Download/$name" >/dev/null
  done
  # One patrol run per test file: each starts the app afresh. The file app lists the downloads
  # as it last loaded them, so it starts afresh too and shows the samples.
  # android/app/build.gradle.kts restricts the run to the patrol bridge; the plain
  # instrumented tests of androidTest run in tool/run_android_tests.sh.
  for test in share_image_from_photos_test share_android_mime_test; do
    adb -s "$serial" shell am force-stop com.google.android.documentsui >/dev/null 2>&1 || true
    adb -s "$serial" shell am force-stop com.android.documentsui >/dev/null 2>&1 || true
    # --no-uninstall: on API 26 the Gradle test run uninstalls the app already, and patrol's own
    # uninstall afterwards fails the run; the trap removes the app at the end.
    patrol_cmd test -d "$serial" --no-uninstall --target "integration_test/share/$test.dart"
  done
  ;;
*)
  echo "usage: tool/run_share_device_tests.sh ios|android" >&2
  exit 2
  ;;
esac
