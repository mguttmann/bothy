# SPDX-License-Identifier: Apache-2.0
"""The scenario world: FakeServer plus Fake-Claude (design note 17, brief "Scenarios").

Space Engineering with the forums Projekt-A (binding ``website``, a Git working tree under the
test's projects root) and Projekt-B (binding ``docs``, directory missing) and the channel
Werkzeuge without a binding. The agent under test is the FakeServer's agent (Lotse there);
Alex triggers, Robin is the owner, Sam has no ``trigger`` (his messages are ``unverified``),
Texter is another agent. The runner runs on the FakeServer's clock, which a background task
moves forward; the runner's own waits (polling, locks, process grace) are real time.
"""

from __future__ import annotations

import asyncio
import contextlib
import dataclasses
import errno
import json
import logging
import os
import signal
import stat
import subprocess
import sys
from collections.abc import AsyncIterator, Callable, Iterable, Iterator
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.sources import EventSource
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.doubles import FakeClock
from bothy_sdk.testing.fakeserver import FakeServer

from claude_code_runner.app import Runner
from claude_code_runner.config import RunnerConfig, load_config

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import helper

API_KEY = "sk-ant-canary-0123456789"


ERROR_CANARY = "ERROR-CANARY-4e9b"
"""A text in the message of the error `fail_turn_records` raises (the file it names)."""


def fail_turn_records(monkeypatch: pytest.MonkeyPatch) -> None:
    """From now on no turn record can be written, as in a state folder without space left.

    The next write raises `OSError` inside the run that makes it, at a place where the runner
    expects no error: the run ends by an unexpected error (09 18.9). Its message names a file,
    as the message of such an error does (`ERROR_CANARY`).
    """

    def no_space(*_: Any) -> None:
        raise OSError(errno.ENOSPC, os.strerror(errno.ENOSPC), f"/state/{ERROR_CANARY}.json")

    monkeypatch.setattr("claude_code_runner.runturn.write_turn_record", no_space)


def live_entries(paths: Iterable[Path]) -> Iterator[tuple[Path, os.stat_result]]:
    """Yield each listed entry with its ``lstat``; skip one that is gone by then.

    A scenario looks at ``runner.state_dir`` while the runner still works: SQLite creates and
    removes the rollback journal of the inbox (``inbox.sqlite3-journal``) around every
    transaction, so an entry of a listing can vanish before it is looked at. What still exists
    is always returned, so a check over it misses nothing that is on disk.
    """
    for path in paths:
        try:
            info = path.lstat()
        except FileNotFoundError:
            continue
        yield path, info


def live_files(paths: Iterable[Path]) -> list[tuple[Path, bytes]]:
    """The content of each listed regular file that still exists (see `live_entries`)."""
    found: list[tuple[Path, bytes]] = []
    for path, info in live_entries(paths):
        if not stat.S_ISREG(info.st_mode):
            continue
        try:
            found.append((path, path.read_bytes()))
        except FileNotFoundError:
            continue
    return found


def assert_private(root: Path) -> set[str]:
    """Assert mode ``0700`` for every directory and ``0600`` for every file under ``root``.

    Returns:
        The checked entries, relative to ``root``.
    """
    checked: set[str] = set()
    for path, info in live_entries(root.rglob("*")):
        want = 0o700 if stat.S_ISDIR(info.st_mode) else 0o600
        assert stat.S_IMODE(info.st_mode) == want, path
        checked.add(str(path.relative_to(root)))
    return checked


@dataclasses.dataclass
class World:
    """One scenario world in a temporary directory."""

    fake: FakeServer
    root: Path
    projects: Path
    state: Path
    home: Path
    bin_dir: Path
    capture: Path
    key_file: Path
    token_file: Path
    forum_a: UUID
    forum_b: UUID
    werkzeuge: UUID
    claude: Path
    runners: list[tuple[Runner, asyncio.Task[Any]]] = dataclasses.field(default_factory=list)
    processes: list[subprocess.Popen[bytes]] = dataclasses.field(default_factory=list)

    def environ(self, **extra: str) -> dict[str, str]:
        env = {
            "HOME": str(self.home),
            "USER": "alex",
            "LOGNAME": "alex",
            "SHELL": "/bin/sh",
            "PATH": f"{self.bin_dir}:/usr/bin:/bin",
            "LANG": "C.UTF-8",
            "TERM": "dumb",
            "TMPDIR": str(self.root),
        }
        env.update(extra)
        return env

    def config(self, **runner: Any) -> RunnerConfig:
        values: dict[str, Any] = {
            "server_url": self.fake.origin,
            "instance_id": str(self.fake.instance_id),
            "agent_token_ref": f"file:{self.token_file}",
            "anthropic_api_key_ref": f"file:{self.key_file}",
            "default_model": "model-large",
            "projects_root": str(self.projects),
            "state_dir": str(self.state),
            "tool_class": "scoped",
            "scoped_allowed_tools": ["Read", "Bash(git status)"],
            "poll_interval": "20ms",
            "stop_grace": "1s",
            "kill_grace": "1s",
            # A wait of the runner never decides a test by the machine's speed: a test of the
            # limit sets a short value itself, every other one waits as long as the patience.
            "session_lock_wait": f"{int(PATIENCE)}s",
            "cli_probe_timeout": f"{int(PATIENCE)}s",
        }
        values.update(runner)
        lines = ["[runner]"]
        for key, value in values.items():
            lines.append(f"{key} = {json.dumps(value)}")
        path = self.root / "runner.toml"
        path.write_text("\n".join(lines) + "\n")
        return dataclasses.replace(load_config(path), sdk=self.fake.sdk_settings)

    def scenario(self, **scenario: Any) -> None:
        helper.update(self.claude, scenario)

    def turns(self) -> list[dict[str, Any]]:
        return helper.turns(self.capture)

    def calls(self, kind: str) -> list[dict[str, Any]]:
        return [c for c in helper.read_capture(self.capture) if c["kind"] == kind]

    async def start(
        self,
        config: RunnerConfig | None = None,
        wrap: Callable[[EventSource], EventSource] | None = None,
        **env: str,
    ) -> Runner:
        runner = Runner(config or self.config(), self.environ(**env), clock=self.fake.clock)
        adapter = await runner.build()
        if wrap is not None:
            adapter.source = wrap(await adapter.make_source())
        task = asyncio.ensure_future(runner.run())
        self.runners.append((runner, task))
        await self.until(
            lambda: any(h.get("details", {}).get("adapter") for h in self.fake.heartbeats)
        )
        return runner

    def spawn(self, **runner: Any) -> subprocess.Popen[bytes]:
        """Start the runner as a process of its own (``python -m claude_code_runner run``).

        It reads the configuration file `config` writes with ``runner`` (its ``[sdk]`` keys are
        the defaults) and writes its log lines to ``runner-<n>.log`` in the world's directory.
        """
        self.config(**runner)
        return self.spawn_command(
            [*self.cli_command(), "run", "--config", str(self.root / "runner.toml")]
        )

    def cli_command(self) -> list[str]:
        """The runner's command line (``python -m claude_code_runner``)."""
        return [sys.executable, "-m", "claude_code_runner"]

    def cli_env(self) -> dict[str, str]:
        """The environment of a runner process of its own."""
        # COVERAGE_*: a coverage run measures the runner process too (patch = subprocess).
        coverage = {k: v for k, v in os.environ.items() if k.startswith("COVERAGE_")}
        return {**self.environ(), "PYTHONPATH": os.pathsep.join(sys.path), **coverage}

    def spawn_command(self, argv: list[str]) -> subprocess.Popen[bytes]:
        """Start ``argv`` as a runner process in its own session, logging to ``runner-<n>.log``.

        The runner is the leader of its own process group, as under a service manager.
        """
        log = (self.root / f"runner-{len(self.processes)}.log").open("wb")
        process = subprocess.Popen(
            argv,
            env=self.cli_env(),
            stdin=subprocess.DEVNULL,
            stdout=log,
            stderr=log,
            start_new_session=True,
        )
        log.close()
        self.processes.append(process)
        return process

    async def stop(self, runner: Runner) -> None:
        for item in list(self.runners):
            if item[0] is runner:
                self.runners.remove(item)
                await runner.stop()
                with contextlib.suppress(Exception):
                    await asyncio.wait_for(item[1], PATIENCE)

    def gate(self, name: str) -> Path:
        """Return the path of a Fake-Claude gate (``auth_gate``, ``turn_gate``), not yet open."""
        return self.root / f"{name}.open"

    def release(self, gate: Path) -> None:
        """Open a gate: every Fake-Claude command waiting on it goes on."""
        gate.touch()

    async def until(self, predicate: Callable[[], Any], timeout: float = PATIENCE) -> None:
        loop = asyncio.get_running_loop()
        deadline = loop.time() + timeout
        while loop.time() < deadline:
            if predicate():
                return
            await asyncio.sleep(0.02)
        raise AssertionError("condition not reached")

    def state_of(self, run_id: UUID) -> str:
        return self.fake.run_state(run_id).state

    async def ended(self, run_id: UUID, timeout: float = PATIENCE) -> str:
        await self.until(
            lambda: self.state_of(run_id) in ("completed", "failed", "cancelled"), timeout
        )
        return self.fake.run_state(run_id).state

    def requests(self, operation: str) -> list[Any]:
        return [r for r in self.fake.requests if r.operation_id == operation]


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture
def world_timings() -> dict[str, float]:
    """FakeServer timings of a module's world (a module overrides this fixture)."""
    return {}


@pytest.fixture
def world_real_time() -> bool:
    """True: the FakeServer clock starts at the real time and follows it (a runner process
    of its own uses the system clock); else it starts in the past and runs fast."""
    return False


@pytest.fixture
async def world(
    tmp_path: Path, world_timings: dict[str, float], world_real_time: bool
) -> AsyncIterator[World]:
    root = tmp_path / "w"
    root.mkdir(mode=0o700)
    projects = root / "projects"
    (projects / "website" / ".git").mkdir(parents=True)
    home = root / "home"
    (home / ".claude").mkdir(parents=True)
    # A subscription login of the runner user: organization must never fall back to it (BR-12).
    (home / ".claude" / "fake-login").write_text("claude.ai")
    bin_dir = root / "bin"
    capture = root / "capture.jsonl"
    claude = helper.install(bin_dir, {"fixture": "happy"}, capture)
    key_file = root / "api_key"
    key_file.write_text(API_KEY + "\n")
    key_file.chmod(0o600)
    clock = FakeClock(datetime.now(UTC)) if world_real_time else None
    async with FakeServer(dpop=False, costs=True, timings=world_timings, clock=clock) as fake:
        token_file = root / "agent_token"
        token_file.write_text(fake.credentials.api_token.reveal())
        token_file.chmod(0o600)
        w = World(
            fake=fake,
            root=root,
            projects=projects,
            state=root / "state",
            home=home,
            bin_dir=bin_dir,
            capture=capture,
            key_file=key_file,
            token_file=token_file,
            forum_a=fake.add_forum("Projekt-A", "website", prompt="Answer in short sentences."),
            forum_b=fake.add_forum("Projekt-B", "docs"),
            werkzeuge=fake.add_forum("Werkzeuge", None),
            claude=claude,
        )
        pump = asyncio.ensure_future(_real_pump(fake) if world_real_time else _pump(fake))
        try:
            yield w
        finally:
            for runner, task in list(w.runners):
                await runner.stop()
                with contextlib.suppress(Exception):
                    await asyncio.wait_for(task, PATIENCE)
            for process in w.processes:
                _kill_group(process.pid)
                with contextlib.suppress(subprocess.TimeoutExpired):
                    process.wait(PATIENCE)
            pump.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await pump
            # No process group that the test or the runner under test started is left.
            left = [g for g in _started_groups(w) if _group_alive(g)]
            for group in left:
                _kill_group(group)
            assert left == [], "a process group of the scenario is left"


def _started_groups(w: World) -> set[int]:
    # A turn's group is its own, or with runner.turn_handover its keeper's (09 18.19.4).
    groups = {c.get("pgid", c["pid"]) for c in w.calls("turn_start")}
    keepers = set()
    turns = w.state / "turns"
    for path in turns.glob("*.keeper") if turns.is_dir() else []:
        with contextlib.suppress(OSError, ValueError, KeyError, TypeError):
            keepers.add(int(json.loads(path.read_text())["pid"]))
    return groups | keepers | {p.pid for p in w.processes}


def _group_alive(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
    except (ProcessLookupError, PermissionError):
        return False
    return True


def _kill_group(pgid: int) -> None:
    # Only groups this test or the runner under test started (their leader's PID is the group).
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pgid, signal.SIGKILL)


async def _real_pump(fake: FakeServer) -> None:
    """Move the shared clock with the real time, so a runner process of its own agrees."""
    loop = asyncio.get_running_loop()
    last = loop.time()
    while True:
        await asyncio.sleep(0.02)
        now = loop.time()
        await fake.clock.advance(now - last)
        last = now


async def _pump(fake: FakeServer) -> None:
    """Move the shared clock, so throttles, heartbeats and renewals of the SDK run."""
    while True:
        await fake.clock.advance(0.05)
        await asyncio.sleep(0.01)


class LogLines(logging.Handler):
    """Every log record of the scenario, kept by a handler of its own on the root logger.

    pytest's ``caplog`` stays empty for the records of these async scenarios, so a check of
    log lines needs its own handler (``records`` and ``text`` like ``caplog``).
    """

    def __init__(self) -> None:
        super().__init__(logging.DEBUG)
        self.records: list[logging.LogRecord] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.records.append(record)

    @property
    def text(self) -> str:
        return "\n".join(r.getMessage() for r in self.records)


def runner_output(logs: LogLines) -> str:
    """What the runner's console script writes of its own log lines.

    It logs from level info on and formats a line as its message, followed by the traceback
    when the record carries an error (``claude_code_runner.__main__``).
    """
    console = logging.Formatter("%(message)s")
    return "\n".join(
        console.format(r)
        for r in logs.records
        if r.name == "claude_code_runner" and r.levelno >= logging.INFO
    )


def debug_tracebacks(logs: LogLines) -> list[str]:
    """The messages of the runner's lines of level debug that carry the traceback of an error."""
    return [
        r.getMessage()
        for r in logs.records
        if r.name == "claude_code_runner" and r.levelno == logging.DEBUG and r.exc_info
    ]


@pytest.fixture
def logs() -> Iterator[LogLines]:
    # Every logger at DEBUG except the frame dumps of the WebSocket library, which the runner
    # keeps at WARNING in production (__main__), because a frame can hold the API token.
    root = logging.getLogger()
    handler = LogLines()
    level = root.level
    root.addHandler(handler)
    root.setLevel(logging.DEBUG)
    websockets = logging.getLogger("websockets")
    ws_level = websockets.level
    websockets.setLevel(logging.WARNING)
    try:
        yield handler
    finally:
        root.removeHandler(handler)
        root.setLevel(level)
        websockets.setLevel(ws_level)
