# SPDX-License-Identifier: Apache-2.0
"""Private files of runner.state_dir and their retention (design notes 11 and 16)."""

from __future__ import annotations

import os
import time
from pathlib import Path

import pytest

from claude_code_runner.statedir import StateDirError, ensure_private_dir, sweep, write_private


def test_private_dir_and_files(tmp_path: Path) -> None:
    d = ensure_private_dir(tmp_path / "a" / "b")
    assert d.stat().st_mode & 0o777 == 0o700
    assert (tmp_path / "a").stat().st_mode & 0o777 == 0o700
    f = d / "x.json"
    write_private(f, "{}")
    assert f.stat().st_mode & 0o777 == 0o600
    write_private(f, b"[1]", atomic=True)
    assert f.read_text() == "[1]"
    assert f.stat().st_mode & 0o777 == 0o600
    assert sorted(p.name for p in d.iterdir()) == ["x.json"]


def test_open_dir_refused(tmp_path: Path) -> None:
    d = tmp_path / "open"
    d.mkdir()
    d.chmod(0o755)
    with pytest.raises(StateDirError):
        ensure_private_dir(d)


def test_sweep(tmp_path: Path) -> None:
    old = tmp_path / "sessions" / "s" / "old.stream.jsonl"
    new = tmp_path / "sessions" / "s" / "new.stream.jsonl"
    old_file = tmp_path / "files" / "t" / "f"
    keep = tmp_path / "turns" / "r.json"
    for p in (old, new, old_file, keep):
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text("x")
    past = time.time() - 3600
    for p in (old, old_file, keep):
        os.utime(p, (past, past))
    assert sweep(tmp_path, 60.0) == 2
    assert not old.exists()
    assert not old_file.exists()
    assert not old_file.parent.exists()
    assert new.exists()
    assert keep.exists()


def test_sweep_keeps_the_session_usage(tmp_path: Path) -> None:
    usage = tmp_path / "sessions" / "s" / "usage.json"
    usage.parent.mkdir(parents=True)
    usage.write_text("{}")
    past = time.time() - 3600
    os.utime(usage, (past, past))
    assert sweep(tmp_path, 60.0) == 0
    assert usage.exists()


def test_sweep_outgoing_and_run_temp(tmp_path: Path) -> None:
    # Staged answer attachments and run temporary directories go by run (M3-12 note 11).
    past = time.time() - 3600

    def make(rel: str, *, old: bool = True) -> Path:
        path = tmp_path / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("x")
        if old:
            os.utime(path, (past, past))
        return path

    make("outgoing/run-old/1/1")
    make("tmp/run-old/out/summary.txt")
    (tmp_path / "tmp" / "run-old" / "link").symlink_to(tmp_path / "elsewhere")
    os.utime(tmp_path / "tmp" / "run-old" / "link", (past, past), follow_symlinks=False)
    # An active run (own, or with a turn record) keeps even an old unpacked file.
    unpacked = make("tmp/run-active/src/main.c")
    recorded = make("tmp/run-recorded/old.txt")
    make("turns/run-recorded.json", old=False)
    # A run directory with one new entry stays as a whole.
    mixed = make("tmp/run-mixed/old.txt")
    make("tmp/run-mixed/new.txt", old=False)
    running = tmp_path / "tmp" / "run-now"
    running.mkdir()  # an empty directory of a run that works now stays
    for d in tmp_path.rglob("*"):
        if d.is_dir() and not d.is_symlink() and d.name != "run-now":
            os.utime(d, (past, past))
    assert sweep(tmp_path, 60.0, active={"run-active"}) == 3
    assert not (tmp_path / "outgoing" / "run-old").exists()
    assert not (tmp_path / "tmp" / "run-old").exists()
    assert (tmp_path / "elsewhere").exists() is False  # a link is removed, never followed
    assert unpacked.exists()
    assert recorded.exists()
    assert mixed.exists()
    assert running.is_dir()
