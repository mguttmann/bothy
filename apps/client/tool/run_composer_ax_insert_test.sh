#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs integration_test/composer_ax_insert_test.dart on macOS (platform-tests.json,
# FIX-macos-ax-text-insert). When this process may use the Accessibility API, the writer of
# another process (macos/AxWriteProbe/ax_write_probe.swift) runs beside the test and the test's
# cross-process case expects its write in the composer; without that permission the case
# reports itself skipped and the cross-process path stays unproven on this host.
set -eu
cd "$(dirname "$0")/.."
out=$(mktemp -d)
watcher=
trap 'status=$?; if [ -n "$watcher" ]; then kill "$watcher" 2>/dev/null || true; cat "$out/probe.log" 2>/dev/null || true; fi; rm -rf "$out"; exit $status' EXIT
swiftc -O -o "$out/ax_write_probe" macos/AxWriteProbe/ax_write_probe.swift
if "$out/ax_write_probe" check; then
  # The marker and the text match axCrossProcessMarker and axCrossProcessText of the test.
  "$out/ax_write_probe" watch 'axprobe|' 'Hallo' 3600 >"$out/probe.log" 2>&1 &
  watcher=$!
  flutter test integration_test/composer_ax_insert_test.dart -d macos --dart-define=BOTHY_AX_CROSS_PROCESS=true
else
  echo "client composer-ax-insert: no Accessibility permission for this process, the cross-process write stays unproven"
  flutter test integration_test/composer_ax_insert_test.dart -d macos
fi
