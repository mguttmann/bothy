# SPDX-License-Identifier: Apache-2.0
"""The no-network guard of the tests (brief M2-50 Design note 19).

`install_guard` replaces ``socket.getaddrinfo``, ``socket.socket.connect`` and
``socket.socket.connect_ex``: a host other than ``127.0.0.1``, ``::1`` or ``localhost`` and an
address outside ``127.0.0.0/8`` and ``::1`` raise `NetworkBlocked`; Unix sockets pass.
"""

from __future__ import annotations

import ipaddress
import socket
from typing import Any

import pytest

LOOPBACK_NAMES = frozenset({"127.0.0.1", "::1", "localhost"})


class NetworkBlockedError(OSError):
    """A test tried to leave the loopback host."""


NetworkBlocked = NetworkBlockedError


def is_loopback_address(host: object) -> bool:
    """Whether a host or address may be reached by a test."""
    if not isinstance(host, str):
        return False
    if host in LOOPBACK_NAMES:
        return True
    try:
        return ipaddress.ip_address(host.split("%", 1)[0]).is_loopback
    except ValueError:
        return False


def _check_address(address: Any, family: int) -> None:
    if family == getattr(socket, "AF_UNIX", None):
        return
    host = address[0] if isinstance(address, tuple) else address
    if not is_loopback_address(host):
        raise NetworkBlocked("tests may only connect to the loopback host")


def install_guard(monkeypatch: pytest.MonkeyPatch) -> None:
    """Install the guard on the socket module for one test."""
    real_getaddrinfo = socket.getaddrinfo
    real_connect = socket.socket.connect
    real_connect_ex = socket.socket.connect_ex

    def getaddrinfo(host: Any, *args: Any, **kwargs: Any) -> Any:
        name = host.decode() if isinstance(host, bytes) else host
        if name is not None and not is_loopback_address(name):
            raise NetworkBlocked("tests may only resolve the loopback host")
        return real_getaddrinfo(host, *args, **kwargs)

    def connect(self: socket.socket, address: Any) -> None:
        _check_address(address, self.family)
        real_connect(self, address)

    def connect_ex(self: socket.socket, address: Any) -> int:
        _check_address(address, self.family)
        return real_connect_ex(self, address)

    monkeypatch.setattr(socket, "getaddrinfo", getaddrinfo)
    monkeypatch.setattr(socket.socket, "connect", connect)
    monkeypatch.setattr(socket.socket, "connect_ex", connect_ex)
