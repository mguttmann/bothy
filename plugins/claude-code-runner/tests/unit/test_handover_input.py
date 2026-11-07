# SPDX-License-Identifier: Apache-2.0
"""An input line in flight at a handover (docs/spec/09-agents.md 18.19.3).

The line is finished within ``runner.kill_grace`` and then no longer recorded as started, so the
adopter keeps full rights; or it is cut and recorded with the count of its bytes written
(``partial_input``), so the adopter writes the rest first. A line recorded as started without a
byte count would make the adopter close stdin.
"""

from __future__ import annotations

import asyncio
import os
import types
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner.launch import KeeperGroup, LaunchFiles
from claude_code_runner.ledger import TurnLedger
from claude_code_runner.processstate import ProcessState
from claude_code_runner.runturn import RunTurn

pytestmark = pytest.mark.anyio

LINE_CHARS = 400_000  # larger than a pipe buffer: the write stays in flight until drained


def _turn(group: KeeperGroup, kill_grace: float, snapshots: list[Any]) -> RunTurn:
    turn = RunTurn.__new__(RunTurn)
    turn.write_lock = asyncio.Lock()
    turn.process = group
    turn.phase = "open"
    turn.stop_reason = None
    turn.lines = {}
    turn.ledger = TurnLedger.begin("turn", 0)
    turn.autonomous = set()
    turn.ps = ProcessState(has_stdin=True)
    turn.handover = True
    turn.handed_over = False
    turn.input_write = None
    turn.keeper_start = None
    turn.launch = None
    turn.record_phase = "process"
    turn.cfg = types.SimpleNamespace(kill_grace=kill_grace, poll_interval=0.01)
    turn.ids = {}
    turn._store_lines = lambda inputs: None  # type: ignore[method-assign]
    turn._persist = lambda: snapshots.append(  # type: ignore[method-assign]
        None if turn.input_write is None else dict(turn.input_write)
    )
    return turn


async def _drain(fd: int, writer: asyncio.Future[Any], delay: float) -> int:
    await asyncio.sleep(delay)
    total = 0
    while True:
        try:
            chunk = os.read(fd, 65536)
        except BlockingIOError:
            if writer.done():
                return total
            await asyncio.sleep(0.005)
            continue
        if not chunk:
            return total
        total += len(chunk)


async def _setup(tmp_path: Path) -> tuple[KeeperGroup, int]:
    files = LaunchFiles(tmp_path / "r.1.1")
    os.mkfifo(files.stdin, 0o600)
    reader = os.open(files.stdin, os.O_RDONLY | os.O_NONBLOCK)
    group = KeeperGroup(files, keeper={}, cli={}, session_ref="s", timeout=1.0)
    return group, reader


async def test_a_line_finished_within_kill_grace_is_not_left_as_started(tmp_path: Path) -> None:
    group, reader = await _setup(tmp_path)
    snapshots: list[Any] = []
    turn = _turn(group, kill_grace=5.0, snapshots=snapshots)
    try:
        writer = asyncio.ensure_future(turn.offer_input("x" * LINE_CHARS))
        while not group.writer.in_flight:
            await asyncio.sleep(0.005)
        drain = asyncio.ensure_future(_drain(reader, writer, 0.1))
        assert await turn.hand_over()
        await writer
        await drain
    finally:
        os.close(reader)
    assert snapshots[-1] is None, f"last persisted input_write={snapshots[-1]}"
    assert turn.input_write is None


async def test_a_line_cut_at_kill_grace_keeps_its_byte_count(tmp_path: Path) -> None:
    group, reader = await _setup(tmp_path)
    snapshots: list[Any] = []
    turn = _turn(group, kill_grace=0.1, snapshots=snapshots)
    try:
        writer = asyncio.ensure_future(turn.offer_input("x" * LINE_CHARS))
        while group.writer.written == 0:
            await asyncio.sleep(0.005)
        assert await turn.hand_over()  # nobody reads: the write is still in flight at the grace
        await writer
    finally:
        os.close(reader)
    last = snapshots[-1]
    assert last is not None
    (line,) = turn.lines.values()
    assert isinstance(last["bytes"], int)
    assert 0 < last["bytes"] < len(line)
    assert last["uuid"] in turn.lines
