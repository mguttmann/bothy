# SPDX-License-Identifier: Apache-2.0
"""No call to a real Jira service in the test (brief M2-50 Design note 19; PLAN row: "no call to
a real Jira service in the test").

The autouse guard of ``tests/conftest.py`` raises `NetworkBlocked` for ``jira.example.com``
through the sync and the async ``httpx`` client, for name resolution and for connections to any
non-loopback IPv4 or IPv6 address; loopback and Unix sockets keep working. The configuration
helper `make_config` refuses a non-loopback ``jira_base_url`` and ``server_url``.
"""

from __future__ import annotations

import shutil
import socket
import tempfile
from pathlib import Path

import httpx
import pytest

from tests.conftest import NetworkBlocked
from tests.helpers import make_config

JIRA_URL = "https://jira.example.com/"


def _chain(exc: BaseException) -> list[BaseException]:
    """The exception and every cause or context behind it."""
    seen: list[BaseException] = []
    current: BaseException | None = exc
    while current is not None and current not in seen:
        seen.append(current)
        current = current.__cause__ or current.__context__
    return seen


def _blocked_inside(exc: BaseException) -> bool:
    return any(isinstance(item, NetworkBlocked) for item in _chain(exc))


def test_sync_httpx_to_jira_is_blocked() -> None:
    with httpx.Client() as client, pytest.raises(httpx.HTTPError) as caught:
        client.get(JIRA_URL)
    assert _blocked_inside(caught.value)


@pytest.mark.anyio
async def test_async_httpx_to_jira_is_blocked() -> None:
    async with httpx.AsyncClient() as client:
        with pytest.raises(httpx.HTTPError) as caught:
            await client.get(JIRA_URL)
    assert _blocked_inside(caught.value)


def test_resolving_jira_is_blocked() -> None:
    with pytest.raises(NetworkBlocked):
        socket.getaddrinfo("jira.example.com", 443)
    with pytest.raises(NetworkBlocked):
        socket.getaddrinfo(b"jira.example.com", 443)


@pytest.mark.parametrize("address", ["192.0.2.1", "10.0.0.1"])
def test_create_connection_to_ipv4_is_blocked(address: str) -> None:
    with pytest.raises(NetworkBlocked):
        socket.create_connection((address, 443))


@pytest.mark.parametrize("address", ["192.0.2.1", "10.0.0.1"])
def test_raw_connect_to_ipv4_is_blocked(address: str) -> None:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        with pytest.raises(NetworkBlocked):
            sock.connect((address, 443))
        with pytest.raises(NetworkBlocked):
            sock.connect_ex((address, 443))


def test_ipv6_non_loopback_is_blocked() -> None:
    with pytest.raises(NetworkBlocked):
        socket.create_connection(("2001:db8::1", 443))
    with socket.socket(socket.AF_INET6, socket.SOCK_STREAM) as sock:
        with pytest.raises(NetworkBlocked):
            sock.connect(("2001:db8::1", 443, 0, 0))
        with pytest.raises(NetworkBlocked):
            sock.connect_ex(("2001:db8::1", 443, 0, 0))


def test_loopback_connection_works() -> None:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as listener:
        listener.bind(("127.0.0.1", 0))
        listener.listen(1)
        port = listener.getsockname()[1]
        with socket.create_connection(("127.0.0.1", port)) as client:
            peer, _ = listener.accept()
            with peer:
                client.sendall(b"ping")
                assert peer.recv(4) == b"ping"
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as other:
            assert other.connect_ex(("127.0.0.1", port)) == 0
            peer, _ = listener.accept()
            peer.close()


def test_localhost_resolves() -> None:
    infos = socket.getaddrinfo("localhost", 443, type=socket.SOCK_STREAM)
    assert infos
    assert all(info[4][0] in {"127.0.0.1", "::1"} for info in infos)


def test_socketpair_passes() -> None:
    left, right = socket.socketpair()
    with left, right:
        left.sendall(b"pong")
        assert right.recv(4) == b"pong"


def test_unix_socket_path_passes(tmp_path: Path) -> None:
    directory = tmp_path
    own: str | None = None
    if len(str(tmp_path / "s.sock")) > 100:
        own = tempfile.mkdtemp(prefix="jn", dir="/tmp")
        directory = Path(own)
    try:
        path = str(directory / "s.sock")
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as server:
            server.bind(path)
            server.listen(1)
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
                client.connect(path)
                peer, _ = server.accept()
                with peer:
                    client.sendall(b"unix")
                    assert peer.recv(4) == b"unix"
        Path(path).unlink()
    finally:
        if own is not None:
            shutil.rmtree(own, ignore_errors=True)


@pytest.mark.parametrize(
    "url", ["https://jira.example.com", "http://192.0.2.1:8080", "http://[2001:db8::1]:80"]
)
def test_make_config_refuses_non_loopback_jira_base_url(tmp_path: Path, url: str) -> None:
    with pytest.raises(ValueError, match="jira_base_url"):
        make_config(tmp_path, jira_base_url=url)


@pytest.mark.parametrize("url", ["https://chat.example.com", "http://10.0.0.1:9"])
def test_make_config_refuses_non_loopback_server_url(tmp_path: Path, url: str) -> None:
    with pytest.raises(ValueError, match="server_url"):
        make_config(tmp_path, jira_base_url="http://127.0.0.1:9", server_url=url)


@pytest.mark.parametrize(
    ("jira", "server"),
    [
        ("http://127.0.0.1:9", "http://127.0.0.1:9"),
        ("http://localhost:9", "http://[::1]:9"),
    ],
)
def test_make_config_accepts_loopback(tmp_path: Path, jira: str, server: str) -> None:
    config = make_config(tmp_path, jira_base_url=jira, server_url=server)
    assert config.jira_base_url == jira
    assert config.server_url == server
