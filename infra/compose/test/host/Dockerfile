# SPDX-License-Identifier: Apache-2.0
#
# Test only: the machine of a real host service bothy-host for the terminal conformance suite
# (brief M2-48 design note 5, services/server/test/terminal). systemd runs as process 1, so
# `bothy-host install` creates real users, units and sockets as on a Linux host (24 2.2). The
# image carries no bothy-host: the suite copies the binary built from the worktree into each
# container. Never part of an installation.
#
# The package set is reproducible: the APT sources point at one timestamp of snapshot.debian.org
# (20261015T000000Z); Check-Valid-Until is off because the Release files of a snapshot expire.
# snapshot.debian.org fails single fetches for a long time (HTTP 503 "No healthy backends" or
# "first byte timeout", 504, connection failures; one file failed for more than ten minutes), so
# `apt-get update` and `apt-get install` run in one bounded retry loop: three attempts on the
# snapshot, then three on the current trixie packages of deb.debian.org, announced on stderr and
# marked by /etc/termsuite-apt-fallback, which stack.HostImage reports and builds again
# (docs/decisions.d/FIX-apt-snapshot-retry.md). `--error-on=any` makes a failed index fail the
# attempt. APT's own retries are off: with them, APT 3.0
# repeats the warning "Tried to start delayed item" without end and grows until the kernel kills it.
# strace, gdb, python3-minimal and netcat-openbsd serve the platform suite of the host service
# (services/host/test/platform, run by e2e/hostplatform): the system call trace of TM-T25, the
# attach attempt and the impostor on the helper endpoint of TM-T26, and the connect attempt on the
# admin endpoint. Without them those checks fail or prove nothing.
FROM debian:trixie-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f

ARG SNAPSHOT=20261015T000000Z
RUN rm -f /etc/apt/sources.list.d/debian.sources \
    && printf 'deb http://snapshot.debian.org/archive/debian/%s trixie main\ndeb http://snapshot.debian.org/archive/debian/%s trixie-updates main\ndeb http://snapshot.debian.org/archive/debian-security/%s trixie-security main\n' "$SNAPSHOT" "$SNAPSHOT" "$SNAPSHOT" > /etc/apt/sources.list \
    && printf 'Acquire::Check-Valid-Until "false";\nAcquire::Retries "0";\n' > /etc/apt/apt.conf.d/99snapshot \
    && for attempt in 1 2 3 4 5 6; do \
         if [ "$attempt" = 4 ]; then \
           echo "termsuite host image: snapshot.debian.org failed 3 times, installing the current trixie packages of deb.debian.org" >&2; \
           printf 'deb http://deb.debian.org/debian trixie main\ndeb http://deb.debian.org/debian trixie-updates main\ndeb http://deb.debian.org/debian-security trixie-security main\n' > /etc/apt/sources.list; \
           touch /etc/termsuite-apt-fallback; \
         fi; \
         apt-get update -qq --error-on=any \
         && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends \
              systemd systemd-sysv dbus sudo cron procps acl iproute2 ca-certificates passwd util-linux \
              strace gdb python3-minimal netcat-openbsd \
         && break; \
         [ "$attempt" = 6 ] && exit 1; sleep $((attempt * 10)); \
       done \
    && rm -rf /var/lib/apt/lists/* \
    && systemctl mask getty@tty1 console-getty.service systemd-firstboot.service systemd-modules-load.service \
    && install -d -m 1777 /var/tmp/termsuite-markers \
    && printf 'termsuite: disposable test machine of the terminal suite\n' > /etc/termsuite-disposable

STOPSIGNAL SIGRTMIN+3
# The image's default user is unprivileged, so it never runs as root by accident. The test service
# termhost of compose.terminal.yaml starts it as root (user 0:0), because systemd is process 1 and
# bothy-host install creates users and units as root (24 2.2); Trivy's DS-0002 names no line, so
# an inline ignore cannot take it.
USER nobody
CMD ["/lib/systemd/systemd"]
