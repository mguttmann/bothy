#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the checks of Runner/LegacyKeychain.swift against a real keychain (apps/client/
# platform-tests.json, FIX-macos-legacy-keychain-prompt). The keychain is a file of its own in a
# temporary folder, never in the search list of the user; its items are written with the
# `security` tool: one whose access list trusts only another program (/usr/bin/true, as the old
# items trust only the build of the old bundle identifier) and one that trusts the checks
# binary (an item the app may read). Needs only the Swift compiler of the command line tools.
set -eu
cd "$(dirname "$0")"
out=$(mktemp -d)
keychain="$out/legacy.keychain-db"
trap 'security delete-keychain "$keychain" 2>/dev/null || true; rm -rf "$out"' EXIT
swiftc -O -o "$out/checks" ../Runner/LegacyKeychain.swift main.swift
security create-keychain -p checks "$keychain"
security unlock-keychain -p checks "$keychain"
# No automatic lock while the checks run.
security set-keychain-settings "$keychain"
service=flutter_secure_storage_service
security add-generic-password -s "$service" -a app/instance -w old -T /usr/bin/true "$keychain"
security add-generic-password -s "$service" -a app/server_address -w https://chat.example.com \
  -T "$out/checks" "$keychain"
"$out/checks" "$keychain"
