#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# After the bothy-host package is removed: let a running systemd reload its units. The state
# folder, the configuration file and the units that bothy-host install wrote stay.
set -eu
if [ -d /run/systemd/system ]; then
  systemctl daemon-reload || true
fi
exit 0
