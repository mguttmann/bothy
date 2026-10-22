# SPDX-License-Identifier: Apache-2.0
"""Static checks of the clamd wiring of the Compose stack (M3-04, docs/spec/12-files.md 5.4,
docs/spec/19-security.md SEC-TLS-5): clamd is reached only over its Unix socket in the volume
clamav-socket, and infra/compose/clamav/clamd.conf sets the mandatory options. No network."""

from __future__ import annotations

import re
from pathlib import Path, PurePosixPath

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
COMPOSE = REPO / "infra" / "compose" / "compose.yaml"
ENV_EXAMPLE = REPO / "infra" / "compose" / ".env.example"
CLAMD_CONF = REPO / "infra" / "compose" / "clamav" / "clamd.conf"
VOLUME = "clamav-socket"

_DEFAULT = re.compile(r"^\$\{BOTHY_CLAMD_ADDRESS:-(?P<value>[^}]+)\}$")
_SIZE = re.compile(r"^(?P<n>\d+)(?P<unit>[kKmMgG]?)$")
_UNITS = {"": 1, "k": 1 << 10, "m": 1 << 20, "g": 1 << 30}


def compose() -> dict:
    return yaml.safe_load(COMPOSE.read_text(encoding="utf-8"))


def chapter_default(chapter: str, key: str) -> str:
    text = (REPO / "docs" / "spec" / chapter).read_text(encoding="utf-8")
    m = re.search(rf"^\| `{re.escape(key)}` \| `(?P<value>[^`]+)` \|", text, re.MULTILINE)
    assert m, f"{key} has no row with a default in {chapter}"
    return m.group("value")


def clamd_options() -> dict[str, list[str]]:
    out: dict[str, list[str]] = {}
    for raw in CLAMD_CONF.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        name, _, value = line.partition(" ")
        out.setdefault(name, []).append(value.strip())
    return out


def clamd_size(value: str) -> int:
    """A size of clamd.conf: a number with an optional suffix k/K, m/M or g/G (1M = 1048576)."""
    m = _SIZE.match(value)
    assert m, f"size {value!r} is not a clamd size"
    return int(m.group("n")) * _UNITS[m.group("unit").lower()]


def volume_mounts(service: dict) -> list[tuple[str, str, bool]]:
    """The (source, target, read_only) of the volume mounts of a service."""
    out = []
    for v in service.get("volumes", []):
        if isinstance(v, str):
            parts = v.split(":")
            out.append((parts[0], parts[1], len(parts) > 2 and "ro" in parts[2].split(",")))
        else:
            out.append((v.get("source", ""), v.get("target", ""), bool(v.get("read_only", False))))
    return out


def socket_dir() -> PurePosixPath:
    return PurePosixPath(clamd_options()["LocalSocket"][0]).parent


def test_one_unix_address_everywhere() -> None:
    env = compose()["x-server-env"]["BOTHY_FILE__SCAN__CLAMD_ADDRESS"]
    m = _DEFAULT.match(env)
    assert m, f"BOTHY_FILE__SCAN__CLAMD_ADDRESS {env!r} has no default"
    env_text = ENV_EXAMPLE.read_text(encoding="utf-8")
    example = re.search(r"^BOTHY_CLAMD_ADDRESS=(\S+)$", env_text, re.MULTILINE)
    assert example, "BOTHY_CLAMD_ADDRESS missing in .env.example"
    chapter = chapter_default("12-files.md", "file.scan.clamd_address")
    assert m.group("value") == example.group(1) == chapter
    assert chapter.startswith("unix:///")
    socket = PurePosixPath(clamd_options()["LocalSocket"][0])
    assert PurePosixPath(chapter.removeprefix("unix://")) == socket


@pytest.mark.parametrize("path", [COMPOSE, ENV_EXAMPLE])
def test_no_tcp_clamd_address(path: Path) -> None:
    assert not re.search(r"tcp://clamav|clamav:3310", path.read_text(encoding="utf-8"))


def test_clamav_publishes_nothing() -> None:
    clamav = compose()["services"]["clamav"]
    assert "ports" not in clamav
    assert "expose" not in clamav


def test_socket_volume_only_in_clamav_and_worker() -> None:
    services = compose()["services"]
    target = str(socket_dir())
    seen = {}
    for name, svc in services.items():
        for source, mount, ro in volume_mounts(svc):
            if source == VOLUME:
                seen[name] = (mount, ro)
    assert seen == {"clamav": (target, False), "worker": (target, True)}
    assert VOLUME in compose()["volumes"]


def test_clamd_conf_mounted_read_only() -> None:
    mounts = volume_mounts(compose()["services"]["clamav"])
    assert ("./clamav/clamd.conf", "/etc/clamav/clamd.conf", True) in mounts


def test_clamd_conf_mandatory_options() -> None:
    opts = clamd_options()
    assert opts.get("AlertExceedsMax") == ["yes"]
    assert len(opts.get("LocalSocket", [])) == 1
    assert "TCPSocket" not in opts, "clamd of the Compose stack listens on its Unix socket only"
    stream = clamd_size(opts["StreamMaxLength"][0])
    assert stream == int(chapter_default("12-files.md", "file.scan.clamd_stream_max_bytes"))
    max_upload = int(chapter_default("03-data-model.md", "file.max_upload_bytes"))
    assert clamd_size(opts["MaxFileSize"][0]) >= max_upload
    for default in ("MaxScanSize", "MaxRecursion", "MaxFiles", "AlertEncrypted"):
        assert default not in opts, f"{default} must stay at the clamd default (12 5.4)"


def test_clamd_size_suffixes() -> None:
    assert clamd_size("100M") == 104857600
    assert clamd_size("25k") == 25600
    assert clamd_size("7") == 7
