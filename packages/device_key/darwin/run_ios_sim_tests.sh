#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the tests of DeviceKeyCore in the iOS simulator (platform-tests.json, brief M4-10, note 5).
# The simulator has no Secure Enclave, so this run proves the keychain fallback on iOS. The first
# available iPhone of `xcrun simctl list` is used, so no device name is written down here.
# Without Xcode there is no simulator: the script prints a SKIP line and exits 0; the apps-ios
# CI job (macos-15 with Xcode) runs it for real (docs/DECISIONS.md D-M4-10-5, PL/44).
set -eu
cd "$(dirname "$0")/device_key/DeviceKeyCore"
if ! xcrun --find simctl >/dev/null 2>&1 || ! xcrun --find xcodebuild >/dev/null 2>&1; then
  echo "SKIP device_key ios-simulator: Xcode with the iOS simulator is not installed on this host"
  exit 0
fi
name=$(xcrun simctl list devices available | sed -n 's/^ *\(iPhone[^(]*\) (.*/\1/p' | head -n 1 | sed 's/ *$//')
if [ -z "$name" ]; then
  echo "device_key ios-simulator: no available iPhone simulator (xcrun simctl list devices available)" >&2
  exit 1
fi
derived=$(mktemp -d)
trap 'rm -rf "$derived"' EXIT
xcodebuild test \
  -scheme DeviceKeyCore \
  -destination "platform=iOS Simulator,OS=latest,name=$name" \
  -derivedDataPath "$derived" \
  -quiet
