# SPDX-License-Identifier: Apache-2.0
"""The parts of the sdkserver driver that run without Go and Postgres (HERMES-E2E-REAL-SERVER).

The driver itself is exercised by the real-server suites (pytest marker ``e2e``); these tests hold
its pure helpers and its refusals.
"""

from __future__ import annotations

import struct
from pathlib import Path

import pytest

from bothy_sdk.testing import sdkserver
from bothy_sdk.testing.sdkserver import ApiAnswer, build_sdkserver, m4a, repository_root


def _boxes(data: bytes) -> list[bytes]:
    kinds, at = [], 0
    while at < len(data):
        (size,) = struct.unpack(">I", data[at : at + 4])
        kinds.append(data[at + 4 : at + 8])
        at += size
    return kinds


def test_m4a_is_an_mp4_audio_file_with_its_duration() -> None:
    data = m4a(3)
    assert _boxes(data) == [b"ftyp", b"moov", b"mdat"]
    assert data[8:12] == b"M4A "
    mvhd = data.index(b"mvhd")
    # version and flags, created, modified, time scale 1000, duration in units of the scale
    assert struct.unpack(">IIII", data[mvhd + 8 : mvhd + 24]) == (0, 0, 1000, 3000)
    assert b"soun" in data


def test_member_reads_nested_members_and_none_for_a_missing_one() -> None:
    answer = ApiAnswer(200, {"run": {"id": "r1", "state": "running"}, "items": [1]})
    assert answer.member("run", "id") == "r1"
    assert answer.member("run", "nope") is None
    assert answer.member("items", "0") is None
    assert answer.member() == answer.body


def test_expect_names_status_and_body() -> None:
    sdkserver._expect(ApiAnswer(201, {}), 201)
    with pytest.raises(RuntimeError, match=r"status 409, want 201: \{'code': 'x'\}"):
        sdkserver._expect(ApiAnswer(409, {"code": "x"}), 201)


def test_repository_root_holds_the_server() -> None:
    root = repository_root()
    assert (root / "services" / "server").is_dir()
    assert Path(sdkserver.__file__).resolve().is_relative_to(root)


def test_build_needs_go(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.setattr(sdkserver.shutil, "which", lambda _name: None)
    with pytest.raises(RuntimeError, match="Go tool chain"):
        build_sdkserver(tmp_path)
