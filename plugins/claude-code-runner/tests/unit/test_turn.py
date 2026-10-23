# SPDX-License-Identifier: Apache-2.0
"""The turn process: own group, files instead of pipes, terminate escalation (09 18.2, 18.6)."""

from __future__ import annotations

import json
import os
import sys
import time
from pathlib import Path

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.turn import (
    Turn,
    remove_turn_record,
    terminate,
    turn_paths,
    write_turn_record,
)

RUN = "0192d3a0-0000-7000-8000-0000000000r1".replace("r", "f")
REF = "0192d3a0-0000-4000-8000-000000000001"


def start(tmp_path: Path, code: str) -> Turn:
    paths = turn_paths(tmp_path / "state", REF, RUN)
    cwd = tmp_path / "project"
    cwd.mkdir(exist_ok=True)
    return Turn.start(
        [sys.executable, "-c", code], cwd=cwd, env={"PATH": "/usr/bin"}, paths=paths, pass_fds=()
    )


def test_paths(tmp_path: Path) -> None:
    p = turn_paths(tmp_path, REF, RUN)
    assert p.stream == tmp_path / "sessions" / REF / f"{RUN}.stream.jsonl"
    assert p.stderr.name == f"{RUN}.stderr.log"
    assert p.system_prompt.name == f"{RUN}.system-prompt.md"
    assert p.input.name == f"{RUN}.input.jsonl"
    assert p.record == tmp_path / "turns" / f"{RUN}.json"


@pytest.mark.anyio
async def test_group_cwd_and_files(tmp_path: Path) -> None:
    code = (
        "import os, sys\n"
        "line = sys.stdin.readline()\n"
        "print(os.getcwd()); print(os.getpgid(0)); print(line.strip())\n"
        "sys.stderr.write('err\\n')\n"
    )
    turn = start(tmp_path, code)
    await turn.write_line('{"type":"user"}')
    turn.close_stdin()
    assert await turn.wait(PATIENCE)
    lines = turn.paths.stream.read_text().splitlines()
    assert Path(lines[0]).resolve() == (tmp_path / "project").resolve()
    assert int(lines[1]) == turn.pgid == turn.pid
    assert turn.pgid != os.getpgid(0)
    assert lines[2] == '{"type":"user"}'
    assert turn.paths.stderr.read_text() == "err\n"
    for f in (turn.paths.stream, turn.paths.stderr):
        assert f.stat().st_mode & 0o777 == 0o600
    assert turn.exit_label() == "0"


@pytest.mark.anyio
async def test_terminate_sigint_first(tmp_path: Path) -> None:
    code = (
        "import signal, sys, time\n"
        "signal.signal(signal.SIGINT, lambda *a: (print('int', flush=True), sys.exit(130)))\n"
        "print('up', flush=True)\n"
        "time.sleep(60)\n"
    )
    turn = start(tmp_path, code)
    await _until_line(turn, "up")
    await terminate(turn, stop_grace=5.0, kill_grace=5.0, poll=0.02)
    assert turn.poll() == 130
    assert "int" in turn.paths.stream.read_text()


@pytest.mark.anyio
async def test_terminate_escalates_to_sigkill(tmp_path: Path) -> None:
    code = (
        "import signal, time\n"
        "signal.signal(signal.SIGINT, signal.SIG_IGN)\n"
        "signal.signal(signal.SIGTERM, signal.SIG_IGN)\n"
        "print('up', flush=True)\n"
        "time.sleep(60)\n"
    )
    turn = start(tmp_path, code)
    await _until_line(turn, "up")
    begin = time.monotonic()
    await terminate(turn, stop_grace=0.3, kill_grace=0.3, poll=0.02)
    assert time.monotonic() - begin >= 0.6
    assert turn.exit_label() == "SIGKILL"


@pytest.mark.anyio
async def test_terminate_after_exit_is_harmless(tmp_path: Path) -> None:
    turn = start(tmp_path, "pass")
    assert await turn.wait(PATIENCE)
    await terminate(turn, stop_grace=0.1, kill_grace=0.1, poll=0.02)


def test_turn_record(tmp_path: Path) -> None:
    path = tmp_path / "turns" / "r.json"
    write_turn_record(path, {"run_id": RUN, "pid": 1})
    assert json.loads(path.read_text()) == {"run_id": RUN, "pid": 1}
    assert path.stat().st_mode & 0o777 == 0o600
    remove_turn_record(path)
    remove_turn_record(path)
    assert not path.exists()


async def _until_line(turn: Turn, text: str) -> None:
    import asyncio

    deadline = time.monotonic() + PATIENCE
    while time.monotonic() < deadline:
        if turn.paths.stream.exists() and text in turn.paths.stream.read_text():
            return
        await asyncio.sleep(0.01)
    raise AssertionError("line not seen")


@pytest.mark.anyio
async def test_large_stdin_line_does_not_block_the_loop(tmp_path: Path) -> None:
    import asyncio

    # The child reads stdin only after the event loop has ticked ten times during the write;
    # a write that blocked the loop would never open the gate and the wait would hang.
    gate = tmp_path / "gate"
    code = (
        "import pathlib, sys, time\n"
        f"gate = pathlib.Path({str(gate)!r})\n"
        "while not gate.exists():\n"
        "    time.sleep(0.01)\n"
        "sys.stdin.read()\n"
    )
    turn = start(tmp_path, code)
    ticks = 0

    async def ticker() -> None:
        nonlocal ticks
        while True:
            ticks += 1
            if ticks == 10:
                gate.touch()
            await asyncio.sleep(0.01)

    task = asyncio.ensure_future(ticker())
    await asyncio.wait_for(turn.write_line("x" * 400_000), PATIENCE)
    task.cancel()
    assert ticks >= 10  # the loop ran while the pipe was full
    turn.close_stdin()
    assert await turn.wait(PATIENCE)


@pytest.mark.anyio
async def test_terminate_ends_children_after_the_leader(tmp_path: Path) -> None:
    code = (
        "import subprocess, sys\n"
        "child = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(60)'])\n"
        "print(child.pid, flush=True)\n"
    )
    turn = start(tmp_path, code)
    assert await turn.wait(PATIENCE)
    child = int(turn.paths.stream.read_text().split()[0])
    os.kill(child, 0)  # still running after the leader exited
    await terminate(turn, stop_grace=1.0, kill_grace=1.0, poll=0.02)
    deadline = time.monotonic() + PATIENCE
    while time.monotonic() < deadline:
        try:
            os.kill(child, 0)
        except ProcessLookupError:
            return
        time.sleep(0.02)
    raise AssertionError("the child of the turn survived")
