#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Builds and runs the checks of Runner/AccessibilityTextInput.swift against a stand-in of the
# engine's text input plugin (apps/client/platform-tests.json, FIX-macos-ax-text-insert). Needs
# only the Swift compiler of the command line tools; no window, no engine.
set -eu
cd "$(dirname "$0")"
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
swiftc -O -o "$out/checks" ../Runner/AccessibilityTextInput.swift main.swift
"$out/checks"
