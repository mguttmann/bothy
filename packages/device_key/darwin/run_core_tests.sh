#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Runs the tests of the Swift package DeviceKeyCore on this Mac (platform-tests.json, brief M4-10).
# With Xcode, `swift test` finds Swift Testing by itself. The command line tools of a Mac without
# Xcode ship Swift Testing too, but outside the default search paths; then the paths are passed,
# so a Mac with an Apple chip can prove the Secure Enclave path without Xcode (D-M4-10-6).
set -eu
cd "$(dirname "$0")/device_key/DeviceKeyCore"
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
