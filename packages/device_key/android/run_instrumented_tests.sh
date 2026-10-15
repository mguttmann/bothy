#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the instrumented tests of devicekeycore on the one attached device or emulator
# (brief M5-01, Design note 5; platform-tests.json, `make apps-platform`, CI job apps-android).
# tools/ci/apps_platform.py has already checked that `adb devices` lists exactly one device.
#
# The step-up key needs a secure lock screen. On an emulator without one the script sets a
# temporary PIN with `locksettings` and clears it at the end, also after a failure; on a physical
# device it never touches the lock screen. Exit code: Gradle's.
set -eu

cd "$(dirname "$0")"

# set-pin fails when a PIN exists already; then the lock screen stays as it is.
if [ "$(adb shell getprop ro.kernel.qemu | tr -d '\r')" = "1" ] && \
   adb shell locksettings set-pin 0000 >/dev/null 2>&1; then
  trap 'adb shell locksettings clear --old 0000 >/dev/null 2>&1 || true' EXIT
fi

./gradlew --no-daemon --console=plain :devicekeycore:connectedDebugAndroidTest
