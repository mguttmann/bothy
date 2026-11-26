#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Sets host_service.computer_use to true in the configuration file of the host service
# (docs/spec/27-computer-use.md 9.3, docs/decisions.d/CU-02.md D-CU-02-10). The macOS package runs
# it when the person installing selects the computer helper component; the Windows MSI runs
# windows/set-computer-use.ps1, which does the same.
#
#   set-computer-use.sh <configuration file>
#
# A missing or empty file becomes "host_service:" with the key. Otherwise the key goes in as the
# first child of the top-level mapping host_service, with the indentation of its other children, or
# an existing computer_use child of that mapping gets the value true; comments and every other line
# stay. A file without host_service gets the mapping appended. A host_service in flow style
# ({...}) is left alone with exit code 3: the person edits it by hand. The file keeps its owner and
# mode, because the script writes through it instead of replacing it.
set -eu

cfg="${1:?usage: set-computer-use.sh <configuration file>}"

if [ ! -s "$cfg" ]; then
  printf 'host_service:\n  computer_use: true\n' > "$cfg"
  exit 0
fi

tmp="$(mktemp "${cfg}.XXXXXX")"
trap 'rm -f "$tmp"' EXIT

# Two passes over the file: the first finds the line of host_service, the indentation of its
# children and an existing computer_use child; the second writes the result.
status=0
awk '
  function spaces(n,   s) { s = ""; while (n-- > 0) s = s " "; return s }
  NR == FNR {
    if (!found && $0 ~ /^host_service:[ \t]*(#.*)?$/) { found = FNR; inblock = 1; next }
    if (!found && $0 ~ /^host_service:/) { flow = 1 }
    if (inblock) {
      if ($0 ~ /^[ \t]*(#.*)?$/) next
      match($0, /^ */)
      if (RLENGTH == 0) { inblock = 0; next }
      if (!child) child = RLENGTH
      if (RLENGTH == child && $0 ~ /^ *computer_use:/) cu = FNR
    }
    next
  }
  flow { failed = 1; exit }
  {
    if (FNR == cu) { print spaces(child) "computer_use: true"; next }
    print
    if (FNR == found && !cu) print spaces(child ? child : 2) "computer_use: true"
  }
  END {
    if (failed) exit 3
    if (!found) { print "host_service:"; print "  computer_use: true" }
  }
' "$cfg" "$cfg" > "$tmp" || status=$?

if [ "$status" -ne 0 ]; then
  echo "set-computer-use: host_service in $cfg is not a block mapping; set host_service.computer_use: true by hand" >&2
  exit "$status"
fi
cat "$tmp" > "$cfg"
