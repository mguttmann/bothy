#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs integration_test/legacy_keychain_test.dart on macOS (platform-tests.json,
# FIX-macos-legacy-keychain-prompt): the move of the old secrets against the real login keychain.
# Before the test an old item goes into the login keychain whose access list trusts only another
# program (/usr/bin/true), as the old items of the live case trust only the build of the old
# bundle identifier; a plain read of it from the app shows a keychain dialog. The item is removed
# again when the script ends.
set -eu
cd "$(dirname "$0")/.."
instance="legacy-keychain-foreign-$$-$(date +%s)"
account="instance/$instance/database_key"
trap 'security delete-generic-password -s flutter_secure_storage_service -a "$account" >/dev/null 2>&1 || true' EXIT
security add-generic-password -s flutter_secure_storage_service -a "$account" -w old -T /usr/bin/true
flutter test integration_test/legacy_keychain_test.dart -d macos --dart-define=BOTHY_FOREIGN_INSTANCE="$instance"
