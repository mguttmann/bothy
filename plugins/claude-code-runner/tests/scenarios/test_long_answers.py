# SPDX-License-Identifier: Apache-2.0
"""Long answers of the runner stay one message (brief M4-47; 01 6.4, 6.13; 09 18.10).

The real runner with Fake-Claude and the FakeServer. An answer above
``message_max_content_bytes`` goes out as one message: the head as its text and the rest as one
text attachment ``answer.txt`` uploaded like an ``ATTACH:`` file; together they are the answer.
An answer within the limit is one message without files. The FakeServer refuses a
``content_md`` above the limit, as the server does, so a cut answer would fail the run.
"""

from __future__ import annotations

from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from .conftest import World

pytestmark = pytest.mark.anyio

LIMIT = 5000


@pytest.fixture
def world_timings() -> dict[str, float]:
    # The server never ends a run on its own timers inside a scenario.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


async def _answer(world: World, text: str, limit: int = LIMIT) -> UUID:
    uploads = world.fake.uploads
    uploads.limits["message_max_content_bytes"] = limit
    uploads.part_size = 1024
    world.scenario(fixture="plain", result_override={"result": text})
    await world.start()
    run = await world.fake.trigger("Write the full report", channel_id=world.forum_a)
    return run.run_id


def _bodies(world: World, op: str) -> list[dict[str, Any]]:
    return [r.json for r in world.requests(op)]


async def test_a_long_answer_is_one_message_with_its_rest_attached(world: World) -> None:
    text = "\n".join(f"Line {i}: the check passed for module {i}." for i in range(200))
    assert len(text.encode()) > LIMIT
    run_id = await _answer(world, text)
    assert await world.ended(run_id) == "completed"
    (message,) = world.fake.messages
    assert len(message["content_md"].encode()) <= LIMIT
    (session,) = world.fake.uploads.sessions.values()
    assert session.name == "answer.txt"
    assert message["attachment_file_ids"] == [str(session.file_id)]
    assert message["content_md"] + session.content().decode() == text
    (finish,) = _bodies(world, "finishRun")
    assert finish["state"] == "completed"


async def test_an_answer_within_the_limit_is_one_message_without_files(world: World) -> None:
    text = "\n".join(f"Line {i}: fine." for i in range(300))
    assert 4096 < len(text.encode()) <= LIMIT
    run_id = await _answer(world, text)
    assert await world.ended(run_id) == "completed"
    (message,) = world.fake.messages
    assert message["content_md"] == text
    assert "attachment_file_ids" not in _bodies(world, "createMessage")[0]
    assert world.requests("createUpload") == []


async def test_a_short_answer_reads_no_limits(world: World) -> None:
    run_id = await _answer(world, "Done.")
    assert await world.ended(run_id) == "completed"
    (message,) = world.fake.messages
    assert message["content_md"] == "Done."
    assert world.requests("getInstance") == []


async def test_the_rest_comes_after_the_attach_files(world: World) -> None:
    notes = world.projects / "website" / "notes.md"
    notes.parent.mkdir(parents=True, exist_ok=True)
    notes.write_bytes(b"notes of the run")
    body = "\n".join(f"Line {i}: the check passed for module {i}." for i in range(200))
    run_id = await _answer(world, f"{body}\nATTACH: notes.md")
    assert await world.ended(run_id) == "completed"
    (message,) = world.fake.messages
    names = [s.name for s in world.fake.uploads.sessions.values()]
    assert names == ["notes.md", "answer.txt"]
    rest = list(world.fake.uploads.sessions.values())[1].content().decode()
    assert len(message["attachment_file_ids"]) == 2
    assert message["content_md"] + rest == body


async def test_without_its_rest_the_head_and_notice_fit_the_limit(world: World) -> None:
    """The rest cannot be attached: the shorter head and the notice still fit the limit."""
    world.fake.uploads.limits["file_max_upload_bytes"] = 10
    text = "\n".join(f"Line {i}: the check passed for module {i}." for i in range(200))
    run_id = await _answer(world, text)
    assert await world.ended(run_id) == "completed"
    (message,) = world.fake.messages
    assert len(message["content_md"].encode()) <= LIMIT
    assert "answer.txt" in message["content_md"]
    assert "attachment_file_ids" not in _bodies(world, "createMessage")[-1]
