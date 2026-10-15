# SPDX-License-Identifier: Apache-2.0
"""`make -C services/server release-build` (component contract of docs/spec/23-qualitaet.md 3,
docs/spec/18-release.md "Komponenten" row server): usage and version errors, and one static
binary `bothy` per image platform (linux/amd64, linux/arm64). GNU make reports every failed
recipe with status 2, so the tests tell the usage error from the version mismatch by the message.
"""

from __future__ import annotations

import os
import struct
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
SERVER = REPO / "services" / "server"
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "VERSION", "OUT"}


def _make(*args: str) -> subprocess.CompletedProcess[str]:
    env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
    return subprocess.run(
        ["make", "-s", "-C", str(SERVER), *args],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=1200,
    )


def test_release_build_without_arguments_is_usage_error() -> None:
    result = _make("release-build")
    assert result.returncode == 2
    assert "usage: make release-build VERSION=<x.y.z> OUT=<dir>" in result.stdout + result.stderr


def test_release_build_rejects_other_version(tmp_path: Path) -> None:
    result = _make("release-build", "VERSION=9.9.9", f"OUT={tmp_path}")
    recorded = (SERVER / "VERSION").read_text(encoding="utf-8").strip()
    assert result.returncode == 2
    assert (
        f"release-build: VERSION file says {recorded}, not 9.9.9" in result.stdout + result.stderr
    )
    assert list(tmp_path.iterdir()) == []


def test_release_build_writes_one_static_binary_per_image_platform(tmp_path: Path) -> None:
    version = (SERVER / "VERSION").read_text(encoding="utf-8").strip()
    result = _make("release-build", f"VERSION={version}", f"OUT={tmp_path}")
    assert result.returncode == 0, result.stdout + result.stderr
    expected = {f"bothy_{version}_linux_amd64": 0x3E, f"bothy_{version}_linux_arm64": 0xB7}
    assert sorted(p.name for p in tmp_path.iterdir()) == sorted(expected)
    for name, machine in expected.items():
        data = (tmp_path / name).read_bytes()
        assert data[:4] == b"\x7fELF", name
        assert struct.unpack_from("<H", data, 18)[0] == machine, name
        # Static (CGO_ENABLED=0): no program header of type PT_INTERP (3).
        phoff = struct.unpack_from("<Q", data, 32)[0]
        phentsize, phnum = struct.unpack_from("<HH", data, 54)
        kinds = {struct.unpack_from("<I", data, phoff + i * phentsize)[0] for i in range(phnum)}
        assert 3 not in kinds, f"{name} is dynamically linked"
