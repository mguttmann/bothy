# SPDX-License-Identifier: Apache-2.0
"""Long answers stay one message (brief M4-47; 01 6.4, 6.13; 09 4.7).

`split_for_limit` cuts a text so its head fits ``message_max_content_bytes`` without splitting a
character or a fenced code block; `send_long_message` sends a text within the limit as it is and
a longer one as one message whose rest is one text attachment, uploaded through the upload
session of 12 3.2; when that attachment cannot be made, the head goes out with a short note,
still as one message. The FakeServer refuses a ``content_md`` above the limit with ``400
validation_failed`` ``too_long``, as the server does.
"""

from __future__ import annotations

import random
from collections.abc import AsyncIterator
from uuid import UUID

import httpx
import pytest

from bothy_sdk.client import Client
from bothy_sdk.errors import ValidationFailed
from bothy_sdk.long_answers import (
    OVERFLOW_NAME,
    send_long_message,
    split_for_limit,
)
from bothy_sdk.outbox import Outbox, OutboxState
from bothy_sdk.testing.fakeserver import FakeServer

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    """The FakeServer runs on asyncio."""
    return "asyncio"


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer(dpop=False) as server:
        server.uploads.part_size = 64
        server.uploads.limits["message_max_content_bytes"] = 200
        yield server


@pytest.fixture
async def client(fake: FakeServer) -> AsyncIterator[Client]:
    async with Client(
        fake.base_url, fake.credentials, settings=fake.sdk_settings, clock=fake.clock
    ) as c:
        yield c


async def _thread(fake: FakeServer) -> tuple[UUID, UUID]:
    forum = fake.add_forum("Projekt-A", "website")
    triggered = await fake.trigger("Hello @Lotse", channel_id=forum)
    return triggered.thread_id, forum


def _bytes(text: str) -> int:
    return len(text.encode("utf-8"))


# ---- split_for_limit -------------------------------------------------------------------------


def test_a_text_within_the_limit_is_not_cut() -> None:
    assert split_for_limit("short", 10) == ("short", None)


def test_the_head_fits_and_ends_at_a_line_break() -> None:
    text = "\n".join(f"line {i}" for i in range(50))
    head, rest = split_for_limit(text, 100)
    assert rest is not None
    assert _bytes(head) <= 100
    assert head.endswith("\n")
    assert head + rest == text


def test_a_cut_never_splits_a_character() -> None:
    head, rest = split_for_limit("é" * 100, 51)
    assert _bytes(head) == 50
    assert head + (rest or "") == "é" * 100


def test_a_fenced_block_moves_whole_into_the_rest() -> None:
    code = "\n".join(f"x = {i}" for i in range(40))
    text = f"Intro line\n```python\n{code}\n```\nAfter"
    head, rest = split_for_limit(text, 120)
    assert head == "Intro line\n"
    assert rest is not None
    assert rest.startswith("```python\n")


def test_a_block_that_starts_the_text_stays_cut() -> None:
    code = "\n".join(f"x = {i}" for i in range(40))
    head, rest = split_for_limit(f"```\n{code}\n```", 60)
    assert head
    assert _bytes(head) <= 60
    assert rest is not None


def test_property_every_cut_fits_and_loses_nothing() -> None:
    rng = random.Random(4747)  # noqa: S311 - test data, not a secret
    parts = ["a", "b", " ", "\n", "é", "\U0001f600", "\n```\n"]
    for _ in range(300):
        text = "".join(rng.choice(parts) for _ in range(rng.randint(0, 300)))
        limit = rng.randint(1, 200)
        head, rest = split_for_limit(text, limit)
        assert _bytes(head) <= limit
        assert head + (rest or "") == text
        assert (rest is None) == (_bytes(text) <= limit)


# ---- the FakeServer limit --------------------------------------------------------------------


async def test_the_fake_server_refuses_content_above_the_limit(
    fake: FakeServer, client: Client
) -> None:
    thread, _ = await _thread(fake)
    entry = await Outbox(client).send_message(thread, "x" * 201)
    assert entry.state is OutboxState.FAILED
    assert isinstance(entry.problem, ValidationFailed)
    assert entry.problem.errors[0].pointer == "/content_md"
    assert entry.problem.errors[0].code == "too_long"


# ---- send_long_message -----------------------------------------------------------------------


async def test_an_answer_within_the_limit_is_one_message_without_files(
    fake: FakeServer, client: Client
) -> None:
    thread, channel = await _thread(fake)
    before = len(fake.messages)
    entry = await send_long_message(
        client, Outbox(client), thread, channel, "Done.", max_content_bytes=200
    )
    assert entry.state is OutboxState.SENT
    assert len(fake.messages) == before + 1
    assert fake.messages[-1]["content_md"] == "Done."
    assert "attachment_file_ids" not in fake.messages[-1]
    assert fake.uploads.sessions == {}


async def test_a_long_answer_is_one_message_with_its_rest_attached(
    fake: FakeServer, client: Client
) -> None:
    thread, channel = await _thread(fake)
    text = "\n".join(f"Step {i}: done" for i in range(80))
    before = len(fake.messages)
    entry = await send_long_message(client, Outbox(client), thread, channel, text)
    assert entry.state is OutboxState.SENT
    assert len(fake.messages) == before + 1, "never split into several messages"
    body = fake.messages[-1]
    assert _bytes(body["content_md"]) <= 200
    (upload,) = fake.uploads.sessions.values()
    assert upload.name == OVERFLOW_NAME
    assert body["attachment_file_ids"] == [str(upload.file_id)]
    assert body["content_md"] + upload.content().decode("utf-8") == text
    assert entry.created is not None
    assert [a.file.id for a in entry.created.attachments] == [upload.file_id]


async def test_the_limit_given_by_the_caller_is_used_without_get_instance(
    fake: FakeServer, client: Client
) -> None:
    thread, channel = await _thread(fake)
    text = "word " * 30  # 150 bytes: within the server's 200, above the caller's 100
    await send_long_message(client, Outbox(client), thread, channel, text, max_content_bytes=100)
    body = fake.messages[-1]
    assert _bytes(body["content_md"]) <= 100
    assert len(body["attachment_file_ids"]) == 1


async def test_without_an_attachment_the_head_goes_out_with_a_note(
    fake: FakeServer, client: Client
) -> None:
    thread, channel = await _thread(fake)
    fake.uploads.limits["file_max_upload_bytes"] = 10
    text = "\n".join(f"Step {i}: done" for i in range(80))
    before = len(fake.messages)
    entry = await send_long_message(client, Outbox(client), thread, channel, text)
    assert entry.state is OutboxState.SENT
    assert len(fake.messages) == before + 1
    body = fake.messages[-1]
    assert _bytes(body["content_md"]) <= 200
    assert "could not be attached" in body["content_md"]
    assert "attachment_file_ids" not in body


async def test_a_failed_upload_is_aborted_before_the_note_goes_out(
    fake: FakeServer, client: Client, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A session that will not complete is aborted, so it holds no open upload slot."""
    thread, channel = await _thread(fake)

    async def broken_part(*args: object, **kwargs: object) -> None:
        raise httpx.ConnectError("connection lost")

    monkeypatch.setattr("bothy_sdk.long_answers.upload_part", broken_part)
    text = "\n".join(f"Step {i}: done" for i in range(80))
    entry = await send_long_message(client, Outbox(client), thread, channel, text)
    assert entry.state is OutboxState.SENT
    (upload,) = fake.uploads.sessions.values()
    assert upload.state == "aborted"
    assert "could not be attached" in fake.messages[-1]["content_md"]
