# SPDX-License-Identifier: Apache-2.0
"""Readiness in the heartbeat, secrets nowhere, private state and retention (BR-13, 09 4.8)."""

from __future__ import annotations

import os
import stat
import time
from pathlib import Path

import pytest

from .conftest import API_KEY, LogLines, World

pytestmark = pytest.mark.anyio


async def test_project_readiness_in_heartbeat(world: World) -> None:
    fake = world.fake
    await world.start()
    beat = fake.heartbeats[-1]
    readiness = {r["channel_id"]: r["ready"] for r in beat["project_readiness"]}
    assert readiness == {str(world.forum_a): True, str(world.forum_b): False}
    assert beat["state"] == "online"
    assert beat["reported_tool_class"] == "scoped"
    assert beat["runtime_version"] == "2.1.259"
    assert beat["billing_modes_supported"] == ["organization"]
    assert "reported_billing_mode" not in beat
    assert "billing_evidence" not in beat
    details = beat["details"]
    assert details["capabilities"] == {
        "followups": "steer",
        "title": True,
        "components": False,
        "voice_input": False,
        "files_out": True,
    }
    assert details["adapter"] == {"name": "claude-code-runner", "version": "0.0.0"}
    assert details["runtime"] == {"name": "claude-code"}
    assert details["protocol_version"] == 1
    count = len(fake.heartbeats)
    (world.projects / "docs" / ".git").mkdir(parents=True)
    fake.update_forum(world.forum_b, prompt="Docs only.")
    await world.until(lambda: len(fake.heartbeats) > count)
    readiness = {r["channel_id"]: r["ready"] for r in fake.heartbeats[-1]["project_readiness"]}
    assert readiness[str(world.forum_b)] is True


async def test_degraded_runtime_refuses_runs(world: World) -> None:
    world.scenario(version="2.0.1")
    await world.start()
    beat = world.fake.heartbeats[-1]
    assert beat["state"] == "degraded"
    assert beat["details"]["degraded_reasons"] == ["runtime_version_unsupported"]
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert world.calls("auth_status") == []


async def test_untested_version_only_warns(world: World) -> None:
    world.scenario(version="2.1.300", fixture="plain")
    await world.start()
    assert world.fake.heartbeats[-1]["details"]["degraded_reasons"] == ["runtime_version_untested"]
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"


def _files(root: Path) -> list[Path]:
    return [p for p in root.rglob("*") if p.is_file()]


async def test_br_13_no_secret_values_leak(world: World, logs: LogLines) -> None:
    fake = world.fake
    token = fake.credentials.api_token.reveal()
    await world.start()
    run = await fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    for request in fake.requests:
        text = repr(request.json) + repr(
            {k: v for k, v in request.headers.items() if k != "authorization"}
        )
        assert API_KEY not in text
        assert token not in text
    assert logs.records  # the check below reads real lines
    for record in logs.records:
        message = record.getMessage()
        assert API_KEY not in message
        assert token not in message
    for path in _files(world.state):
        try:
            data = path.read_bytes()
        except FileNotFoundError:  # a SQLite journal that was just removed
            continue
        assert API_KEY.encode() not in data, path
        assert token.encode() not in data, path
    for turn in world.turns():
        blob = repr(turn["argv"]) + repr(turn["stdin"]) + (turn["system_prompt"] or "")
        assert API_KEY not in blob
        assert token not in blob
        assert turn["env"]["ANTHROPIC_API_KEY"] == API_KEY
        assert token not in repr(turn["env"])
    for call in world.calls("auth_status"):
        assert call["env"]["ANTHROPIC_API_KEY"] == API_KEY
    for call in world.calls("version"):
        assert "ANTHROPIC_API_KEY" not in repr(call)


async def test_state_files_private_and_retention(world: World) -> None:
    fake = world.fake
    attachment = fake.add_file(b"x", name="a.txt", mime="text/plain")
    await world.start(world.config(stream_log_retention="1h"))
    first = await fake.trigger("One", channel_id=world.forum_a, attachments=[attachment])
    assert await world.ended(first.run_id) == "completed"
    for path in world.state.rglob("*"):
        mode = stat.S_IMODE(path.lstat().st_mode)
        assert mode == (0o700 if path.is_dir() else 0o600), path
    old = [p for p in _files(world.state / "sessions") if p.name != "usage.json"]
    old += _files(world.state / "files")
    past = time.time() - 7200
    for path in old:
        os.utime(path, (past, past))
    world.scenario(fixture="plain")
    second = await fake.trigger("Two", channel_id=world.forum_a)
    assert await world.ended(second.run_id) == "completed"
    await world.until(lambda: not any(p.exists() for p in old))
    assert any(str(second.run_id) in p.name for p in _files(world.state / "sessions"))
