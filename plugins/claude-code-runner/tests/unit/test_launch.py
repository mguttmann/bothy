# SPDX-License-Identifier: Apache-2.0
"""The runner's side of the launch handshake (docs/spec/09-agents.md 18.19.4, 18.19.5).

The files of a launch and their modes, the start of a real keeper with one extra descriptor, the
go byte, the group of keeper and CLI as the runner writes into it and closes its stdin, the write
of a line that the CLI does not read (in flight, then aborted with its byte count), and the four
cases of a record in phase ``launching`` that the next runner settles.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import signal
import stat
import subprocess
import sys
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.launch import (
    KeeperGroup,
    LaunchFiles,
    PipeWriter,
    Settled,
    prepare_launch,
    process_identity,
    settle_launching,
    start_keeper,
)

pytestmark = pytest.mark.anyio

REF = "0192f4a0-0000-7000-8000-0000000000aa"

CLI = r"""
import json, sys, time
capture = sys.argv[1]
mode = sys.argv[2]
def note(**d):
    with open(capture, "a") as fh:
        fh.write(json.dumps(d) + "\n")
note(kind="start")
if mode == "sleep":
    time.sleep(600)
for raw in sys.stdin:
    note(kind="line", line=raw.rstrip("\n"))
note(kind="eof")
sys.exit(5)
"""


class Started(list[int]):
    """The groups a test started, with their leaders to reap."""

    def __init__(self) -> None:
        super().__init__()
        self.leaders: list[subprocess.Popen[bytes]] = []

    def add(self, process: subprocess.Popen[bytes]) -> None:
        self.append(process.pid)
        self.leaders.append(process)


@pytest.fixture
async def groups() -> AsyncIterator[Started]:
    started = Started()
    yield started
    for pgid in started:
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(pgid, signal.SIGKILL)
    for process in started.leaders:
        with contextlib.suppress(subprocess.TimeoutExpired):
            process.wait(PATIENCE)


def files_of(tmp_path: Path, attempt: int = 1, try_: int = 1) -> LaunchFiles:
    return LaunchFiles.of(tmp_path / "state", "run-1", attempt, try_)


def spec(tmp_path: Path, mode: str = "read") -> dict[str, Any]:
    script = tmp_path / "cli.py"
    script.write_text(CLI)
    return {
        # The session reference stands in the command line, as for the real CLI.
        "argv": [sys.executable, str(script), str(tmp_path / "cli.jsonl"), mode, REF],
        "env": {"PATH": "/usr/bin:/bin"},
        "cwd": str(tmp_path),
        "stdout": str(tmp_path / "stream.jsonl"),
        "stderr": str(tmp_path / "stderr.log"),
        "lock_fds": [],
    }


def notes(tmp_path: Path) -> list[dict[str, Any]]:
    path = tmp_path / "cli.jsonl"
    if not path.exists():
        return []
    return [json.loads(x) for x in path.read_text().splitlines() if x.strip()]


async def until(predicate: Any, timeout: float = PATIENCE) -> None:
    loop = asyncio.get_running_loop()
    deadline = loop.time() + timeout
    while loop.time() < deadline:
        if predicate():
            return
        await asyncio.sleep(0.01)
    raise AssertionError("condition not reached")


async def launched(
    tmp_path: Path, groups: Started, mode: str = "read", inputs: bytes = b'{"n":1}\n'
) -> tuple[LaunchFiles, KeeperGroup]:
    files = files_of(tmp_path)
    prepare_launch(files, inputs, spec(tmp_path, mode))
    started = start_keeper(files, compaction=False, lock_fds=())
    groups.add(started.process)
    await until(files.keeper.exists)
    started.go()
    await until(files.cli.exists)
    keeper = json.loads(files.keeper.read_text())
    cli = json.loads(files.cli.read_text())
    group = KeeperGroup(
        files,
        keeper={**keeper, **process_identity(keeper["pid"], PATIENCE)},
        cli={**cli, **process_identity(cli["pid"], PATIENCE)},
        session_ref=REF,
        timeout=PATIENCE,
        process=started.process,
    )
    return files, group


def test_launch_files_carry_run_attempt_and_try(tmp_path: Path) -> None:
    files = LaunchFiles.of(tmp_path, "r1", 2, 3)
    assert files.prefix == tmp_path / "turns" / "r1.2.3"
    assert files.stdin.name == "r1.2.3.stdin"
    assert {"stdin", "keeper", "cli", "exit", "fed"} <= set(files.record())


def test_prepare_writes_private_files_and_removes_an_earlier_try(tmp_path: Path) -> None:
    first = files_of(tmp_path, try_=1)
    prepare_launch(first, b"x\n", spec(tmp_path))
    second = files_of(tmp_path, try_=2)
    prepare_launch(second, b"x\n", spec(tmp_path), earlier=first)
    assert not first.stdin.exists()  # the named pipe of an earlier try is removed
    assert not first.spec.exists()  # and its spec, which may hold a key
    assert stat.S_ISFIFO(second.stdin.stat().st_mode)
    for path in (second.stdin, second.inputs, second.spec):
        assert stat.S_IMODE(path.stat().st_mode) == 0o600
    assert second.inputs.read_bytes() == b"x\n"


async def test_handshake_then_write_and_close_stdin(tmp_path: Path, groups: Started) -> None:
    files, group = await launched(tmp_path, groups)
    assert group.pgid != group.pid
    assert os.getpgid(group.pid) == group.pgid
    await until(files.fed.exists)
    assert group.alive()
    assert await group.write_bytes(b'{"n":2}\n')
    group.close_stdin()  # the runner's end, then SIGUSR1 to the keeper after its identity check
    assert await group.wait(PATIENCE, 0.01)
    assert [n["line"] for n in notes(tmp_path) if n["kind"] == "line"] == ['{"n":1}', '{"n":2}']
    assert group.exit_label() == "5"
    assert not await group.write_bytes(b"late\n")  # a closed stdin is never written


async def test_a_line_the_cli_does_not_read_is_aborted_with_its_byte_count(
    tmp_path: Path, groups: Started
) -> None:
    # 18.19.3: the write runs off the event loop with the write end non-blocking; at a handover
    # an unfinished line is recorded with the bytes written (partial_input).
    files, group = await launched(tmp_path, groups, mode="sleep", inputs=b"")
    await until(files.fed.exists)
    writer = group.writer
    line = b'{"x":"' + b"y" * (1 << 20) + b'"}\n'
    task = asyncio.ensure_future(group.write_bytes(line))
    await until(lambda: writer.written > 0)
    assert not task.done()
    written = await writer.abort()
    assert not await task
    assert 0 < written < len(line)
    assert writer.written == written


async def test_a_gone_reader_is_never_a_hang(tmp_path: Path) -> None:
    fifo = tmp_path / "x.stdin"
    os.mkfifo(fifo, 0o600)
    assert not await PipeWriter(fifo).write(b"line\n")  # ENXIO: the turn takes no more input


# ---- the cases of a record in phase launching (18.19.5) ----------------------------------------


async def test_go_not_sent_relaunches_after_the_keeper_left(
    tmp_path: Path, groups: Started
) -> None:
    files = files_of(tmp_path)
    prepare_launch(files, b"x\n", spec(tmp_path))
    started = start_keeper(files, compaction=False, lock_fds=())
    groups.add(started.process)
    await until(files.keeper.exists)
    os.close(started.go_w)  # the runner died before step 3
    settled = await settle_launching(
        files,
        go_sent=False,
        keeper=None,
        session_ref=REF,
        kill_grace=PATIENCE,
        poll=0.01,
        timeout=PATIENCE,
    )
    assert settled is Settled.RELAUNCH
    assert started.process.wait(PATIENCE) == 0
    assert not files.cli.exists()
    assert notes(tmp_path) == []


async def test_go_sent_with_a_live_cli_is_adopted(tmp_path: Path, groups: Started) -> None:
    files, group = await launched(tmp_path, groups)
    settled = await settle_launching(
        files,
        go_sent=True,
        keeper=group.keeper,
        session_ref=REF,
        kill_grace=PATIENCE,
        poll=0.01,
        timeout=PATIENCE,
    )
    assert settled is Settled.LIVE


async def test_go_sent_with_the_cli_gone_ended(tmp_path: Path, groups: Started) -> None:
    files, group = await launched(tmp_path, groups)
    group.close_stdin()
    assert await group.wait(PATIENCE, 0.01)
    settled = await settle_launching(
        files,
        go_sent=True,
        keeper=group.keeper,
        session_ref=REF,
        kill_grace=PATIENCE,
        poll=0.01,
        timeout=PATIENCE,
    )
    assert settled is Settled.ENDED


def fake_keeper(tmp_path: Path, files: LaunchFiles, groups: Started) -> subprocess.Popen[bytes]:
    """A keeper that wrote its identity file and hangs before it names a CLI."""
    process = subprocess.Popen(
        [sys.executable, "-c", "import time; time.sleep(600)", "--launch", str(files.prefix)],
        start_new_session=True,
    )
    groups.add(process)
    files.keeper.parent.mkdir(parents=True, exist_ok=True)
    files.keeper.write_text(json.dumps({"pid": process.pid, "pgid": process.pid}))
    return process


async def test_go_sent_without_cli_file_kills_the_keeper_and_relaunches(
    tmp_path: Path, groups: Started
) -> None:
    files = files_of(tmp_path)
    process = fake_keeper(tmp_path, files, groups)
    keeper = {"pid": process.pid, "pgid": process.pid, **process_identity(process.pid, PATIENCE)}
    settled = await settle_launching(
        files,
        go_sent=True,
        keeper=keeper,
        session_ref=REF,
        kill_grace=0.2,
        poll=0.01,
        timeout=PATIENCE,
    )
    assert settled is Settled.RELAUNCH
    assert process.wait(PATIENCE) == -signal.SIGKILL  # its group, which holds no CLI


async def test_a_cli_file_written_after_the_last_look_counts_as_a_cli_that_ran(
    tmp_path: Path, groups: Started
) -> None:
    # AR-T04: the keeper names a CLI after the runner's last look; the read after the group is
    # gone decides, so the attempt is never launched twice.
    files = files_of(tmp_path)
    process = fake_keeper(tmp_path, files, groups)
    keeper = {"pid": process.pid, "pgid": process.pid, **process_identity(process.pid, PATIENCE)}

    def late() -> None:
        files.cli.write_text(json.dumps({"pid": process.pid + 100000, "pgid": process.pid}))

    settled = await settle_launching(
        files,
        go_sent=True,
        keeper=keeper,
        session_ref=REF,
        kill_grace=0.2,
        poll=0.01,
        timeout=PATIENCE,
        after_wait=late,
    )
    assert settled is Settled.ENDED


async def test_a_keeper_the_record_does_not_name_is_never_signalled(
    tmp_path: Path, groups: Started
) -> None:
    # The keeper file names a process whose command is not this launch's keeper: no signal.
    files = files_of(tmp_path)
    other = subprocess.Popen(
        [sys.executable, "-c", "import time; time.sleep(600)"], start_new_session=True
    )
    groups.add(other)
    files.keeper.parent.mkdir(parents=True, exist_ok=True)
    files.keeper.write_text(json.dumps({"pid": other.pid, "pgid": other.pid}))
    settled = await settle_launching(
        files,
        go_sent=False,
        keeper=None,
        session_ref=REF,
        kill_grace=0.2,
        poll=0.01,
        timeout=PATIENCE,
    )
    assert settled is Settled.RELAUNCH
    assert other.poll() is None
