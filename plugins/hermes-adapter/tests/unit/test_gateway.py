# SPDX-License-Identifier: Apache-2.0
"""Start, readiness, listener check and stop of the gateway process group (stand-in command)."""

from __future__ import annotations

import os
import subprocess
import sys
import time
from pathlib import Path

import pytest
from hermes_harness.control import ControlServer
from hermes_harness.gateway import GatewayError, HermesGateway, unexpected_listeners

PARENT = """
import subprocess, sys, time
child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(600)"])
print(child.pid, flush=True)
time.sleep(600)
"""
LISTENER = """
import socket, time
s = socket.socket()
s.bind(("127.0.0.1", 0))
s.listen()
print(s.getsockname()[1], flush=True)
time.sleep(600)
"""
STUBBORN = """
import signal, time
signal.signal(signal.SIGTERM, signal.SIG_IGN)
print("ready", flush=True)
time.sleep(600)
"""


def _gateway(tmp_path: Path, code: str) -> HermesGateway:
    env = {"PATH": "/usr/bin:/bin", "HOME": str(tmp_path)}
    return HermesGateway(
        [sys.executable, "-c", code], env=env, cwd=tmp_path, log=tmp_path / "gw.log"
    )


def _first_line(log: Path) -> str:
    for _ in range(200):
        if log.is_file() and log.read_text().strip():
            return log.read_text().split()[0]
        time.sleep(0.05)
    raise AssertionError("stand-in printed nothing")


def _alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    # A zombie still answers kill(0); it no longer runs.
    state = subprocess.run(
        ["ps", "-o", "stat=", "-p", str(pid)], capture_output=True, text=True, check=False
    )
    return bool(state.stdout.strip()) and not state.stdout.strip().startswith("Z")


def test_stop_ends_the_whole_process_group(tmp_path: Path) -> None:
    outsider = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(600)"])
    gateway = _gateway(tmp_path, PARENT)
    try:
        gateway.start()
        child = int(_first_line(tmp_path / "gw.log"))
        members = gateway.members()
        assert gateway.pid in members
        assert child in members
        assert outsider.pid not in members
        gateway.stop()
        assert not _alive(gateway.pid)
        assert not _alive(child)
        assert gateway.members() == []
        assert outsider.poll() is None
    finally:
        outsider.kill()
        outsider.wait()


def test_stop_kills_after_the_grace_and_is_idempotent(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    import hermes_harness.gateway as module

    monkeypatch.setattr(module, "STOP_GRACE_S", 0.5)
    gateway = _gateway(tmp_path, STUBBORN)
    gateway.start()
    _first_line(tmp_path / "gw.log")
    gateway.stop()
    assert gateway.members() == []
    gateway.stop()
    assert gateway.returncode is not None


def test_listening_sockets_reports_a_listener(tmp_path: Path) -> None:
    (tmp_path / "a").mkdir()
    (tmp_path / "b").mkdir()
    listener = _gateway(tmp_path / "a", LISTENER)
    quiet = _gateway(tmp_path / "b", PARENT)
    try:
        listener.start()
        port = _first_line(tmp_path / "a" / "gw.log")
        quiet.start()
        _first_line(tmp_path / "b" / "gw.log")
        found = listener.listening_sockets()
        assert any(f"127.0.0.1:{port}" in entry and entry.startswith("tcp") for entry in found), (
            found
        )
        assert quiet.listening_sockets() == []
    finally:
        listener.stop()
        quiet.stop()


def test_start_fails_after_the_named_deadline_without_connect(tmp_path: Path) -> None:
    gateway = _gateway(tmp_path, PARENT)
    with ControlServer() as control:
        gateway.start()
        try:
            with pytest.raises(GatewayError, match="connect"):
                gateway.wait_connected(control, deadline_s=0.5)
        finally:
            gateway.stop()


def test_start_reports_an_early_exit(tmp_path: Path) -> None:
    gateway = _gateway(tmp_path, "raise SystemExit(3)")
    with ControlServer() as control:
        gateway.start()
        with pytest.raises(GatewayError, match="exited with 3"):
            gateway.wait_connected(control, deadline_s=30)
        gateway.stop()


def test_connect_is_seen(tmp_path: Path) -> None:
    with ControlServer() as control:
        code = (
            "import json, urllib.request, time\n"
            "body = json.dumps({'method': 'connect', 'args': {}}).encode()\n"
            "req = urllib.request.Request('URL/call', body, {'Content-Type': 'application/json'})\n"
            "urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req)\n"
            "time.sleep(600)\n"
        ).replace("URL", control.url)
        gateway = _gateway(tmp_path, code)
        gateway.start()
        try:
            assert gateway.wait_connected(control, deadline_s=30)["method"] == "connect"
        finally:
            gateway.stop()


def test_gateway_must_be_started_before_use(tmp_path: Path) -> None:
    gateway = _gateway(tmp_path, PARENT)
    with pytest.raises(GatewayError, match="not started"):
        gateway.members()
    gateway.stop()


def test_only_the_runtime_api_of_hermes_may_listen() -> None:
    # Hermes v0.22 always serves its local runtime API on one loopback port (no setting turns
    # it off); every other listener, and a second loopback one, still fails a run.
    assert unexpected_listeners([]) == []
    assert unexpected_listeners(["tcp 127.0.0.1:52307"]) == []
    assert unexpected_listeners(["tcp 0.0.0.0:8080"]) == ["tcp 0.0.0.0:8080"]
    assert unexpected_listeners(["tcp [::1]:52307"]) == ["tcp [::1]:52307"]
    assert unexpected_listeners(["udp 127.0.0.1:5353"]) == ["udp 127.0.0.1:5353"]
    both = ["tcp 127.0.0.1:52307", "tcp 127.0.0.1:52308"]
    assert unexpected_listeners(both) == both
    mixed = ["tcp 127.0.0.1:52307", "tcp *:9000"]
    assert unexpected_listeners(mixed) == ["tcp *:9000"]
