# SPDX-License-Identifier: Apache-2.0
"""Adoption decisions and the identity check before any signal (design note 12)."""

from __future__ import annotations

import os
import subprocess
import sys
from collections.abc import Iterator
from typing import Any

import pytest

from claude_code_runner.adoption import (
    AdoptionAction,
    adoption_decision,
    process_matches,
    read_identity,
)

REF = "0192d3a0-0000-4000-8000-0000000000dd"


def test_adoption_decision() -> None:
    assert adoption_decision(alive=True, run_state="running", result_in_log=False) is (
        AdoptionAction.ADOPT
    )
    for ended in ("failed", "cancelled", "completed"):
        assert adoption_decision(alive=True, run_state=ended, result_in_log=True) is (
            AdoptionAction.STOP
        )
    assert adoption_decision(alive=False, run_state="running", result_in_log=True) is (
        AdoptionAction.FINISH_FROM_LOG
    )
    assert adoption_decision(alive=False, run_state="running", result_in_log=False) is (
        AdoptionAction.LEAVE
    )


@pytest.fixture
def child() -> Iterator[subprocess.Popen[bytes]]:
    # A process of this test in its own group, with the session reference in its command line.
    process = subprocess.Popen(
        [sys.executable, "-c", "import sys; sys.stdin.read()", REF],
        stdin=subprocess.PIPE,
        start_new_session=True,
    )
    try:
        yield process
    finally:
        process.kill()
        process.wait()
        if process.stdin is not None:
            process.stdin.close()


def record_of(process: subprocess.Popen[bytes]) -> dict[str, Any]:
    identity = read_identity(process.pid, 30.0)
    assert identity is not None
    return {
        "pid": process.pid,
        "pgid": os.getpgid(process.pid),
        "session_ref": REF,
        "identity": {"lstart": identity.lstart, "command": identity.command},
    }


def test_identity_matches_the_started_process(child: subprocess.Popen[bytes]) -> None:
    assert process_matches(record_of(child), 30.0) is True


def test_identity_mismatch_counts_as_gone(child: subprocess.Popen[bytes]) -> None:
    record = record_of(child)
    assert process_matches({**record, "pgid": record["pgid"] + 1}, 30.0) is False
    other_start = {**record["identity"], "lstart": "Thu Jan  1 00:00:00 1970"}
    assert process_matches({**record, "identity": other_start}, 30.0) is False
    assert process_matches({**record, "session_ref": "another-reference"}, 30.0) is False
    assert process_matches({**record, "pid": 1}, 30.0) is False
    child.kill()
    child.wait()
    assert process_matches(record, 30.0) is False
