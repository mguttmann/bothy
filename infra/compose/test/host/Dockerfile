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
# strace, gdb, python3-minimal and netcat-openbsd serve the platform suite of the host service
# (services/host/test/platform, run by e2e/hostplatform): the system call trace of TM-T25, the
# attach attempt and the impostor on the helper endpoint of TM-T26, and the connect attempt on the
# admin endpoint. Without them those checks fail or prove nothing.
FROM debian:trixie-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f

ARG SNAPSHOT=20261015T000000Z
RUN rm -f /etc/apt/sources.list.d/debian.sources \
    && printf 'deb http://snapshot.debian.org/archive/debian/%s trixie main\ndeb http://snapshot.debian.org/archive/debian/%s trixie-updates main\ndeb http://snapshot.debian.org/archive/debian-security/%s trixie-security main\n' "$SNAPSHOT" "$SNAPSHOT" "$SNAPSHOT" > /etc/apt/sources.list \
    && printf 'Acquire::Check-Valid-Until "false";\nAcquire::Retries "5";\n' > /etc/apt/apt.conf.d/99snapshot \
    && apt-get update -qq \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends \
       systemd systemd-sysv dbus sudo cron procps acl iproute2 ca-certificates passwd util-linux \
       strace gdb python3-minimal netcat-openbsd \
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
