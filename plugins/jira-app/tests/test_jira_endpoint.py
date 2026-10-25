# SPDX-License-Identifier: Apache-2.0
"""``POST /jira/webhook`` and the HTTP surface (brief M2-50 Design notes 4 to 6)."""

from __future__ import annotations

import asyncio
import json
from collections.abc import AsyncIterator
from pathlib import Path

import httpx
import pytest

from tests.fakes.jira_api import render_fixture
from tests.helpers import World, eventually, start_world

pytestmark = pytest.mark.anyio


@pytest.fixture
async def world(tmp_path: Path) -> AsyncIterator[World]:
    w = await start_world(tmp_path, members={"max_body_bytes": 4096, "max_queued_jobs": 3})
    try:
        yield w
    finally:
        await w.close()


async def test_valid_issue_created_is_accepted_with_one_job(world: World) -> None:
    assert await world.deliver(render_fixture("jira:issue_created")) == 202
    accepted = world.logs_with("delivery.accepted")
    assert len(accepted) == 1
    assert accepted[0]["job_kind"] == "jira_event"
    await world.settle()
    assert len(world.chat.calls_of("createThread")) == 1


@pytest.mark.parametrize(
    "options",
    [
        {"alter": True},
        {"omit": True},
        {"truncate": True},
        {"method": "sha1"},
        {"method": "sha512"},
        {"secret": "another secret"},
        {"second_header": True},
    ],
)
async def test_bad_signatures_are_refused_without_a_job(
    world: World, options: dict[str, object]
) -> None:
    assert await world.deliver(render_fixture("jira:issue_created"), **options) == 401
    await world.settle()
    assert not world.logs_with("delivery.accepted")
    assert not world.chat.calls_of("createThread")
    assert not world.chat.calls_of("createMessage")


async def test_unsupported_event_is_answered_204(world: World) -> None:
    payload = render_fixture("jira:issue_created")
    payload["webhookEvent"] = "jira:issue_deleted"
    assert await world.deliver(payload) == 204
    assert world.logs_with("unsupported")


async def test_identical_body_again_is_a_duplicate(world: World) -> None:
    body = json.dumps(render_fixture("jira:issue_created")).encode()
    assert await world.deliver(body) == 202
    assert await world.deliver(body) == 204
    await world.settle()
    assert len(world.logs_with("delivery.accepted")) == 1
    assert len(world.chat.calls_of("createThread")) == 1
    assert world.logs_with("duplicate")


async def test_unknown_project_is_answered_204(world: World) -> None:
    assert await world.deliver(render_fixture("jira:issue_created", key="OPS-1")) == 204
    assert world.logs_with("project_not_configured")


async def test_payload_without_issue_id_is_400(world: World) -> None:
    payload = render_fixture("jira:issue_created")
    del payload["issue"]["id"]
    assert await world.deliver(payload) == 400
    assert world.logs_with("invalid_payload")


@pytest.mark.parametrize(
    "body",
    [b"{not json", b'{"a": 1, "a": 2}', b"\xff\xfe", b"[NaN]"],
)
async def test_invalid_json_is_400(world: World, body: bytes) -> None:
    assert await world.deliver(body) == 400


async def test_text_plain_is_415(world: World) -> None:
    assert (
        await world.deliver(render_fixture("jira:issue_created"), content_type="text/plain") == 415
    )


async def test_json_with_parameters_is_accepted(world: World) -> None:
    status = await world.deliver(
        render_fixture("jira:issue_created"), content_type="application/json; charset=utf-8"
    )
    assert status == 202


async def test_body_above_max_body_bytes_is_413(world: World) -> None:
    payload = render_fixture("jira:issue_created", text="x" * 5000)
    assert await world.deliver(payload) == 413


async def test_get_is_405_and_unknown_path_404(world: World) -> None:
    async with httpx.AsyncClient() as http:
        get = await http.get(f"{world.app.url}/jira/webhook")
        assert get.status_code == 405
        assert get.content == b""
        unknown = await http.post(f"{world.app.url}/other", content=b"{}")
        assert unknown.status_code == 404
        assert unknown.content == b""


async def test_full_queue_answers_503(world: World) -> None:
    # The worker holds the first job in its step, so the jobs stay queued.
    world.chat.gate = asyncio.Event()
    statuses = [
        await world.deliver(
            render_fixture("jira:issue_created", issue_id=str(20000 + n), key=f"SUP-{100 + n}")
        )
        for n in range(4)
    ]
    assert statuses == [202, 202, 202, 503]
    assert world.logs_with("queue_full")
    world.chat.gate.set()
    await world.settle()
    assert len(world.chat.threads) == 3


async def test_webhook_secret_with_mode_0644_answers_503(world: World) -> None:
    (world.directory / "webhook-secret").chmod(0o644)
    assert await world.deliver(render_fixture("jira:issue_created")) == 503
    assert world.logs_with("secret_file")
    (world.directory / "webhook-secret").chmod(0o600)
    assert await world.deliver(render_fixture("jira:issue_created")) == 202


async def test_stopping_app_answers_503(world: World) -> None:
    world.chat.gate = asyncio.Event()
    assert await world.deliver(render_fixture("jira:issue_created")) == 202
    await eventually(lambda: bool(world.chat.calls_of("createThread")), "the running step")
    stopping = asyncio.ensure_future(world.stop_app())

    async def refused() -> bool:
        return await world.deliver(render_fixture("comment_created")) == 503

    await eventually(refused, "the 503 of the stopping app")
    assert not stopping.done()
    world.chat.gate.set()
    await stopping
    # The running step finished before the stop.
    assert len(world.chat.threads) == 1
