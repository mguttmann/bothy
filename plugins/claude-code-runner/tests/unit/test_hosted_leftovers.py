# SPDX-License-Identifier: Apache-2.0
"""Hosted turns an earlier runner left behind (docs/spec/26-builtin-agents.md 3.2 item 5).

Hosted turns are not adopted. At its start the runner stops a process that still matches its
turn record under ``hosted/<agent_id>/turns/`` and removes the record; a process that does not
match is never signalled, and its record goes only when its locks are free. The stray process of
these tests is no child of the test, like a turn whose runner died.
"""

from __future__ import annotations

import contextlib
import json
import os
import signal
import subprocess
import sys
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.testing._patience import PATIENCE, eventually
from scenarios import conftest as scenarios
from scenarios.conftest import LogLines

from claude_code_runner.adoption import read_identity
from claude_code_runner.hosting import stop_leftover_turns
from claude_code_runner.locks import acquire, release

pytestmark = pytest.mark.anyio

logs = scenarios.logs

AGENT = "0192f4a0-7b1c-7d07-84b0-358556a08a99"
RUN = "0192f4a0-7b1c-7d07-84b0-000000000001"
THREAD = "0192f4a0-7b1c-7d07-84b0-000000000002"
REF = "0192d3a0-0000-4000-8000-0000000000dd"
_GRACE: dict[str, float] = {"stop_grace": 1.0, "kill_grace": 1.0, "poll": 0.01, "timeout": 30.0}


def _events(logs: LogLines, event: str) -> list[dict[str, str]]:
    out = []
    for record in logs.records:
        try:
            line = json.loads(record.getMessage())
        except ValueError:
            continue
        if isinstance(line, dict) and line.get("event") == event:
            out.append(line)
    return out


def _alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


@pytest.fixture
async def stray(tmp_path: Path) -> AsyncIterator[int]:
    """A process like a turn that outlived its runner: a group of its own, no child of this
    test (so it is reaped once it ends), with the session reference in its command line."""
    ready = tmp_path / "ready"
    turn = (
        "import pathlib, signal, sys, time\n"
        "signal.signal(signal.SIGINT, signal.SIG_DFL)\n"
        "pathlib.Path(sys.argv[2]).touch()\n"
        "time.sleep(3600)\n"  # only a bound for an orphan: the fixture ends the process
    )
    starter = (
        "import subprocess, sys\n"
        "gone = subprocess.DEVNULL\n"
        "turn = subprocess.Popen([sys.executable, '-c', *sys.argv[1:]], start_new_session=True,\n"
        "                        stdin=gone, stdout=gone, stderr=gone)\n"
        "print(turn.pid)\n"
    )
    started = subprocess.run(
        [sys.executable, "-c", starter, turn, REF, str(ready)],
        capture_output=True,
        text=True,
        check=True,
    )
    pid = int(started.stdout)
    try:
        await eventually(ready.exists, "the stray process to run")
        yield pid
    finally:
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(pid, signal.SIGKILL)


def _leftover(state: Path, pid: int, **changes: Any) -> Path:
    """The record a crashed runner left of a hosted run whose turn is the process ``pid``."""
    identity = read_identity(pid, 30.0)
    assert identity is not None
    record = {
        "run_id": RUN,
        "thread_id": THREAD,
        "session_ref": REF,
        "pid": pid,
        "pgid": pid,
        "identity": {"lstart": identity.lstart, "command": identity.command},
        **changes,
    }
    turns = state / "hosted" / AGENT / "turns"
    turns.mkdir(parents=True)
    path = turns / f"{RUN}.json"
    path.write_text(json.dumps(record))
    return path


async def test_a_leftover_hosted_turn_that_runs_is_stopped_and_its_record_removed(
    tmp_path: Path, stray: int, logs: LogLines
) -> None:
    path = _leftover(tmp_path, stray)
    # SIGINT ends it: an escalation would come only after the whole patience.
    await stop_leftover_turns(tmp_path, **{**_GRACE, "stop_grace": PATIENCE})
    assert not _alive(stray)  # the stop returns only once no process of the group is left
    assert not path.exists()
    assert _events(logs, "turn_adopted") == [
        {
            "event": "turn_adopted",
            "action": "stop",
            "agent_id": AGENT,
            "run_id": RUN,
            "thread_id": THREAD,
        }
    ]
    assert _events(logs, "stop_escalated") == []


async def test_the_record_of_a_gone_hosted_turn_is_removed_and_an_unreadable_one_kept(
    tmp_path: Path, stray: int, logs: LogLines
) -> None:
    path = _leftover(tmp_path, stray)
    unreadable = path.with_name("0192f4a0-7b1c-7d07-84b0-000000000003.json")
    unreadable.write_text("{")
    os.killpg(stray, signal.SIGKILL)
    await eventually(lambda: not _alive(stray), "the end of the stray process")
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert not path.exists()
    assert [e["action"] for e in _events(logs, "turn_adopted")] == ["leave"]
    assert unreadable.exists()
    assert [e["record"] for e in _events(logs, "turn_record_unreadable")] == [unreadable.stem]


async def test_a_process_that_does_not_match_its_hosted_record_is_never_signalled(
    tmp_path: Path, stray: int, logs: LogLines
) -> None:
    # Any doubt about the identity counts as gone (09 18.9): no signal. A turn that lives on
    # holds the thread's lock through its inherited descriptor, so its record stays.
    identity = {"lstart": "Thu Jan  1 00:00:00 1970", "command": "claude"}
    path = _leftover(tmp_path, stray, identity=identity)
    lock = tmp_path / "hosted" / AGENT / "locks" / f"thread-{THREAD}.lock"
    held = await acquire(lock, 0, 0.01)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert path.exists()
    assert len(_events(logs, "adoption_lock_busy")) == 1
    assert _events(logs, "turn_adopted") == []
    release(held)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert not path.exists()
    assert [e["action"] for e in _events(logs, "turn_adopted")] == ["leave"]
    assert _alive(stray)


async def test_no_hosted_folder_is_no_leftover(tmp_path: Path) -> None:
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert list(tmp_path.iterdir()) == []
