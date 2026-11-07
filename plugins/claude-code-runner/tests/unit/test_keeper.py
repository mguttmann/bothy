# SPDX-License-Identifier: Apache-2.0
"""The keeper ``python -m claude_code_runner.keeper`` (docs/spec/09-agents.md 18.19.4, 18.19.5).

Each test starts a real keeper as the runner does (its own session, the read end of a launch
pipe as its only extra descriptor) with a small fake CLI, and checks one item of 18.19.4: the
session and group, the named pipe, the identity files and the release of the CLI only after
``<launch>.cli``, the initial inputs written once without blocking and ``<launch>.fed`` only after
the last byte, ignored signals, the wait status in ``<launch>.exit`` and the exit code, SIGUSR1,
a launch pipe closed without a go byte, a CLI that stops reading inside a large input, and that
the keeper holds no lock descriptor after the fork.
"""

from __future__ import annotations

import contextlib
import fcntl
import json
import os
import shutil
import signal
import subprocess
import sys
import time
from collections.abc import Iterator
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.testing._patience import PATIENCE

SRC = Path(__file__).resolve().parents[2] / "src"

FAKE_CLI = r"""
import json, os, signal, sys, time
capture, mode = sys.argv[1], sys.argv[2]
launch = sys.argv[3]
def note(**data):
    with open(capture, "a") as fh:
        fh.write(json.dumps(data) + "\n")
cli = json.load(open(launch + ".cli"))
note(kind="start", pid=os.getpid(), pgid=os.getpgid(0), named=cli["pid"] == os.getpid(),
     sigint=getattr(signal.getsignal(signal.SIGINT), "name", "python"),
     sigterm=getattr(signal.getsignal(signal.SIGTERM), "name", "python"))
if mode == "read_one_then_exit":
    note(kind="line", line=sys.stdin.buffer.readline(64).decode(errors="replace"))
    sys.exit(4)
if mode == "never_read":
    while True:
        time.sleep(0.05)
if mode == "kill_self":
    os.kill(os.getpid(), signal.SIGKILL)
for raw in sys.stdin:
    note(kind="line", line=raw.rstrip("\n"))
note(kind="eof")
sys.exit(3)
"""


@dataclass
class Launch:
    """One started keeper and the files of its launch."""

    prefix: Path
    keeper: subprocess.Popen[bytes]
    go_w: int
    capture: Path

    def file(self, suffix: str) -> Path:
        return Path(f"{self.prefix}.{suffix}")

    def read(self, suffix: str) -> dict[str, Any]:
        return json.loads(self.file(suffix).read_text())

    def notes(self) -> list[dict[str, Any]]:
        if not self.capture.exists():
            return []
        return [json.loads(x) for x in self.capture.read_text().splitlines() if x.strip()]

    def go(self) -> None:
        os.write(self.go_w, b"g")
        os.close(self.go_w)
        self.go_w = -1


def until(predicate: Any, timeout: float = PATIENCE) -> None:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(0.01)
    raise AssertionError("condition not reached")


def alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


@pytest.fixture
def launches(tmp_path: Path) -> Iterator[list[Launch]]:
    started: list[Launch] = []
    yield started
    for item in started:
        if item.go_w >= 0:
            with contextlib.suppress(OSError):
                os.close(item.go_w)
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(item.keeper.pid, signal.SIGKILL)
        with contextlib.suppress(subprocess.TimeoutExpired):
            item.keeper.wait(PATIENCE)


def start(
    tmp_path: Path,
    launches: list[Launch],
    mode: str = "read_all",
    inputs: bytes = b"",
    *,
    compaction: bool = False,
    lock_fds: tuple[int, ...] = (),
    fifo: bool = True,
) -> Launch:
    """Start a keeper the way the runner does (18.19.5 steps 1 and 2)."""
    prefix = tmp_path / "turns" / f"run.1.{len(launches) + 1}"
    prefix.parent.mkdir(mode=0o700, exist_ok=True)
    capture = tmp_path / f"cli-{len(launches) + 1}.jsonl"
    script = tmp_path / "fake_cli.py"
    script.write_text(FAKE_CLI)
    if fifo:
        os.mkfifo(f"{prefix}.stdin", 0o600)
    Path(f"{prefix}.inputs").write_bytes(inputs)
    spec = {
        "argv": [sys.executable, str(script), str(capture), mode, str(prefix)],
        "env": {"PATH": "/usr/bin:/bin", "LANG": "C"},
        "cwd": str(tmp_path),
        "stdout": str(tmp_path / "stream.jsonl"),
        "stderr": str(tmp_path / "stderr.log"),
        "lock_fds": list(lock_fds),
    }
    spec_path = Path(f"{prefix}.spec")
    spec_path.write_text(json.dumps(spec))
    spec_path.chmod(0o600)
    go_r, go_w = os.pipe()
    argv = [sys.executable, "-m", "claude_code_runner.keeper", "--launch", str(prefix)]
    argv += ["--go-fd", str(go_r)]
    if compaction:
        argv.append("--compaction")
    keeper = subprocess.Popen(
        argv,
        # COVERAGE_*: a coverage run measures the keeper process too (patch = subprocess).
        env={"PATH": "/usr/bin:/bin", "PYTHONPATH": str(SRC), **coverage_env()},
        stdin=subprocess.DEVNULL,
        start_new_session=True,
        pass_fds=(go_r, *lock_fds),
        close_fds=True,
    )
    os.close(go_r)
    launch = Launch(prefix, keeper, go_w, capture)
    launches.append(launch)
    return launch


def coverage_env() -> dict[str, str]:
    """The ``COVERAGE_*`` variables of this process, so that a child process is measured."""
    return {k: v for k, v in os.environ.items() if k.startswith("COVERAGE_")}


def runner_end(launch: Launch) -> int:
    """The runner's own write end (18.19.4): ``O_WRONLY | O_NONBLOCK``."""
    return os.open(launch.file("stdin"), os.O_WRONLY | os.O_NONBLOCK)


def test_keeper_leads_its_session_and_names_the_cli_before_it_runs(
    tmp_path: Path, launches: list[Launch]
) -> None:
    launch = start(tmp_path, launches, inputs=b'{"n":1}\n')
    until(lambda: launch.file("keeper").exists())
    ident = launch.read("keeper")
    assert ident["pid"] == launch.keeper.pid
    assert os.getpgid(launch.keeper.pid) == launch.keeper.pid
    assert os.getsid(launch.keeper.pid) == launch.keeper.pid
    assert not launch.file("cli").exists()  # no CLI before the go
    assert not launch.file("spec").exists()  # the spec (it may hold a key) is read and removed
    assert stat_mode(launch.file("keeper")) == 0o600
    launch.go()
    until(lambda: any(n["kind"] == "line" for n in launch.notes()))
    [first] = [n for n in launch.notes() if n["kind"] == "start"]
    assert first["named"] is True  # the CLI ran only once <launch>.cli named it
    assert first["pgid"] == launch.keeper.pid  # in the keeper's group
    assert first["pid"] != launch.keeper.pid
    assert launch.read("cli") == {"pid": first["pid"], "pgid": launch.keeper.pid}
    # The default handlers restored (an interpreter installs its own for SIGINT unless the
    # signal was ignored when it started).
    assert first["sigint"] != "SIG_IGN"
    assert first["sigterm"] == "SIG_DFL"
    until(lambda: launch.file("fed").exists())
    fd = runner_end(launch)
    os.write(fd, b'{"n":2}\n')
    os.close(fd)
    os.kill(launch.keeper.pid, signal.SIGUSR1)  # close stdin: the keeper closes its write end
    assert launch.keeper.wait(PATIENCE) == 3
    lines = [n["line"] for n in launch.notes() if n["kind"] == "line"]
    assert lines == ['{"n":1}', '{"n":2}']  # the initial input exactly once, before the rest
    exit_file = launch.read("exit")
    assert exit_file["code"] == 3
    assert os.waitstatus_to_exitcode(exit_file["status"]) == 3
    assert exit_file["label"] == "3"


def stat_mode(path: Path) -> int:
    return path.stat().st_mode & 0o777


def test_keeper_ignores_int_term_hup_and_the_group_stop_reaches_the_cli(
    tmp_path: Path, launches: list[Launch]
) -> None:
    launch = start(tmp_path, launches, mode="never_read")
    until(lambda: launch.file("keeper").exists())
    launch.go()
    until(lambda: any(n["kind"] == "start" for n in launch.notes()))
    for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
        os.kill(launch.keeper.pid, sig)
    time.sleep(0.3)
    assert launch.keeper.poll() is None  # the keeper ignores them
    cli = launch.read("cli")["pid"]
    assert alive(cli)
    os.killpg(launch.keeper.pid, signal.SIGTERM)  # 18.6: SIGTERM to the one group
    assert launch.keeper.wait(PATIENCE) == 128 + signal.SIGTERM
    assert launch.read("exit")["label"] == "SIGTERM"


def test_cli_killed_by_a_signal_gives_128_plus_its_number(
    tmp_path: Path, launches: list[Launch]
) -> None:
    launch = start(tmp_path, launches, mode="kill_self")
    until(lambda: launch.file("keeper").exists())
    launch.go()
    assert launch.keeper.wait(PATIENCE) == 128 + signal.SIGKILL
    exit_file = launch.read("exit")
    assert exit_file["label"] == "SIGKILL"
    assert os.WIFSIGNALED(exit_file["status"])


def test_launch_pipe_closed_without_go_starts_no_cli(
    tmp_path: Path, launches: list[Launch]
) -> None:
    launch = start(tmp_path, launches)
    until(lambda: launch.file("keeper").exists())
    os.close(launch.go_w)  # the runner died, or handed over before step 3
    launch.go_w = -1
    launch.keeper.wait(PATIENCE)
    assert launch.read("exit")["label"] == "not_started"
    assert not launch.file("cli").exists()
    assert launch.notes() == []


def test_keeper_without_its_named_pipe_does_not_start(
    tmp_path: Path, launches: list[Launch]
) -> None:
    launch = start(tmp_path, launches, fifo=False)
    launch.keeper.wait(PATIENCE)
    assert launch.read("exit")["label"] == "not_started"
    assert not launch.file("keeper").exists()


def test_cli_that_stops_reading_inside_a_large_input_never_blocks_the_keeper(
    tmp_path: Path, launches: list[Launch]
) -> None:
    # 18.19.4 item 3: the inputs are written without blocking in a loop that watches the CLI's
    # exit; a CLI that ends first gets no <launch>.fed.
    big = b'{"type":"user","text":"' + b"x" * (1 << 20) + b'"}\n'
    launch = start(tmp_path, launches, mode="read_one_then_exit", inputs=big)
    until(lambda: launch.file("keeper").exists())
    launch.go()
    assert launch.keeper.wait(PATIENCE) == 4
    assert not launch.file("fed").exists()
    assert launch.read("exit")["code"] == 4


def test_compaction_keeper_closes_its_write_end_after_the_inputs(
    tmp_path: Path, launches: list[Launch]
) -> None:
    launch = start(tmp_path, launches, inputs=b'{"c":"/compact"}\n', compaction=True)
    until(lambda: launch.file("keeper").exists())
    launch.go()
    assert launch.keeper.wait(PATIENCE) == 3  # end of input after the one line
    assert launch.file("fed").exists()
    assert [n["line"] for n in launch.notes() if n["kind"] == "line"] == ['{"c":"/compact"}']


def test_a_keeper_killed_closes_its_write_end(tmp_path: Path, launches: list[Launch]) -> None:
    # 18.19.4: a SIGKILL to the keeper alone closes its write end; the CLI sees end of input
    # once the runner's end is closed too, and no exit file is written (exit_label unknown).
    launch = start(tmp_path, launches)
    until(lambda: launch.file("keeper").exists())
    launch.go()
    until(lambda: launch.file("fed").exists())
    cli = launch.read("cli")["pid"]
    os.kill(launch.keeper.pid, signal.SIGKILL)
    launch.keeper.wait(PATIENCE)
    until(lambda: any(n["kind"] == "eof" for n in launch.notes()))
    until(lambda: not alive(cli))
    assert not launch.file("exit").exists()


def _open_fds(pid: int) -> str:
    lsof = shutil.which("lsof", path="/usr/sbin:/usr/bin:/sbin:/bin")
    fds = Path(f"/proc/{pid}/fd")
    if fds.is_dir():
        return "\n".join(str(fd.readlink()) for fd in fds.iterdir())
    assert lsof, "lsof is needed to list the keeper's descriptors on this platform"
    out = subprocess.run(
        [lsof, "-n", "-P", "-p", str(pid)], capture_output=True, text=True, check=False
    )
    return out.stdout


def test_keeper_holds_no_lock_descriptor_and_the_locks_free_after_the_cli(
    tmp_path: Path, launches: list[Launch]
) -> None:
    # Design note 8 of the brief: the CLI gets the lock descriptors as today; the keeper closes
    # its copies after the fork, so a new runner can take the locks once the CLI exited.
    lock_path = tmp_path / "thread-x.lock"
    lock_fd = os.open(lock_path, os.O_RDWR | os.O_CREAT, 0o600)
    fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    launch = start(tmp_path, launches, lock_fds=(lock_fd,))
    os.close(lock_fd)  # the runner hands its lock over to the turn (as with Popen today)
    until(lambda: launch.file("keeper").exists())
    launch.go()
    until(lambda: launch.file("fed").exists())
    cli = launch.read("cli")["pid"]
    assert str(lock_path) not in _open_fds(launch.keeper.pid)
    assert str(lock_path) in _open_fds(cli)
    probe = os.open(lock_path, os.O_RDWR)
    try:
        with pytest.raises(BlockingIOError):
            fcntl.flock(probe, fcntl.LOCK_EX | fcntl.LOCK_NB)  # the CLI holds it
        os.kill(launch.keeper.pid, signal.SIGUSR1)
        launch.keeper.wait(PATIENCE)
        until(lambda: not alive(cli))
        fcntl.flock(probe, fcntl.LOCK_EX | fcntl.LOCK_NB)  # free once the CLI exited
    finally:
        os.close(probe)
