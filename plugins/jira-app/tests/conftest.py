# SPDX-License-Identifier: Apache-2.0
"""Shared fixtures of the app's tests: the no-network guard (brief M2-50 Design note 19).

The autouse fixture `no_network` replaces ``socket.getaddrinfo``, ``socket.socket.connect`` and
``socket.socket.connect_ex``: a host other than ``127.0.0.1``, ``::1`` or ``localhost`` and an
address outside ``127.0.0.0/8`` and ``::1`` raise `NetworkBlocked`; Unix sockets pass. No test
can reach a real Jira service or any other host.
"""

from __future__ import annotations

import pytest

from tests.guard import (
    LOOPBACK_NAMES,
    NetworkBlocked,
    NetworkBlockedError,
    install_guard,
    is_loopback_address,
)

__all__ = [
    "LOOPBACK_NAMES",
    "NetworkBlocked",
    "NetworkBlockedError",
    "install_guard",
    "is_loopback_address",
]


@pytest.fixture(autouse=True)
def no_network(monkeypatch: pytest.MonkeyPatch) -> None:
    """Refuse every connection off the loopback host for the whole test."""
    install_guard(monkeypatch)


@pytest.fixture
def anyio_backend() -> str:
    """Async tests run on asyncio (the anyio pytest plugin of httpx)."""
    return "asyncio"
