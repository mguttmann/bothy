#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the tests of the Swift package ClientKit on this Mac (apps/client/platform-tests.json,
# brief M5-04). The same form as packages/device_key/darwin/run_core_tests.sh: with Xcode
# `swift test` finds Swift Testing by itself; the command line tools of a Mac without Xcode ship
# Swift Testing outside the default search paths, so the paths are passed.
set -eu
cd "$(dirname "$0")"
developer=$(xcode-select -p 2>/dev/null || true)
frameworks="$developer/Library/Developer/Frameworks"
if [ ! -d "$developer/Platforms" ] && [ -d "$frameworks/Testing.framework" ]; then
  libs="$developer/Library/Developer/usr/lib"
  exec swift test \
    -Xswiftc -F -Xswiftc "$frameworks" \
    -Xlinker -F -Xlinker "$frameworks" \
    -Xlinker -rpath -Xlinker "$frameworks" \
    -Xlinker -rpath -Xlinker "$libs" "$@"
fi
exec swift test "$@"
