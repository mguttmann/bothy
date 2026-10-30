# SPDX-License-Identifier: Apache-2.0
"""The runner's own code with Fake-Claude against the real server of this tree (sdkserver).

One server per session (``bothy_sdk.testing.sdkserver``): the server of this tree on a disposable
PostgreSQL with the world of 07 "Examples"; the runner is Lotse there, with its DPoP-bound API
token, a consumer of its own per test, tool class ``chat`` (Lotse's configured class) and the
folder ``website`` as the default workspace of every channel without a binding (RUNNER-NO-GIT).
Without the Go tool chain or a Docker-compatible runtime the tests fail, they never skip.
"""

from __future__ import annotations

import asyncio
import contextlib
import dataclasses
import itertools
import json
import os
import shutil
import sys
import tempfile
from collections.abc import AsyncIterator, Iterator
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.sources import SourceReady
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.doubles import RecordingSource
from bothy_sdk.testing.sdkserver import RealServer, build_sdkserver, start_sdkserver

from claude_code_runner.app import Runner
from claude_code_runner.config import RunnerConfig, load_config

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import helper

API_KEY = "sk-ant-canary-0123456789"
_CONSUMERS = itertools.count(1)


@pytest.fixture(scope="session")
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture(scope="session")
def sdkserver_binary() -> Iterator[Path]:
    work = Path(tempfile.mkdtemp(prefix="bothy-runner-sdkserver-"))
    try:
        yield build_sdkserver(work)
    finally:
        shutil.rmtree(work, ignore_errors=True)


@pytest.fixture(scope="session")
async def real(sdkserver_binary: Path) -> AsyncIterator[RealServer]:
    workdir = Path(tempfile.mkdtemp(prefix="bothy-runner-sdkserver-run-"))
    os.chmod(workdir, 0o700)  # noqa: PTH101 - the token files live here
    log = os.environ.get("BOTHY_E2E_SERVER_LOG")
    # No virus scanner runs here: uploads become final with scan_status skipped (12 3.4).
    server = await start_sdkserver(
        sdkserver_binary,
        workdir,
        config={"file.scan.mode": "off"},
        log=Path(log) if log else None,
    )
    try:
        yield server
    finally:
        await server.stop()
        shutil.rmtree(workdir, ignore_errors=True)


@dataclasses.dataclass
class RealWorld:
    """One runner world against the session's real server."""

    real: RealServer
    root: Path
    projects: Path
    home: Path
    bin_dir: Path
    capture: Path
    claude: Path
    runners: list[tuple[Runner, asyncio.Task[Any]]] = dataclasses.field(default_factory=list)

    def environ(self) -> dict[str, str]:
        return {
            "HOME": str(self.home),
            "USER": "alex",
            "LOGNAME": "alex",
            "SHELL": "/bin/sh",
            "PATH": f"{self.bin_dir}:/usr/bin:/bin",
            "LANG": "C.UTF-8",
            "TERM": "dumb",
            "TMPDIR": str(self.root),
        }

    def config(self, **runner: Any) -> RunnerConfig:
        token = self.root / "agent_token"
        if not token.exists():
            token.write_text(self.real.credentials.api_token.reveal())
            token.chmod(0o600)
        key = self.root / "api_key"
        if not key.exists():
            key.write_text(API_KEY + "\n")
            key.chmod(0o600)
        values: dict[str, Any] = {
            "server_url": self.real.origin,
            "instance_id": str(self.real.instance_id),
            "agent_token_ref": f"file:{token}",
            "dpop_key_ref": f"file:{self.real.dpop_key_file}",
            "anthropic_api_key_ref": f"file:{key}",
            "consumer": f"runner{next(_CONSUMERS)}",
            "default_model": "model-large",
            "projects_root": str(self.projects),
            "default_workspace": "website",
            "state_dir": str(self.root / "state"),
            "tool_class": "chat",
            "poll_interval": "20ms",
            "stop_grace": "1s",
            "kill_grace": "1s",
            "session_lock_wait": f"{int(PATIENCE)}s",
            "cli_probe_timeout": f"{int(PATIENCE)}s",
        }
        values.update(runner)
        lines = ["[runner]", *(f"{k} = {json.dumps(v)}" for k, v in values.items())]
        path = self.root / "runner.toml"
        path.write_text("\n".join(lines) + "\n")
        return dataclasses.replace(load_config(path), sdk=self.real.sdk_settings)

    def scenario(self, **scenario: Any) -> None:
        helper.update(self.claude, scenario)

    def calls(self, kind: str) -> list[dict[str, Any]]:
        return [c for c in helper.read_capture(self.capture) if c["kind"] == kind]

    def turns(self) -> list[dict[str, Any]]:
        return helper.turns(self.capture)

    def gate(self, name: str) -> Path:
        return self.root / f"{name}.open"

    async def start(self, config: RunnerConfig | None = None) -> Runner:
        """Start the runner and wait until its event source is ready (no event is missed)."""
        runner = Runner(config or self.config(), self.environ(), clock=self.real.clock)
        adapter = await runner.build()
        source = RecordingSource(await adapter.make_source())
        adapter.source = source
        task = asyncio.ensure_future(runner.run())
        self.runners.append((runner, task))
        loop = asyncio.get_running_loop()
        deadline = loop.time() + PATIENCE
        while not any(isinstance(d, SourceReady) for d in source.deliveries):
            if task.done():
                task.result()
            if loop.time() > deadline:
                raise AssertionError("the runner's source did not become ready")
            await asyncio.sleep(0.05)
        return runner


@pytest.fixture
async def world(real: RealServer, tmp_path: Path) -> AsyncIterator[RealWorld]:
    root = tmp_path / "w"
    root.mkdir(mode=0o700)
    projects = root / "projects"
    (projects / "website" / ".git").mkdir(parents=True)
    home = root / "home"
    (home / ".claude").mkdir(parents=True)
    bin_dir = root / "bin"
    capture = root / "capture.jsonl"
    claude = helper.install(bin_dir, {"fixture": "happy"}, capture)
    w = RealWorld(real, root, projects, home, bin_dir, capture, claude)
    try:
        yield w
    finally:
        for runner, task in w.runners:
            await runner.stop()
            with contextlib.suppress(Exception):
                await asyncio.wait_for(task, PATIENCE)
