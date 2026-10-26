#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Before the bothy-host package is removed (dpkg: "remove", rpm: 0 packages left), stop and disable
# the host service, whose binary goes away with the package. An upgrade ("upgrade", rpm: 1)
# leaves the service alone.
set -eu
case "${1:-}" in
  remove | 0)
    if [ -d /run/systemd/system ]; then
      systemctl disable --now bothy-host.service >/dev/null 2>&1 || true
    fi
    ;;
esac
exit 0
