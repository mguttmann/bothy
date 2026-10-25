# SPDX-License-Identifier: Apache-2.0
"""The command line entry (brief M2-50 Design note 2): exit codes and the clean stop."""

from __future__ import annotations

import asyncio
import os
from pathlib import Path
from typing import Any

import httpx
import pytest
from bothy_sdk.errors import problem_error

import jira_app.__main__ as entry
from jira_app.config import ConfigError
from jira_app.store.base import StoreLockedError
from tests.helpers import make_config


def test_missing_config_exits_1(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    missing = tmp_path / "absent.json"
    assert entry.main(["--config", str(missing)]) == 1
    assert str(missing) in capsys.readouterr().err


@pytest.mark.parametrize(
    "error",
    [
        ConfigError("jira_app.listen in x.json: expected host:port"),
        StoreLockedError("jira_app.state_dir (x): another process uses this directory"),
        problem_error(401, {"code": "unauthenticated"}),
        httpx.ConnectError("refused"),
        OSError("address in use"),
    ],
)
def test_start_failures_exit_1(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str], error: Exception
) -> None:
    async def fail(path: str, stop: asyncio.Event | None = None) -> None:
        raise error

    monkeypatch.setattr(entry, "run", fail)
    assert entry.main(["--config", "config.json"]) == 1
    assert capsys.readouterr().err.startswith("jira_app: ")


def test_umask_is_set_and_a_clean_run_exits_0(monkeypatch: pytest.MonkeyPatch) -> None:
    async def ok(path: str, stop: asyncio.Event | None = None) -> None:
        return None

    monkeypatch.setattr(entry, "run", ok)
    old = os.umask(0o022)
    try:
        assert entry.main(["--config", "config.json"]) == 0
        assert os.umask(0o022) == 0o077
    finally:
        os.umask(old)


@pytest.mark.anyio
async def test_run_stops_the_app_when_asked(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    config = make_config(tmp_path, jira_base_url="http://127.0.0.1:9")
    stopped: list[bool] = []

    class FakeApp:
        async def stop(self) -> None:
            stopped.append(True)

    async def fake_start(loaded: Any) -> FakeApp:
        assert loaded.listen_port == 0
        return FakeApp()

    monkeypatch.setattr(entry, "start_app", fake_start)
    stop = asyncio.Event()
    stop.set()
    await entry.run(config.source, stop)
    assert stopped == [True]
