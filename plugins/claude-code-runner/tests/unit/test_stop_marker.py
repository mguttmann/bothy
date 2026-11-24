# SPDX-License-Identifier: Apache-2.0
"""The marker of ``stop --turns`` counts only while the command that wrote it lives (09 18.19.3)."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

from claude_code_runner.handover import stop_turns_requested


def _dead_pid() -> int:
    proc = subprocess.Popen([sys.executable, "-c", "pass"])
    proc.wait()
    return proc.pid


def test_no_marker_asks_for_nothing(tmp_path: Path) -> None:
    assert not stop_turns_requested(tmp_path)


def test_a_marker_of_a_living_command_asks_for_the_stop(tmp_path: Path) -> None:
    (tmp_path / "stop-turns").write_text(json.dumps({"pid": os.getpid()}))
    assert stop_turns_requested(tmp_path)


def test_a_marker_left_by_a_killed_command_is_ignored(tmp_path: Path) -> None:
    # A command ended by SIGKILL leaves the marker; the next SIGTERM must hand over, not stop.
    (tmp_path / "stop-turns").write_text(json.dumps({"pid": _dead_pid()}))
    assert not stop_turns_requested(tmp_path)


def test_an_empty_or_broken_marker_is_ignored(tmp_path: Path) -> None:
    marker = tmp_path / "stop-turns"
    marker.touch()
    assert not stop_turns_requested(tmp_path)
    marker.write_text("{not json")
    assert not stop_turns_requested(tmp_path)
