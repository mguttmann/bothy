# SPDX-License-Identifier: Apache-2.0
"""`make -C services/host release-build` (docs/spec/23-quality.md 3, docs/spec/18-release.md
"Components" row host, brief M7-23): usage and version errors, one unsigned binary per platform
with the right executable format and machine, and the .deb and .rpm packages of infra/host/linux
with the binary and the reference unit owned by root and writable by root only, nothing
below /etc. GNU make reports every
failed recipe with status 2, so the tests tell the usage error from the version mismatch by the
message. The installation of the packages runs in containers (`make release-trial COMPONENT=host`).
"""

from __future__ import annotations

import gzip
import io
import os
import struct
import subprocess
import tarfile
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
HOST = REPO / "services" / "host"
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "VERSION", "OUT"}


def _make(*args: str) -> subprocess.CompletedProcess[str]:
    env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
    return subprocess.run(
        ["make", "-s", "-C", str(HOST), *args],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=900,
    )


def test_release_build_without_arguments_is_usage_error() -> None:
    result = _make("release-build")
    assert result.returncode == 2
    assert "usage: make release-build VERSION=<x.y.z> OUT=<dir>" in result.stdout + result.stderr


def test_release_build_rejects_other_version(tmp_path: Path) -> None:
    result = _make("release-build", "VERSION=9.9.9", f"OUT={tmp_path}")
    recorded = (HOST / "VERSION").read_text(encoding="utf-8").strip()
    assert result.returncode == 2
    assert (
        f"release-build: VERSION file says {recorded}, not 9.9.9" in result.stdout + result.stderr
    )
    assert list(tmp_path.iterdir()) == []


def _format(path: Path) -> tuple[str, int]:
    """Return the executable format and machine field of a binary."""
    data = path.read_bytes()[:4096]
    if data[:4] == b"\x7fELF":
        return "elf", struct.unpack_from("<H", data, 18)[0]
    if data[:4] == b"\xcf\xfa\xed\xfe":
        return "macho", struct.unpack_from("<I", data, 4)[0]
    if data[:2] == b"MZ":
        pe = struct.unpack_from("<I", data, 0x3C)[0]
        assert data[pe : pe + 4] == b"PE\x00\x00"
        return "pe", struct.unpack_from("<H", data, pe + 4)[0]
    raise AssertionError(f"{path.name}: unknown executable format")


def test_release_build_writes_one_binary_per_platform(tmp_path: Path) -> None:
    version = (HOST / "VERSION").read_text(encoding="utf-8").strip()
    result = _make("release-build", f"VERSION={version}", f"OUT={tmp_path}")
    assert result.returncode == 0, result.stdout + result.stderr
    expected = {
        f"bothy-host_{version}_darwin_arm64": ("macho", 0x0100000C),
        f"bothy-host_{version}_darwin_amd64": ("macho", 0x01000007),
        f"bothy-host_{version}_linux_amd64": ("elf", 0x3E),
        f"bothy-host_{version}_linux_arm64": ("elf", 0xB7),
        f"bothy-host_{version}_windows_amd64.exe": ("pe", 0x8664),
    }
    packages = {
        f"bothy-host_{version}_linux_{arch}.{kind}"
        for kind in ("deb", "rpm")
        for arch in ("amd64", "arm64")
    }
    assert sorted(p.name for p in tmp_path.iterdir()) == sorted({*expected, *packages})
    for name, want in expected.items():
        assert _format(tmp_path / name) == want, name
    for name in packages:
        magic = (tmp_path / name).read_bytes()[:8]
        assert magic == (
            b"!<arch>\n" if name.endswith(".deb") else b"\xed\xab\xee\xdb" + magic[4:]
        ), name
    for arch, machine in (("amd64", 0x3E), ("arm64", 0xB7)):
        members = _deb_data(tmp_path / f"bothy-host_{version}_linux_{arch}.deb")
        assert members["./opt/bothy-host/bin/bothy-host"][:3] == (0o755, "root", "root")
        assert members["./opt/bothy-host/share/bothy-host.service"][:3] == (0o644, "root", "root")
        assert not [m for m in members if m.startswith("./etc/")]
        for folder in ("./opt/bothy-host/", "./opt/bothy-host/bin/", "./opt/bothy-host/share/"):
            assert members[folder][:3] == (0o755, "root", "root"), folder
        unit = (REPO / "infra" / "host" / "linux" / "bothy-host.service").read_bytes()
        assert members["./opt/bothy-host/share/bothy-host.service"][3] == unit
        binary = members["./opt/bothy-host/bin/bothy-host"][3]
        assert binary[:4] == b"\x7fELF"
        assert struct.unpack_from("<H", binary, 18)[0] == machine


def _deb_data(deb: Path) -> dict[str, tuple[int, str, str, bytes]]:
    """Return mode, owner, group and content of every member of the data archive of a .deb."""
    raw = deb.read_bytes()
    pos, data = 8, b""
    while pos < len(raw):
        name = raw[pos : pos + 16].decode().strip()
        size = int(raw[pos + 48 : pos + 58].decode().strip())
        body = raw[pos + 60 : pos + 60 + size]
        if name.startswith("data.tar"):
            data = gzip.decompress(body) if name.endswith(".gz") else body
        pos += 60 + size + (size % 2)
    assert data, f"{deb.name} has no data.tar"
    out = {}
    with tarfile.open(fileobj=io.BytesIO(data)) as archive:
        for m in archive.getmembers():
            name = m.name + ("/" if m.isdir() and not m.name.endswith("/") else "")
            content = archive.extractfile(m).read() if m.isfile() else b""  # type: ignore[union-attr]
            out[name] = (m.mode, m.uname, m.gname, content)
    return out
