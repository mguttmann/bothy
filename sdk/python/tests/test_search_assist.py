# SPDX-License-Identifier: Apache-2.0
"""The search assistant hook of the adapter (docs/spec/14-search.md 4.10, 09 10.6).

`search_assist.request_from` turns the event into an `AssistQuestion`; `guarded_search_assist`
runs the handler until ``expires_at``; the adapter calls ``on_search_assist`` for
``search.assist_requested`` and sends its answer with ``answerSearchAssist``.
"""

from __future__ import annotations

import asyncio
import contextlib
from collections.abc import AsyncIterator, Callable
from datetime import timedelta
from typing import Any
from uuid import UUID

import pytest

from bothy_sdk.adapter import Adapter, AdapterConfig
from bothy_sdk.search_assist import (
    AssistCandidate,
    AssistChoice,
    AssistQuestion,
    AssistReply,
    guarded_search_assist,
)
from bothy_sdk.sources import EventSource, SourceReady
from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.doubles import FakeClock, RecordingHandler, RecordingSource
from bothy_sdk.testing.fakeserver import FakeServer

pytestmark = pytest.mark.anyio


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


async def until(fake: FakeServer, predicate: Callable[[], Any]) -> None:
    await eventually(predicate, "the condition", step=lambda: fake.pass_time(0.05))


class AssistHandler(RecordingHandler):
    """Answers every question with the first candidate."""

    def __init__(self) -> None:
        super().__init__()
        self.asked: list[AssistQuestion] = []

    async def on_search_assist(self, request: AssistQuestion) -> AssistReply | None:
        self.asked.append(request)
        picks = [AssistChoice(request.candidates[0].ref, "The move.")] if request.candidates else []
        return AssistReply(picks=picks, terms=["migration"])


@contextlib.asynccontextmanager
async def running(fake: FakeServer, handler: RecordingHandler) -> AsyncIterator[Adapter]:
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer="default",
        settings=fake.sdk_settings,
    )
    handler.clock = fake.clock
    adapter = Adapter(config, handler, clock=fake.clock)
    source: EventSource = RecordingSource(await adapter.make_source())
    adapter.source = source
    task = asyncio.ensure_future(adapter.run())
    await until(fake, lambda: any(isinstance(d, SourceReady) for d in source.deliveries))  # type: ignore[attr-defined]
    try:
        yield adapter
    finally:
        await adapter.stop()
        with contextlib.suppress(Exception):
            await within_patience(task)


async def test_the_adapter_answers_a_question_through_its_handler(fake: FakeServer) -> None:
    handler = AssistHandler()
    async with running(fake, handler):
        request_id = await fake.request_search_assist(
            "where did the steering session move",
            [{"ref": "c1", "title": "Takeover", "channel_name": "bothy", "excerpt": "moved"}],
        )
        await until(fake, lambda: fake.search_assist_answer(request_id) is not None)
    answer = fake.search_assist_answer(request_id)
    assert answer == {"picks": [{"ref": "c1", "reason": "The move."}], "terms": ["migration"]}
    asked = handler.asked[0]
    assert asked.request_id == request_id
    assert asked.q == "where did the steering session move"
    assert asked.candidates[0] == AssistCandidate(
        ref="c1",
        title="Takeover",
        channel_name="bothy",
        space_name=None,
        at=asked.candidates[0].at,
        excerpt="moved",
        shown_before=False,
    )
    assert asked.max_picks == 1


async def test_a_handler_without_the_hook_answers_nothing(fake: FakeServer) -> None:
    async with running(fake, RecordingHandler()):
        request_id = await fake.request_search_assist("anything", [])
        await fake.pass_time(1)
    assert fake.search_assist_answer(request_id) is None


def _request(clock: FakeClock, expires_in: float) -> AssistQuestion:
    return AssistQuestion(
        request_id=UUID("0192f4a0-7b1c-78f9-8074-fa94e84bab7d"),
        q="q",
        history=(),
        candidates=(),
        max_picks=0,
        max_terms=4,
        expires_at=clock.now() + timedelta(seconds=expires_in),
    )


async def test_the_guard_sends_nothing_after_expiry() -> None:
    clock = FakeClock()
    sent: list[AssistReply] = []

    async def send(_: UUID, answer: AssistReply) -> None:
        sent.append(answer)

    async def slow(_: AssistQuestion) -> AssistReply | None:
        await clock.sleep(10)
        return AssistReply()

    task = asyncio.ensure_future(guarded_search_assist(slow, _request(clock, 3), send, clock=clock))
    await eventually(lambda: clock.sleepers == 2, "both sleepers")
    await clock.advance(3)
    assert await task is False
    assert sent == []

    async def none(_: AssistQuestion) -> AssistReply | None:
        return None

    assert await guarded_search_assist(none, _request(clock, 3), send, clock=clock) is False
    assert await guarded_search_assist(none, _request(clock, 0), send, clock=clock) is False
    assert sent == []
