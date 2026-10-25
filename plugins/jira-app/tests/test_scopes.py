# SPDX-License-Identifier: Apache-2.0
"""Scopes of the manifest and a missing scope at call time (11 10.1 row "Scopes" through 10.2;
07 2.10).
"""

from __future__ import annotations

import json
import re
from pathlib import Path

import pytest

from tests.fakes.chat import problem
from tests.fakes.jira_api import render_fixture
from tests.helpers import eventually, start_world

pytestmark = pytest.mark.anyio

APP = Path(__file__).resolve().parents[1]
SPEC = APP.parents[1] / "docs" / "spec" / "11-extensions.md"


def scopes_row() -> tuple[list[str], list[str]]:
    """The required and optional scopes of the row "Scopes" of 10.1."""
    text = SPEC.read_text(encoding="utf-8")
    section = text.split("### 10.1", 1)[1].split("### 10.2", 1)[0]
    row = next(line for line in section.splitlines() if line.startswith("| Scopes |"))
    required, _, optional = row.split("|")[2].partition("optionally")
    pattern = re.compile(r"`([a-z]+:[a-z]+)`")
    # dict.fromkeys keeps the first mention of a scope the row repeats in its reason.
    return pattern.findall(required), list(dict.fromkeys(pattern.findall(optional)))


def test_manifest_requests_exactly_the_scopes_of_the_chapter() -> None:
    manifest = json.loads((APP / "manifest.json").read_text(encoding="utf-8"))
    required, optional = scopes_row()
    assert required == ["messages:write", "commands:write", "components:write"]
    assert manifest["scopes"]["required"] == required
    assert manifest["scopes"]["optional"] == optional
    assert manifest["events"] == ["message.created"]


async def test_scope_missing_ends_the_job_after_one_attempt(tmp_path: Path) -> None:
    world = await start_world(tmp_path)
    try:
        world.chat.fail("createThread", problem(403, "scope_missing"))
        assert await world.deliver(render_fixture("jira:issue_created")) == 202
        await world.settle()
        assert len(world.chat.calls_of("createThread")) == 1
        lines = world.logs_with("scope_missing")
        assert len(lines) == 1
        assert lines[0]["level"] == "warn"
        assert lines[0]["operation"] == "createThread"
        assert lines[0]["status"] == 403
        assert world.chat.threads == {}
        # The job is gone: the next delivery is handled at once.
        await world.deliver(render_fixture("jira:issue_created", issue_id="10013", key="SUP-13"))
        await world.settle()
        assert len(world.chat.threads) == 1
    finally:
        await world.close()


@pytest.mark.parametrize(
    ("status", "code", "reason"),
    [
        (403, "forbidden", "forbidden"),
        (403, "channel_read_only", "channel_read_only"),
        (422, "idempotency_key_reuse", "idempotency_key_reuse"),
        (400, "validation_failed", "validation_failed"),
    ],
)
async def test_permanent_answers_end_the_job(
    tmp_path: Path, status: int, code: str, reason: str
) -> None:
    world = await start_world(tmp_path)
    try:
        world.chat.fail("createThread", problem(status, code, reason=reason))
        assert await world.deliver(render_fixture("jira:issue_created")) == 202
        await world.settle()
        assert len(world.chat.calls_of("createThread")) == 1
        assert world.logs_with(reason) or world.logs_with("job.refused")
    finally:
        await world.close()


async def test_unauthenticated_answers_503_until_get_me_succeeds(tmp_path: Path) -> None:
    world = await start_world(tmp_path)
    try:
        world.chat.fail("createThread", problem(401, "unauthenticated"))
        world.chat.fail("getMe", problem(401, "unauthenticated"))
        assert await world.deliver(render_fixture("jira:issue_created")) == 202
        await world.settle()
        assert world.logs_with("unauthenticated")

        async def accepted_again() -> bool:
            payload = render_fixture("comment_created", comment_id=str(len(world.handler.lines)))
            return await world.deliver(payload) in (202, 204)

        await eventually(accepted_again, "the recheck of getMe")
        assert world.logs_with("app.authenticated")
    finally:
        await world.close()


async def test_rate_limited_is_retried(tmp_path: Path) -> None:
    world = await start_world(tmp_path)
    try:
        world.chat.fail("createThread", problem(429, "rate_limited"), problem(503, "unavailable"))
        assert await world.deliver(render_fixture("jira:issue_created")) == 202
        await world.settle()
        assert len(world.chat.calls_of("createThread")) == 3
        assert len(world.chat.threads) == 1
    finally:
        await world.close()
