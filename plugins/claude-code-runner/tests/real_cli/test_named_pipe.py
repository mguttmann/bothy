# SPDX-License-Identifier: Apache-2.0
"""The pinned CLI with a named pipe as stdin (09 18.19.4, 18.19.12, row "Named pipe as stdin").

Collected only with ``CLAUDE_CODE_RUNNER_REAL_CLI=1`` (``tests/conftest.py``); free: no login and no
model call. The CLI runs against a local stand-in for the API on ``127.0.0.1``
(``real_cli/stand_in.py``, the method of ``tests/real_cli/captured``: ``ANTHROPIC_BASE_URL`` on
loopback, a dummy key, its own ``CLAUDE_CONFIG_DIR``).

1. With the named pipe as its stdin directly, the pinned CLI reads and answers every line, also one
   written by a second process that opened the pipe again, but on macOS it does not end when the
   last write end closes (the same CLI ends at once on an anonymous pipe). This is why the keeper
   relays the named pipe into an anonymous pipe that is the CLI's stdin.
2. Under the real keeper (``python -m claude_code_runner.keeper``): the initial input, a line of
   the runner and a line of a second process are each echoed and answered in their order, and
   "close stdin" (the runner's end closed, SIGUSR1 to the keeper) ends the CLI with exit 0.
"""

from __future__ import annotations

import json
import os
import signal
import subprocess
import sys
import tempfile
import time
import uuid
from pathlib import Path
from typing import Any

from real_cli.stand_in import cli_env, stand_in, user_line
from real_cli.test_real_cli import claude_bin

WAIT = 120.0
"""The bound of every wait of this test (seconds)."""


def _until_results(out: Path, count: int) -> list[dict[str, Any]]:
    deadline = time.monotonic() + WAIT
    while time.monotonic() < deadline:
        messages = [json.loads(x) for x in out.read_text().splitlines() if x.startswith("{")]
        if sum(1 for m in messages if m.get("type") == "result") >= count:
            return messages
        time.sleep(0.1)
    raise AssertionError(f"no {count} results; stream: {out.read_text()[-2000:]}")


def _argv() -> list[str]:
    return [
        claude_bin(),
        "-p",
        "--input-format",
        "stream-json",
        "--output-format",
        "stream-json",
        "--verbose",
        "--replay-user-messages",
        "--session-id",
        str(uuid.uuid4()),
        "--model",
        "claude-haiku-4-5",
        "--permission-mode",
        "dontAsk",
        "--tools",
        "",
        "--permission-prompts",
        "none",
    ]


_SECOND_WRITER = (
    "import os, sys\n"
    "fd = os.open(sys.argv[1], os.O_WRONLY | os.O_NONBLOCK)\n"
    "os.write(fd, sys.argv[2].encode())\n"
    "os.close(fd)\n"
)


def _write_from_another_process(fifo: Path, line: bytes) -> None:
    subprocess.run(
        [sys.executable, "-c", _SECOND_WRITER, str(fifo), line.decode()], check=True, timeout=WAIT
    )


def _answered(messages: list[dict[str, Any]], uuids: list[str]) -> None:
    echoes = [m.get("uuid") for m in messages if m.get("isReplay")]
    assert echoes[: len(uuids)] == uuids
    results = [m for m in messages if m.get("type") == "result"]
    assert [r.get("is_error") for r in results] == [False] * len(uuids)
    assert [r.get("result") for r in results] == [f"ok {n + 1}" for n in range(len(uuids))]
    for result, line_uuid in zip(results, uuids, strict=True):
        named = result.get("user_message_uuids") or [result.get("user_message_uuid")]
        assert line_uuid in named


def test_named_pipe_directly_as_stdin() -> None:
    """Lines through a named pipe are read and answered; on macOS its end does not end the CLI."""
    with tempfile.TemporaryDirectory() as tmp, stand_in() as (url, handler):
        root = Path(tmp)
        fifo = root / "launch.stdin"
        os.mkfifo(fifo, 0o600)
        env = cli_env(root, url)
        read_end = os.open(fifo, os.O_RDONLY | os.O_NONBLOCK)
        keeper_end = os.open(fifo, os.O_WRONLY)
        os.set_blocking(read_end, True)
        out = root / "stream.jsonl"
        with out.open("wb") as stdout:
            cli = subprocess.Popen(
                _argv(),
                cwd=root,
                env=env,
                stdin=read_end,
                stdout=stdout,
                stderr=subprocess.DEVNULL,
            )
        os.close(read_end)
        try:
            first_uuid, first = user_line("Reply with one word.")
            writer = os.open(fifo, os.O_WRONLY | os.O_NONBLOCK)
            os.write(writer, first)
            os.close(writer)
            _until_results(out, 1)
            second_uuid, second = user_line("Reply with one more word.")
            _write_from_another_process(fifo, second)
            messages = _until_results(out, 2)
            os.close(keeper_end)  # the last write end of the named pipe
            keeper_end = -1
            try:
                ended: int | None = cli.wait(timeout=10)
            except subprocess.TimeoutExpired:
                ended = None
        finally:
            if keeper_end >= 0:
                os.close(keeper_end)
            if cli.poll() is None:
                cli.kill()
                cli.wait()
        _answered(messages, [first_uuid, second_uuid])
        assert handler.answers == ["ok 1", "ok 2"]
        if sys.platform == "darwin":
            assert ended is None  # the finding of 18.19.12: no end of input from a named pipe


def test_named_pipe_through_the_keeper() -> None:
    """The keeper relays the named pipe into the CLI's stdin; close stdin ends the CLI."""
    from claude_code_runner.launch import LaunchFiles, prepare_launch, start_keeper

    with tempfile.TemporaryDirectory() as tmp, stand_in() as (url, handler):
        root = Path(tmp)
        files = LaunchFiles.of(root / "state", "run", 1, 1)
        initial_uuid, initial = user_line("Reply with one word.")
        out = root / "stream.jsonl"
        spec = {
            "argv": _argv(),
            "env": cli_env(root, url),
            "cwd": str(root),
            "stdout": str(out),
            "stderr": str(root / "stderr.log"),
            "lock_fds": [],
        }
        prepare_launch(files, initial, spec)
        started = start_keeper(files, compaction=False, lock_fds=())
        try:
            deadline = time.monotonic() + WAIT
            while not files.keeper.exists() and time.monotonic() < deadline:
                time.sleep(0.05)
            started.go()
            while not files.fed.exists() and time.monotonic() < deadline:
                time.sleep(0.05)
            _until_results(out, 1)
            runner_uuid, runner_line = user_line("Reply with one more word.")
            writer = os.open(files.stdin, os.O_WRONLY | os.O_NONBLOCK)  # the runner's end
            os.write(writer, runner_line)
            _until_results(out, 2)
            adopter_uuid, adopter_line = user_line("And one last word.")
            _write_from_another_process(files.stdin, adopter_line)  # an adopting runner
            messages = _until_results(out, 3)
            os.close(writer)
            os.kill(started.process.pid, signal.SIGUSR1)  # close stdin (18.19.4 item 5)
            assert started.process.wait(timeout=WAIT) == 0  # the keeper exits with the CLI's code
        finally:
            if started.process.poll() is None:
                os.killpg(started.process.pid, signal.SIGKILL)
                started.process.wait()
        _answered(messages, [initial_uuid, runner_uuid, adopter_uuid])
        assert handler.answers == ["ok 1", "ok 2", "ok 3"]
        assert '"label": "0"' in files.exit.read_text()
