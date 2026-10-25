# SPDX-License-Identifier: Apache-2.0
"""Mirroring and loop protection (brief M2-50 Design note 13; PLAN row: "loop protection")."""

from __future__ import annotations

from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest

from tests.fakes.chat import ALEX_ID, APP_ID, envelope, message_json, principal
from tests.fakes.jira_api import render_fixture
from tests.helpers import SUP_FORUM, World, start_world

pytestmark = pytest.mark.anyio


@pytest.fixture
async def world(tmp_path: Path) -> AsyncIterator[World]:
    w = await start_world(tmp_path)
    try:
        await w.deliver(render_fixture("jira:issue_created"))
        await w.settle()
        yield w
    finally:
        await w.close()


def thread_of(world: World) -> str:
    return next(iter(world.chat.threads))


def created(world: World, **members: Any) -> dict[str, Any]:
    author = members.pop("author", principal(ALEX_ID, "user", "Alex"))
    message = message_json(
        thread_id=members.pop("thread_id", thread_of(world)),
        channel_id=SUP_FORUM,
        content=members.pop("content", "Thanks, looking into it."),
        author=author,
        **members,
    )
    return envelope("message.created", {"message": message}, channel_id=SUP_FORUM)


async def send(world: World, event: dict[str, Any]) -> int:
    response = await world.server_request("/chat/events", event)
    await world.settle()
    return response.status_code


async def test_own_message_is_never_mirrored(world: World) -> None:
    event = created(world, author=principal(APP_ID, "app", "Jira"), origin="app")
    assert await send(world, event) == 204
    assert world.jira.comments() == []
    assert world.logs_with("own_message")


@pytest.mark.parametrize(
    ("members", "reason"),
    [
        ({"origin": "agent"}, "not_human"),
        ({"origin": "app"}, "not_human"),
        ({"origin": "import"}, "not_human"),
        ({"origin": "system"}, "not_human"),
        ({"kind": "card"}, "not_text"),
        ({"deleted_at": "2026-10-13T09:00:00Z"}, "deleted"),
        ({"thread_id": "0198a4f2-6c1e-7a3b-9d2e-0000000000dd"}, "thread_not_mapped"),
    ],
)
async def test_other_messages_are_not_mirrored(
    world: World, members: dict[str, Any], reason: str
) -> None:
    assert await send(world, created(world, **members)) == 204
    assert world.jira.comments() == []
    assert world.logs_with(reason)


async def test_human_reply_becomes_one_comment(world: World) -> None:
    text = "Thanks @alex, see [x](https://example.com) and {code} [~accountid:x]"
    assert await send(world, created(world, content=text)) == 202
    comments = world.jira.comments()
    assert len(comments) == 1
    assert comments[0].path == "/rest/api/3/issue/10012/comment"
    body = comments[0].body["body"]
    assert body["type"] == "doc"
    assert body["version"] == 1
    head, block = body["content"]
    assert head["content"][0] == {"type": "text", "text": "Alex", "marks": [{"type": "code"}]}
    assert head["content"][1] == {"type": "text", "text": " wrote in chat:"}
    assert block == {"type": "codeBlock", "content": [{"type": "text", "text": text}]}
    # The echoes of the comment (comment_created and an update without changelog) add nothing.
    thread = world.chat.threads[thread_of(world)]
    assert len(thread.messages) == 1
    assert world.logs_with("own_jira_object")
    assert world.logs_with("no_change")


async def test_truncated_message_is_loaded(world: World) -> None:
    event = created(world, content="short", content_truncated=True)
    message = event["payload"]["message"]
    world.chat.full_messages[message["id"]] = {
        **message,
        "content_md": "the full text",
        "content_truncated": False,
    }
    assert await send(world, event) == 202
    assert world.chat.calls_of("getMessage")
    block = world.jira.comments()[0].body["body"]["content"][1]
    assert block["content"][0]["text"] == "the full text"


async def test_jira_failure_is_logged_and_not_repeated(world: World) -> None:
    world.jira.fail("500")
    assert await send(world, created(world)) == 202
    assert len(world.jira.comments()) == 1
    failed = world.logs_with("mirror.failed")
    assert len(failed) == 1
    assert failed[0]["status"] == 500


async def test_other_event_types_are_ignored(world: World) -> None:
    event = envelope("thread.created", {"thread": {}}, channel_id=SUP_FORUM)
    assert await send(world, event) == 204
    assert world.jira.comments() == []
