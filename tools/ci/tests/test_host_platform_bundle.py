# SPDX-License-Identifier: Apache-2.0
"""`make -C services/host platform-bundle` (brief M2-48 design note 11): the usage error without
``OUT``, and a Linux bundle built on any machine whose ``bundle.json`` lists every file with its
SHA-256, whose test binary is an ELF executable for the asked machine, and whose fake programs
log where the platform suite looks on the test machine. Nothing of the bundle runs here: the
suite creates users and services and runs only on disposable machines.
"""

from __future__ import annotations

import hashlib
import json
import os
import struct
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
HOST = REPO / "services" / "host"
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "OUT", "GOOS", "GOARCH", "PLATFORM_LOGDIR"}


def _make(*args: str) -> subprocess.CompletedProcess[str]:
    env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
    env["GOTOOLCHAIN"] = "local"
    return subprocess.run(
        ["make", "-s", "-C", str(HOST), *args],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=1800,
    )


def _sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def test_platform_bundle_without_out_is_usage_error() -> None:
    result = _make("platform-bundle")
    assert result.returncode == 2
    assert (
        "usage: make platform-bundle OUT=<dir> [GOOS=<os> GOARCH=<arch>]"
        in result.stdout + result.stderr
    )


def test_platform_bundle_writes_bundle_for_linux(tmp_path: Path) -> None:
    out = tmp_path / "bundle"
    result = _make("platform-bundle", f"OUT={out}", "GOOS=linux", "GOARCH=amd64")
    assert result.returncode == 0, result.stdout + result.stderr

    manifest = json.loads((out / "bundle.json").read_text(encoding="utf-8"))
    head = subprocess.run(
        ["git", "rev-parse", "HEAD"], cwd=REPO, capture_output=True, text=True, check=True
    ).stdout.strip()
    assert manifest["commit"] == head
    assert manifest["goos"] == "linux"
    assert manifest["goarch"] == "amd64"
    assert manifest["go_version"].startswith("go")

    on_disk = sorted(
        p.relative_to(out).as_posix()
        for p in out.rglob("*")
        if p.is_file() and p.name != "bundle.json"
    )
    listed = {f["path"]: f["sha256"] for f in manifest["files"]}
    assert sorted(listed) == on_disk
    for rel, digest in listed.items():
        assert _sha256(out / rel) == digest, rel

    for name in (
        "bin/bothy-host",
        "bin/claude",
        "bin/node",
        "bin/suite-claude",
        "bin/bighome",
        "bin/platform.test",
        "bin/pty.test",
        "bin/session.test",
        "bin/profiles.test",
        "bin/confirm.test",
        "run.sh",
    ):
        assert name in listed, name
    assert os.access(out / "run.sh", os.X_OK)

    head_bytes = (out / "bin" / "platform.test").read_bytes()[:20]
    assert head_bytes[:4] == b"\x7fELF"
    assert struct.unpack_from("<H", head_bytes, 18)[0] == 0x3E  # x86-64

    # The fake programs of a copied bundle log in the fixed folders of the suite (programDir),
    # never in the folder of the build machine.
    suite_fake = (out / "bin" / "suite-claude").read_bytes()
    assert b"/opt/bothy-platform/fake/claude-starts.log" in suite_fake
    assert b"/opt/bothy-platform/claude-starts.log" in (out / "bin" / "claude").read_bytes()
    assert str(tmp_path).encode() not in suite_fake
