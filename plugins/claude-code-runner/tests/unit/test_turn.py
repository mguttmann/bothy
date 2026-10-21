# SPDX-License-Identifier: Apache-2.0
"""The turn process: own group, files instead of pipes, terminate escalation (09 18.2, 18.6)."""

from __future__ import annotations

import json
import os
import sys
import time
from pathlib import Path

import pytest

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
    turn.write_line('{"type":"user"}')
    turn.close_stdin()
    assert await turn.wait(10)
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
    assert await turn.wait(10)
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

    for _ in range(500):
        if turn.paths.stream.exists() and text in turn.paths.stream.read_text():
            return
        await asyncio.sleep(0.01)
    raise AssertionError("line not seen")
