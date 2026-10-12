# SPDX-License-Identifier: Apache-2.0
"""The network sandbox of the Hermes process tree (macOS) and its Linux counterpart."""

from __future__ import annotations

import socket
import subprocess
import sys
from pathlib import Path

from hermes_harness.environment import ThrowawayHomes, build_environment
from hermes_harness.sandbox import sandbox_profile, wrap_command

_PROBE = """
import socket, sys
port = int(sys.argv[1])
s = socket.socket()
s.settimeout(5)
try:
    s.connect(("192.0.2.1", 443))
    print("outbound:connected")
except PermissionError:
    print("outbound:denied")
except OSError as exc:
    print("outbound:" + type(exc).__name__)
c = socket.create_connection(("127.0.0.1", port), timeout=5)
print("loopback:ok")
try:
    socket.create_connection(("127.0.0.1", int(sys.argv[2])), timeout=5)
    print("other loopback:connected")
except PermissionError:
    print("other loopback:denied")
except OSError as exc:
    print("other loopback:" + type(exc).__name__)
"""


# One case per platform, chosen by sys.platform; no skip marker (R-QA-02).
if sys.platform == "darwin":

    def test_sandbox_denies_outbound_except_loopback(tmp_path: Path) -> None:
        _sandbox_denies_outbound_except_loopback(ThrowawayHomes.create(tmp_path / "root"))

else:

    def test_linux_environment_reaches_no_credential_store(tmp_path: Path) -> None:
        _linux_environment_reaches_no_credential_store(
            ThrowawayHomes.create(tmp_path / "root"), tmp_path
        )


def _sandbox_denies_outbound_except_loopback(homes: ThrowawayHomes) -> None:
    listener = socket.socket()
    listener.bind(("127.0.0.1", 0))
    listener.listen()
    port = listener.getsockname()[1]
    # A loopback service the harness did not start (another local model or gateway).
    other = socket.socket()
    other.bind(("127.0.0.1", 0))
    other.listen()
    other_port = other.getsockname()[1]
    profile = homes.root / "sandbox.sb"
    profile.write_text(
        sandbox_profile(
            homes.root, readable=[Path(sys.base_prefix), Path(sys.prefix)], ports=[port]
        ),
        encoding="utf-8",
    )
    command = wrap_command([sys.executable, "-c", _PROBE, str(port), str(other_port)], profile)
    assert command[0] == "/usr/bin/sandbox-exec"
    result = subprocess.run(command, capture_output=True, text=True, timeout=30, check=False)
    listener.close()
    other.close()
    assert result.stdout.splitlines() == [
        "outbound:denied",
        "loopback:ok",
        "other loopback:denied",
    ], result.stderr


def _linux_environment_reaches_no_credential_store(homes: ThrowawayHomes, tmp_path: Path) -> None:
    env = build_environment(homes, "probe", "http://127.0.0.1:9", {}, venv_bin=tmp_path)
    assert "DBUS_SESSION_BUS_ADDRESS" not in env
    assert Path(env["HOME"]).is_relative_to(homes.root)
    assert env["PATH"].split(":")[0] == str(homes.shims)
    assert wrap_command(["true"], tmp_path / "unused.sb") == ["true"]


def test_profile_confines_the_process_tree(tmp_path: Path) -> None:
    root = (tmp_path / "root").resolve()
    text = sandbox_profile(
        root, readable=[tmp_path / "python"], ports=[4711, 4712], real_home=tmp_path
    )
    assert "(deny network-outbound)" in text
    assert "localhost:*" not in text
    assert '(allow network-outbound (remote ip "localhost:4711"))' in text
    assert '(allow network-outbound (remote ip "localhost:4712"))' in text
    assert f'(allow network-outbound (subpath "{root}"))' in text
    assert f'(deny file-read-data (subpath "{tmp_path.resolve()}"))' in text
    assert f'(allow file-write* (subpath "{root}"))' in text
    assert f'(allow file-read-data (subpath "{(tmp_path / "python").resolve()}"))' in text
    assert "(allow signal (target same-sandbox))" in text
    assert "(allow process-info* (target same-sandbox))" in text
    assert '(deny process-exec (literal "/bin/launchctl"))' in text


def test_profile_refuses_quotes_in_paths(tmp_path: Path) -> None:
    import pytest

    with pytest.raises(ValueError, match="path"):
        sandbox_profile(tmp_path / 'ro"ot', readable=[], ports=[])


def test_sandbox_denies_reading_the_real_home_and_signalling_others(tmp_path: Path) -> None:
    if sys.platform != "darwin":
        assert wrap_command(["true"], tmp_path / "unused.sb") == ["true"]
        return
    home = tmp_path / "home"
    (home / "secret").mkdir(parents=True)
    (home / "secret" / "file").write_text("x", encoding="utf-8")
    root = home / "root"
    root.mkdir()
    profile = root / "p.sb"
    profile.write_text(
        sandbox_profile(
            root, readable=[Path(sys.base_prefix), Path(sys.prefix)], ports=[], real_home=home
        ),
        encoding="utf-8",
    )
    outsider = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"])
    code = (
        "import os, sys\n"
        "for label, call in (('read', lambda: open(sys.argv[1]).read()),"
        " ('signal', lambda: os.kill(int(sys.argv[2]), 0)),"
        " ('write', lambda: open(sys.argv[3], 'w').write('x'))):\n"
        "    try:\n        call(); print(label, 'allowed')\n"
        "    except PermissionError:\n        print(label, 'denied')\n"
    )
    try:
        result = subprocess.run(
            wrap_command(
                [
                    sys.executable,
                    "-c",
                    code,
                    str(home / "secret" / "file"),
                    str(outsider.pid),
                    str(home / "written"),
                ],
                profile,
            ),
            capture_output=True,
            text=True,
            timeout=30,
            check=False,
        )
    finally:
        outsider.kill()
        outsider.wait()
    assert result.stdout.splitlines() == ["read denied", "signal denied", "write denied"], (
        result.stderr
    )
