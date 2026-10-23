# SPDX-License-Identifier: Apache-2.0
"""Fake-Claude waits on gates the test opens, never on a fixed real time."""

from __future__ import annotations

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
