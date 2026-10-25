# SPDX-License-Identifier: Apache-2.0
"""One thread per Jira issue (brief M2-50 Design note 10; PLAN row: "``jira:issue_created``,
``jira:issue_updated`` and ``comment_created`` from fixtures yield a thread and messages as in
M2-49").
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from pathlib import Path

import pytest

from tests.fakes.chat import problem, transport_error
from tests.fakes.jira_api import render_fixture
from tests.helpers import SUP_FORUM, World, start_world

pytestmark = pytest.mark.anyio


@pytest.fixture
async def world(tmp_path: Path) -> AsyncIterator[World]:
    w = await start_world(tmp_path)
    try:
        yield w
    finally:
        await w.close()


async def test_issue_created_opens_one_forum_post(world: World) -> None:
    assert await world.deliver(render_fixture("jira:issue_created")) == 202
    await world.settle()
    assert len(world.chat.threads) == 1
    thread = next(iter(world.chat.threads.values()))
    assert thread.channel_id == SUP_FORUM
    assert thread.title == "SUP-12 Login fails"
    first = thread.messages[0]["content_md"]
    assert first.startswith("**Issue [SUP-12](")
    assert "/browse/SUP-12) created** by `Sam`" in first
    assert "Users cannot log in since the update." in first


async def test_comment_and_update_land_in_the_same_thread(world: World) -> None:
    await world.deliver(render_fixture("jira:issue_created"))
    await world.settle()
    assert await world.deliver(render_fixture("comment_created")) == 202
    status = render_fixture(
        "jira:issue_updated",
        changelog=[{"field": "status", "fromString": "To Do", "toString": "In Progress"}],
    )
    assert await world.deliver(status) == 202
    await world.settle()
    assert len(world.chat.threads) == 1
    thread = next(iter(world.chat.threads.values()))
    texts = [m["content_md"] for m in thread.messages]
    assert len(texts) == 3
    assert texts[1].startswith("`Robin` commented on [SUP-12](")
    assert texts[2].endswith("updated** by `Sam`:\n`status`: `To Do` to `In Progress`")


async def test_update_without_changelog_posts_nothing(world: World) -> None:
    await world.deliver(render_fixture("jira:issue_created"))
    await world.settle()
    assert await world.deliver(render_fixture("jira:issue_updated", changelog=None)) == 204
    assert await world.deliver(render_fixture("jira:issue_updated", changelog=[])) == 204
    await world.settle()
    thread = next(iter(world.chat.threads.values()))
    assert len(thread.messages) == 1
    assert world.logs_with("no_change")


async def test_second_issue_creates_a_second_post(world: World) -> None:
    await world.deliver(render_fixture("jira:issue_created"))
    await world.deliver(
        render_fixture("jira:issue_created", issue_id="10013", key="SUP-13", summary="Crash")
    )
    await world.settle()
    titles = sorted(t.title for t in world.chat.threads.values())
    assert titles == ["SUP-12 Login fails", "SUP-13 Crash"]


async def test_mapping_survives_a_restart(world: World) -> None:
    await world.deliver(render_fixture("jira:issue_created"))
    await world.settle()
    await world.restart()
    await world.deliver(render_fixture("comment_created"))
    await world.settle()
    assert len(world.chat.threads) == 1
    thread = next(iter(world.chat.threads.values()))
    assert len(thread.messages) == 2


async def test_moved_issue_keeps_its_thread(world: World) -> None:
    await world.deliver(render_fixture("jira:issue_created"))
    await world.settle()
    moved = render_fixture(
        "jira:issue_updated",
        key="WEB-3",
        project="WEB",
        changelog=[{"field": "Key", "fromString": "SUP-12", "toString": "WEB-3"}],
    )
    assert await world.deliver(moved) == 202
    await world.settle()
    assert len(world.chat.threads) == 1
    thread = next(iter(world.chat.threads.values()))
    assert "[WEB-3](" in thread.messages[1]["content_md"]


async def test_unreachable_thread_is_replaced_once(world: World) -> None:
    await world.deliver(render_fixture("jira:issue_created"))
    await world.settle()
    world.chat.fail("createMessage", problem(404, "not_found"))
    await world.deliver(render_fixture("comment_created"))
    await world.settle()
    assert len(world.chat.threads) == 2
    assert len(world.logs_with("thread_unreachable")) == 1
    # The new post is the thread of the issue from now on.
    await world.deliver(render_fixture("comment_created", comment_id="10051", text="Again."))
    await world.settle()
    newest = list(world.chat.threads.values())[1]
    assert len(newest.messages) == 2


async def test_second_not_found_ends_the_job(world: World) -> None:
    await world.deliver(render_fixture("jira:issue_created"))
    await world.settle()
    world.chat.fail("createMessage", problem(404, "not_found"))
    world.chat.fail("createThread", problem(404, "not_found"))
    await world.deliver(render_fixture("comment_created"))
    await world.settle()
    assert len(world.chat.threads) == 1
    assert len(world.chat.calls_of("createThread")) == 2


async def test_rerun_after_crash_repeats_the_same_client_msg_id(world: World) -> None:
    # The thread is created, but the answer is lost before the map write (a crash there).
    world.chat.fail_after("createThread", transport_error())
    await world.deliver(render_fixture("jira:issue_created"))
    await world.settle()
    calls = world.chat.calls_of("createThread")
    assert len(calls) >= 2
    ids = {c.args["first_message"]["client_msg_id"] for c in calls}
    keys = {c.args["idempotency_key"] for c in calls}
    assert len(ids) == 1
    assert len(keys) == 1
    assert len(world.chat.threads) == 1
