#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# The performance test of the live topology on macOS in profile mode (brief M6-12, design
# note 6): platform-tests.json (`make apps-platform`) and `make perf` call it.
#
# The platform folders of the harness are not checked in (docs/DECISIONS.md D-M6-11-6), so the
# first run makes macos/ with `flutter create`. That command also writes a sample test, a
# member lock file and its own metadata next to it; they would break the workspace, so the
# ones it added are removed again. macos/ stays and is ignored by git.
set -eu
cd "$(dirname "$0")/.."
if [ ! -d macos ]; then
  added=""
  for f in test/widget_test.dart pubspec.lock .metadata .gitignore bothy_viz_harness.iml; do
    [ -e "$f" ] || added="$added $f"
  done
  flutter create --platforms=macos --no-pub .
  for f in $added; do
    rm -f "./$f"
  done
fi
exec flutter drive --profile -d macos \
  --driver=test_driver/integration_test.dart \
  --target=integration_test/topology_perf_test.dart
