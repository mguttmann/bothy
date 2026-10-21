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
import json
import logging
import sys
from collections.abc import AsyncIterator, Callable
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.sources import EventSource
from bothy_sdk.testing.fakeserver import FakeServer

from claude_code_runner.app import Runner
from claude_code_runner.config import RunnerConfig, load_config

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import helper

API_KEY = "sk-ant-canary-0123456789"


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
            "session_lock_wait": "3s",
            "cli_probe_timeout": "30s",
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

    async def stop(self, runner: Runner) -> None:
        for item in list(self.runners):
            if item[0] is runner:
                self.runners.remove(item)
                await runner.stop()
                with contextlib.suppress(Exception):
                    await asyncio.wait_for(item[1], 20)

    async def until(self, predicate: Callable[[], Any], timeout: float = 90.0) -> None:
        loop = asyncio.get_running_loop()
        deadline = loop.time() + timeout
        while loop.time() < deadline:
            if predicate():
                return
            await asyncio.sleep(0.02)
        raise AssertionError("condition not reached")

    def state_of(self, run_id: UUID) -> str:
        return self.fake.run_state(run_id).state

    async def ended(self, run_id: UUID, timeout: float = 90.0) -> str:
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
async def world(tmp_path: Path) -> AsyncIterator[World]:
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
    async with FakeServer(dpop=False, costs=True) as fake:
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
        pump = asyncio.ensure_future(_pump(fake))
        try:
            yield w
        finally:
            for runner, task in list(w.runners):
                await runner.stop()
                with contextlib.suppress(Exception):
                    await asyncio.wait_for(task, 20)
            pump.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await pump


async def _pump(fake: FakeServer) -> None:
    """Move the shared clock, so throttles, heartbeats and renewals of the SDK run."""
    while True:
        await fake.clock.advance(0.05)
        await asyncio.sleep(0.01)


@pytest.fixture
def logs(caplog: pytest.LogCaptureFixture) -> pytest.LogCaptureFixture:
    # Every logger at DEBUG except the frame dumps of the WebSocket library, which the runner
    # keeps at WARNING in production (__main__), because a frame can hold the API token.
    caplog.set_level(logging.DEBUG)
    caplog.set_level(logging.WARNING, logger="websockets")
    return caplog
