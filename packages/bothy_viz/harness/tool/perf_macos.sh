#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# The performance test of the live topology on macOS in profile mode (brief M6-12, design
# note 6): platform-tests.json (`make apps-platform`) and `make perf` call it.
#
# The platform folders of the harness are not checked in (docs/DECISIONS.md D-M6-11-6), so the
# script makes macos/ with `flutter create` for the run. That command and `flutter drive` also
# write a sample test, a member lock file, metadata and IDE files, and `flutter create` adds an
# exclude to analysis_options.yaml. Everything the run added is removed and the analysis options
# are restored afterwards, also after a failed run: a native folder under packages/ fails rule 4
# of `make arch`, and the sample test would break `flutter test`. The exit code is the one of
# `flutter drive`.
set -eu
cd "$(dirname "$0")/.."
added=""
for f in macos test/widget_test.dart pubspec.lock .metadata .gitignore bothy_viz_harness.iml \
  .idea .flutter-plugins-dependencies; do
  [ -e "$f" ] || added="$added $f"
done
options=$(mktemp)
cp analysis_options.yaml "$options"
cleanup() {
  for f in $added; do
    rm -rf "./$f"
  done
  cp "$options" analysis_options.yaml
  rm -f "$options"
}
trap cleanup EXIT
flutter create --platforms=macos --no-pub .
status=0
flutter drive --profile -d macos \
  --driver=test_driver/integration_test.dart \
  --target=integration_test/topology_perf_test.dart || status=$?
exit "$status"
