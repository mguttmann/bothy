#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Starts (or restarts) the computer helper's LaunchAgent in the GUI session of one user
# (docs/spec/27-computer-use.md 9.3). The helper package's postinstall runs it as root for the
# user at the console; an update runs it again so the new build replaces the running one.
#
#   sh start-computer-helper.sh <uid> <LaunchAgent plist>
#
# launchctl bootout returns before launchd has removed the service, and a bootstrap in that time
# fails with "Bootstrap failed: 5: Input/output error" (seen live on 26.11. at an update, which
# left the helper stopped). So the script waits until the service is gone, then bootstraps, and
# tries again a few times; a service that never goes away is restarted with kickstart -k.
# Prints one line; exit 0 when the helper runs, 1 otherwise.
set -u

uid=${1:?uid}
plist=${2:?plist}
label=com.example.bothy-computer-helper
service="gui/$uid/$label"

launchctl bootout "$service" 2>/dev/null || true
i=0
while launchctl print "$service" >/dev/null 2>&1 && [ "$i" -lt 50 ]; do
  sleep 0.2
  i=$((i + 1))
done

try=0
while :; do
  if launchctl bootstrap "gui/$uid" "$plist" 2>/dev/null; then
    echo "start-computer-helper: started in gui/$uid"
    exit 0
  fi
  try=$((try + 1))
  if [ "$try" -ge 5 ]; then
    if launchctl print "$service" >/dev/null 2>&1 && launchctl kickstart -k "$service" 2>/dev/null; then
      echo "start-computer-helper: restarted in gui/$uid"
      exit 0
    fi
    echo "start-computer-helper: could not start the helper in gui/$uid"
    exit 1
  fi
  sleep 1
done
