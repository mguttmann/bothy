# SPDX-License-Identifier: Apache-2.0
"""Fake-Claude waits on gates the test opens, never on a fixed real time."""

from __future__ import annotations

import signal
import subprocess
import sys
import time
from pathlib import Path

import pytest
from bothy_sdk.testing._patience import PATIENCE

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import helper


@pytest.mark.parametrize("key", sorted(helper.WALL_CLOCK_KEYS))
def test_wall_clock_keys_are_refused(tmp_path: Path, key: str) -> None:
    with pytest.raises(ValueError, match=key):
        helper.install(tmp_path / "bin", {key: 1}, tmp_path / "capture.jsonl")
    wrapper = helper.install(tmp_path / "bin", {}, tmp_path / "capture.jsonl")
    with pytest.raises(ValueError, match=key):
        helper.update(wrapper, {key: 1})


def test_auth_gate_holds_the_answer_until_released(tmp_path: Path) -> None:
    gate = tmp_path / "gate"
    capture = tmp_path / "capture.jsonl"
    wrapper = helper.install(tmp_path / "bin", {"auth_gate": str(gate)}, capture)
    proc = subprocess.Popen([str(wrapper), "auth", "status"], stdout=subprocess.PIPE, text=True)
    try:
        while not any(c["kind"] == "gate_wait" for c in helper.read_capture(capture)):
            assert proc.poll() is None, "answered before the gate opened"
            time.sleep(0.01)
        assert proc.poll() is None
        gate.touch()
        out, _ = proc.communicate(timeout=PATIENCE)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()
    assert '"loggedIn"' in out


def test_sigint_during_a_blocked_write_ends_the_turn_cleanly(tmp_path: Path) -> None:
    # The echo of a large first line fills the stdout pipe, which the test reads only after the
    # SIGINT, so the signal arrives while Fake-Claude is (or will be) inside its flush. The
    # handler writes the error result itself; it must run after the flush, not inside it.
    capture = tmp_path / "capture.jsonl"
    wrapper = helper.install(tmp_path / "bin", {"fixture": "plain", "hold": True}, capture)
    proc = subprocess.Popen(
        [str(wrapper), "-p", "--output-format", "stream-json"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    assert proc.stdin is not None
    try:
        proc.stdin.write(b'{"type":"user","pad":"' + b"x" * 400_000 + b'"}\n')
        proc.stdin.flush()
        while not any(c["kind"] == "stdin" for c in helper.read_capture(capture)):
            assert proc.poll() is None
            time.sleep(0.01)
        proc.send_signal(signal.SIGINT)
        out, err = proc.communicate(timeout=PATIENCE)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()
    assert proc.returncode == 130, err.decode()
    ends = [c for c in helper.read_capture(capture) if c["kind"] == "turn_end"]
    assert [e.get("signal") for e in ends] == ["SIGINT"]
    assert b'"error_during_execution"' in out.splitlines()[-1]
