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
# A helper the user started outside launchd (for example with `open`) survives the bootout; two
# helpers take the host's channel from each other and every computer session ends at once (seen
# live on 26.11.). So every process of the user that runs the helper's exact path is ended before
# the start, and the script checks that exactly one helper runs afterwards.
# Prints one line; exit 0 when exactly one helper runs, 1 otherwise.
set -u

uid=${1:?uid}
plist=${2:?plist}
label=com.example.bothy-computer-helper
service="gui/$uid/$label"
exe="/Library/Application Support/bothy-host/bin/bothy-computer-helper.app/Contents/MacOS/bothy-computer-helper"

# helpers prints the process ID of every process of the user that runs the helper's executable.
helpers() {
  ps -axo pid=,uid=,comm= | while read -r pid puid comm; do
    if [ "$puid" = "$uid" ] && [ "$comm" = "$exe" ]; then echo "$pid"; fi
  done
}

# started prints the result once exactly one helper runs, and fails after about ten seconds.
started() {
  n=0
  j=0
  while [ "$j" -lt 50 ]; do
    n=$(helpers | wc -l | tr -d ' ')
    if [ "$n" -eq 1 ]; then
      echo "start-computer-helper: $1 in gui/$uid"
      exit 0
    fi
    sleep 0.2
    j=$((j + 1))
  done
  echo "start-computer-helper: $n helpers run in gui/$uid, expected one"
  exit 1
}

launchctl bootout "$service" 2>/dev/null || true
i=0
while launchctl print "$service" >/dev/null 2>&1 && [ "$i" -lt 50 ]; do
  sleep 0.2
  i=$((i + 1))
done

left=$(helpers)
if [ -n "$left" ]; then
  # shellcheck disable=SC2086 # one process ID per word
  kill -TERM $left 2>/dev/null || true
  i=0
  while [ -n "$(helpers)" ] && [ "$i" -lt 25 ]; do
    sleep 0.2
    i=$((i + 1))
  done
  left=$(helpers)
  # shellcheck disable=SC2086 # one process ID per word
  if [ -n "$left" ]; then kill -KILL $left 2>/dev/null || true; fi
  i=0
  while [ -n "$(helpers)" ] && [ "$i" -lt 25 ]; do
    sleep 0.2
    i=$((i + 1))
  done
fi

try=0
while :; do
  if launchctl bootstrap "gui/$uid" "$plist" 2>/dev/null; then
    started started
  fi
  try=$((try + 1))
  if [ "$try" -ge 5 ]; then
    if launchctl print "$service" >/dev/null 2>&1 && launchctl kickstart -k "$service" 2>/dev/null; then
      started restarted
    fi
    echo "start-computer-helper: could not start the helper in gui/$uid"
    exit 1
  fi
  sleep 1
done
