#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs integration_test/legacy_keychain_test.dart on macOS (platform-tests.json,
# FIX-macos-legacy-keychain-prompt, FIX-macos-migration-denied): the move of the old secrets
# against the real login keychain. Before the test old items go into the login keychain whose
# access list trusts only another program (/usr/bin/true), as the old items of the live cases trust
# only the build of an older bundle identifier; a plain read of one from the app shows a keychain
# dialog. One is the database key of an instance of its own; the other is `app/instance` of the
# owner's case of 2026-11-01, which the build before the app scope never read at a normal start.
# `app/instance` and `app/server_address` have fixed names, so the owner's case runs only on a
# keychain that has neither (a CI runner); on a machine with them it is skipped and nothing of
# them is touched. The items are removed again when the script ends.
set -eu
cd "$(dirname "$0")/.."
service=flutter_secure_storage_service
instance="legacy-keychain-foreign-$$-$(date +%s)"
account="instance/$instance/database_key"
owner=false
cleanup() {
  security delete-generic-password -s "$service" -a "$account" >/dev/null 2>&1 || true
  if [ "$owner" = true ]; then
    security delete-generic-password -s "$service" -a app/instance >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
security add-generic-password -s "$service" -a "$account" -w old -T /usr/bin/true
# Attributes only (no -g or -w): no access check, no dialog.
if security find-generic-password -s "$service" -a app/instance >/dev/null 2>&1 ||
  security find-generic-password -s "$service" -a app/server_address >/dev/null 2>&1; then
  echo "app/instance or app/server_address exists in this keychain: the owner's case is skipped"
else
  owner=true
  security add-generic-password -s "$service" -a app/instance -w '{}' -T /usr/bin/true
fi
flutter test integration_test/legacy_keychain_test.dart -d macos --dart-define=BOTHY_FOREIGN_INSTANCE="$instance" --dart-define=BOTHY_OWNER_STATE="$owner"
