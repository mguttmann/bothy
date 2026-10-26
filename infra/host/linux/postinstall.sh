#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# After the bothy-host package is installed or upgraded: let a running systemd reload its units.
# Nothing is started or enabled and nothing pairs (docs/spec/16-operations.md 4.4); a running host
# service keeps its old binary until "bothy-host restart".
set -eu
if [ -d /run/systemd/system ]; then
  systemctl daemon-reload || true
fi
exit 0
