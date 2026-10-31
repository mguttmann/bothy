#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Builds and runs the tests of the header-only parts of the Windows runner with the host C++
# compiler (CXX, default c++), on any system: `make runner-test`, part of `make test` for the
# apps (docs/decisions.d/FIX-windows-dictation.md). The binaries go to a temporary folder.
set -eu
cd "$(dirname "$0")"
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
for t in *_test.cpp; do
  "${CXX:-c++}" -std=c++17 -Wall -Wextra -Werror -o "$out/${t%.cpp}" "$t"
  echo "$t"
  "$out/${t%.cpp}"
done
