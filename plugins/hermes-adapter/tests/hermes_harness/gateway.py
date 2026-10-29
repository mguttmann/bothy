# SPDX-License-Identifier: Apache-2.0
"""Start and stop of the Hermes gateway as its own process group.

:class:`HermesGateway` starts the foreground gateway command of the pinned CLI by absolute
path in a new session (``start_new_session=True``), waits for the probe's ``connect`` within
:data:`~hermes_harness.limits.GATEWAY_START_DEADLINE_S`, lists the listening TCP and UDP
sockets of its process group, and stops exactly that group by its ID: ``SIGTERM``, then
``SIGKILL`` after :data:`~hermes_harness.limits.STOP_GRACE_S`. It never signals a process by
name and never a process outside the group.

Example:
    >>> gateway = HermesGateway(cmd, env=env, cwd=homes.work, log=root / "gateway.log")
    >>> gateway.start()
    >>> gateway.wait_connected(control)
    >>> assert unexpected_listeners(gateway.listening_sockets()) == []
    >>> gateway.stop()
"""

from __future__ import annotations

import contextlib
import os
import signal
import subprocess
import sys
import time
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import IO, Any

from hermes_harness.control import ControlServer
from hermes_harness.limits import GATEWAY_START_DEADLINE_S, STOP_GRACE_S

_POLL_S = 0.1


class GatewayError(RuntimeError):
    """The gateway did not connect in time, exited early or left a process behind."""


class HermesGateway:
    """One gateway process group of the harness."""

    def __init__(
        self, command: Sequence[str], *, env: Mapping[str, str], cwd: Path, log: Path
    ) -> None:
        """Prepare the start; nothing runs yet.

        Args:
            command: The full command line, the executable by absolute path.
            env: The complete environment (from the allowlist).
            cwd: The working directory.
            log: File that receives stdout and stderr of the group (inside the temporary root).
        """
        self.command = list(command)
        self.env = dict(env)
        self.cwd = cwd
        self.log = log
        self._process: subprocess.Popen[bytes] | None = None
        self._log_handle: IO[bytes] | None = None
        self.pgid: int | None = None

    @property
    def pid(self) -> int:
        """PID of the group leader."""
        return self._started().pid

    @property
    def returncode(self) -> int | None:
        """Exit code of the leader once it ended."""
        return self._process.returncode if self._process is not None else None

    def _started(self) -> subprocess.Popen[bytes]:
        if self._process is None:
            raise GatewayError("gateway not started")
        return self._process

    def start(self) -> None:
        """Start the command in a new session and process group."""
        self.log.parent.mkdir(parents=True, exist_ok=True)
        self._log_handle = self.log.open("ab")
        self._process = subprocess.Popen(
            self.command,
            env=self.env,
            cwd=self.cwd,
            stdin=subprocess.DEVNULL,
            stdout=self._log_handle,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
        self.pgid = self._process.pid

    def running(self) -> bool:
        """Whether the group leader still runs."""
        return self._started().poll() is None

    def wait_connected(
        self, control: ControlServer, *, deadline_s: float = GATEWAY_START_DEADLINE_S
    ) -> dict[str, Any]:
        """Wait for the probe's ``connect`` call.

        Args:
            control: The control channel the probe reports to.
            deadline_s: The named start deadline.

        Returns:
            The ``connect`` call.

        Raises:
            GatewayError: The leader exited first, or no ``connect`` arrived in time.
        """
        process = self._started()
        end = time.monotonic() + deadline_s
        while True:
            try:
                return control.wait_for("connect", deadline_s=_POLL_S)
            except TimeoutError:
                pass
            code = process.poll()
            if code is not None:
                raise GatewayError(f"gateway exited with {code} before connect (see {self.log})")
            if time.monotonic() >= end:
                raise GatewayError(f"no connect within the gateway start deadline (see {self.log})")

    def members(self) -> list[int]:
        """PIDs of the running members of the group (zombies excluded).

        Raises:
            GatewayError: The gateway was never started.
        """
        self._started()
        out = subprocess.run(
            ["ps", "-A", "-o", "pid=,pgid=,stat="], capture_output=True, text=True, check=True
        ).stdout
        found = []
        for line in out.splitlines():
            parts = line.split()
            if len(parts) >= 3 and parts[1] == str(self.pgid) and not parts[2].startswith("Z"):
                found.append(int(parts[0]))
        return sorted(found)

    def listening_sockets(self) -> list[str]:
        """Listening TCP and bound, unconnected UDP sockets of the group.

        Returns:
            Entries ``"tcp <address>"`` or ``"udp <address>"``; empty when none listens.
        """
        pids = self.members()
        if not pids:
            return []
        if sys.platform.startswith("linux"):
            return _linux_listeners(pids)
        return _lsof_listeners(pids)

    def stop(self) -> None:
        """End the process group: ``SIGTERM``, ``SIGKILL`` after the grace, then verify.

        Raises:
            GatewayError: A member of the group survived ``SIGKILL``.
        """
        if self._process is None:
            return
        process = self._process
        assert self.pgid is not None  # noqa: S101 - set with the process
        with contextlib.suppress(ProcessLookupError):
            os.killpg(self.pgid, signal.SIGTERM)
        end = time.monotonic() + STOP_GRACE_S
        while time.monotonic() < end and (process.poll() is None or self.members()):
            time.sleep(_POLL_S)
        if process.poll() is None or self.members():
            with contextlib.suppress(ProcessLookupError):
                os.killpg(self.pgid, signal.SIGKILL)
        with contextlib.suppress(subprocess.TimeoutExpired):
            process.wait(STOP_GRACE_S)
        end = time.monotonic() + STOP_GRACE_S
        while self.members() and time.monotonic() < end:
            time.sleep(_POLL_S)
        if self._log_handle is not None:
            self._log_handle.close()
            self._log_handle = None
        survivors = self.members()
        if survivors or process.poll() is None:
            raise GatewayError(f"process group {self.pgid} survived SIGKILL: {survivors}")


def unexpected_listeners(listeners: Sequence[str]) -> list[str]:
    """The listeners of :meth:`HermesGateway.listening_sockets` that fail a run.

    Hermes v0.22 always serves its local runtime API on one TCP port of ``127.0.0.1`` (clients
    attach with a ticket of its control socket; no setting turns it off). Exactly one such
    listener is Hermes's own; any other listener, and every loopback one when there are more
    than one, is unexpected.

    Args:
        listeners: Entries ``"tcp <address>"`` or ``"udp <address>"``.

    Returns:
        The unexpected entries, in the given order.
    """
    loopback = [s for s in listeners if s.startswith("tcp 127.0.0.1:")]
    allowed = set(loopback) if len(loopback) == 1 else set()
    return [s for s in listeners if s not in allowed]


def _lsof_listeners(pids: list[int]) -> list[str]:
    found: list[str] = []
    joined = ",".join(str(p) for p in pids)
    for proto, extra in (("tcp", ["-iTCP", "-sTCP:LISTEN"]), ("udp", ["-iUDP"])):
        result = subprocess.run(
            ["lsof", "-nP", "-a", "-p", joined, *extra, "-Fn"],
            capture_output=True,
            text=True,
            check=False,
        )
        for line in result.stdout.splitlines():
            if line.startswith("n") and "->" not in line:
                found.append(f"{proto} {line[1:]}")
    return sorted(set(found))


def _hex_address(text: str) -> str:
    host, _, port = text.partition(":")
    raw = bytes.fromhex(host)
    if len(raw) == 4:
        address = ".".join(str(b) for b in reversed(raw))
    else:
        words = [raw[i : i + 4][::-1].hex() for i in range(0, len(raw), 4)]
        address = ":".join(words)
    return f"{address}:{int(port, 16)}"


def _linux_listeners(pids: list[int]) -> list[str]:
    inodes: set[str] = set()
    for pid in pids:
        fd_dir = Path(f"/proc/{pid}/fd")
        with contextlib.suppress(OSError):
            for fd in fd_dir.iterdir():
                with contextlib.suppress(OSError):
                    target = str(fd.readlink())
                    if target.startswith("socket:["):
                        inodes.add(target[8:-1])
    found: list[str] = []
    for proto in ("tcp", "tcp6", "udp", "udp6"):
        table = Path(f"/proc/{pids[0]}/net/{proto}")
        if not table.is_file():
            continue
        for row in table.read_text().splitlines()[1:]:
            cols = row.split()
            if len(cols) < 10 or cols[9] not in inodes:
                continue
            listening = cols[3] == "0A" if proto.startswith("tcp") else set(cols[2]) <= set("0:")
            if listening:
                found.append(f"{proto[:3]} {_hex_address(cols[1])}")
    return sorted(set(found))
