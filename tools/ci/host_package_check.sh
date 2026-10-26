#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Installs one package of the host service in a throwaway container and checks what an operator
# gets (brief M7-23 Design note 1, docs/spec/16-operations.md 4.4); run by
# `make release-trial COMPONENT=host` (tools/ci/release_trial.py) as
#   docker run --rm --network none -v <pkg>:/pkg/<name>:ro -v <this>:/check.sh:ro <image> \
#     /check.sh deb|rpm /pkg/<name> <version>
# Checks: the binary, its folders and the reference unit belong to root and only root writes them,
# the binary prints the version, the package created no user, wrote no live unit and enabled or
# started no service, the package owns binary and reference, an upgrade keeps the live unit that
# bothy-host install wrote (with another --config), and removing the package takes binary and
# reference and keeps the state folder, the configuration file and the live unit.
set -eu
kind=$1
pkg=$2
version=$3
binary=/opt/bothy-host/bin/bothy-host
unit=/opt/bothy-host/share/bothy-host.service
live=/etc/systemd/system/bothy-host.service

fail() {
  echo "package check: $kind: $*" >&2
  exit 1
}

expect() {
  got=$(stat -c '%U:%G %a' "$1")
  [ "$got" = "$2" ] || fail "$1 is $got, want $2"
  echo "  $1 $got"
}

case "$kind" in
  deb) dpkg -i "$pkg" >/dev/null ;;
  rpm) rpm -i "$pkg" ;;
  *) fail "unknown package kind" ;;
esac

expect /opt/bothy-host "root:root 755"
expect /opt/bothy-host/bin "root:root 755"
expect /opt/bothy-host/share "root:root 755"
expect "$binary" "root:root 755"
expect "$unit" "root:root 644"
[ "$("$binary" version)" = "$version" ] || fail "the binary prints $("$binary" version), want $version"
if getent passwd bothy-host >/dev/null; then fail "the package created the service user"; fi
[ ! -e "$live" ] || fail "the package wrote the live unit that bothy-host install writes"
[ ! -e /etc/systemd/system/multi-user.target.wants/bothy-host.service ] || fail "the package enabled the service"
for comm in /proc/[0-9]*/comm; do
  if [ "$(cat "$comm" 2>/dev/null || true)" = bothy-host ]; then fail "a bothy-host process runs"; fi
done
case "$kind" in
  deb) dpkg -S "$binary" "$unit" >/dev/null || fail "the package does not own binary and unit" ;;
  rpm) rpm -qf "$binary" "$unit" >/dev/null || fail "the package does not own binary and unit" ;;
esac

# What bothy-host install leaves behind: the state folder, the configuration and the live unit,
# here with another configuration path (install --config). An upgrade (the package again) keeps it.
mkdir -p /var/lib/bothy-host /etc/bothy-host
echo state >/var/lib/bothy-host/marker
echo "host_service: {}" >/etc/bothy-host/config.yaml
mkdir -p /etc/systemd/system
sed 's|run --config /etc/bothy-host/config.yaml|run --config /srv/bothy-host/config.yaml|' "$unit" >"$live"
grep -q "run --config /srv/bothy-host/config.yaml" "$live" || fail "the test unit has no other configuration path"
before=$(cat "$live")
case "$kind" in
  deb) dpkg -i "$pkg" >/dev/null ;;
  rpm) rpm -U --replacepkgs "$pkg" ;;
esac
[ "$(cat "$live")" = "$before" ] || fail "the upgrade changed the unit of bothy-host install"
echo "  upgrade keeps $live (run --config /srv/bothy-host/config.yaml)"
case "$kind" in
  deb) dpkg -r bothy-host >/dev/null ;;
  rpm) rpm -e bothy-host ;;
esac
[ ! -e "$binary" ] || fail "the removal left the binary"
[ ! -e "$unit" ] || fail "the removal left the reference unit"
[ -f /var/lib/bothy-host/marker ] || fail "the removal took the state folder"
[ -f /etc/bothy-host/config.yaml ] || fail "the removal took the configuration"
[ -f "$live" ] || fail "the removal took the unit of bothy-host install"
echo "package check: $kind ok"
