# SPDX-License-Identifier: Apache-2.0
"""The interview hook of the adapter (docs/spec/26-builtin-agents.md 6.1, 6.2).

`agent_interview.request_from` turns the event into an `InterviewRequest`;
`guarded_agent_interview` runs the handler until ``expires_at``; the adapter calls
``on_agent_interview`` for ``agent.interview_requested`` and sends its answer once with
``answerAgentInterview``.
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
from bothy_sdk.agent_interview import (
    InterviewLimits,
    InterviewReply,
    InterviewRequest,
    InterviewTurn,
    guarded_agent_interview,
)
from bothy_sdk.generated.models import CostReport
from bothy_sdk.sources import EventSource, SourceReady
from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.doubles import FakeClock, RecordingHandler, RecordingSource
from bothy_sdk.testing.fakeserver import DEFAULT_INTERVIEW_LIMITS, FakeServer

pytestmark = pytest.mark.anyio

_DRAFT: dict[str, Any] = {
    "instructions": "You help Alex write short texts for example.com.\n",
    "skills": [],
    "mcp_servers": [],
    "settings": {"tool_class_wish": "chat"},
    "origin": {"kind": "interview"},
    "name": "Writing helper",
    "handle": "writing-helper",
    "description": "Writes short texts",
    "channel_suggestion": {"space_name": "Team", "channel_name": "writing"},
}


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


async def until(fake: FakeServer, predicate: Callable[[], Any]) -> None:
    await eventually(predicate, "the condition", step=lambda: fake.pass_time(0.05))


class BuilderHandler(RecordingHandler):
    """Asks one question, or drafts when the person asks for a draft."""

    def __init__(self) -> None:
        super().__init__()
        self.asked: list[InterviewRequest] = []

    async def on_agent_interview(self, request: InterviewRequest) -> InterviewReply | None:
        self.asked.append(request)
        costs = [CostReport(provider="anthropic", model="model-medium", currency="USD")]
        if request.want_draft:
            return InterviewReply.propose(_DRAFT, costs)
        return InterviewReply.ask("Who reads the texts?", costs=costs)


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


async def test_the_adapter_answers_with_questions(fake: FakeServer) -> None:
    handler = BuilderHandler()
    catalogue = [
        {
            "slug": "writing-helper",
            "name": "Writing helper",
            "description": "Writes texts",
            "tool_class_wish": "chat",
            "mcp_servers": [],
        }
    ]
    async with running(fake, handler):
        request_id = await fake.request_agent_interview(
            [{"role": "owner", "text": "A writing helper for example.com"}], catalogue=catalogue
        )
        await until(fake, lambda: fake.agent_interview_answer(request_id) is not None)
    answer = fake.agent_interview_answer(request_id)
    assert answer == {
        "kind": "questions",
        "questions": ["Who reads the texts?"],
        "costs": [{"provider": "anthropic", "model": "model-medium", "currency": "USD"}],
    }
    asked = handler.asked[0]
    assert asked.request_id == request_id
    assert asked.turns == (InterviewTurn("owner", "A writing helper for example.com"),)
    assert asked.want_draft is False
    assert asked.catalogue[0]["slug"] == "writing-helper"
    assert asked.limits.max_skills == DEFAULT_INTERVIEW_LIMITS["max_skills"]
    assert asked.limits.mcp_stdio_runners == ("npx", "uvx")


async def test_the_adapter_answers_with_a_draft(fake: FakeServer) -> None:
    async with running(fake, BuilderHandler()):
        request_id = await fake.request_agent_interview(
            [{"role": "owner", "text": "Draft it now"}], want_draft=True
        )
        await until(fake, lambda: fake.agent_interview_answer(request_id) is not None)
    answer = fake.agent_interview_answer(request_id)
    assert answer is not None
    assert answer["kind"] == "draft"
    assert answer["draft"]["handle"] == "writing-helper"
    assert answer["draft"]["channel_suggestion"] == {
        "space_name": "Team",
        "channel_name": "writing",
    }
    assert "questions" not in answer


async def test_a_handler_without_the_hook_answers_nothing(fake: FakeServer) -> None:
    async with running(fake, RecordingHandler()):
        request_id = await fake.request_agent_interview([])
        await fake.pass_time(1)
    assert fake.agent_interview_answer(request_id) is None


def test_the_body_drops_members_outside_the_schema() -> None:
    draft = dict(_DRAFT, hooks={"PreToolUse": []})
    body = InterviewReply.propose(draft).to_body().to_dict()
    assert "hooks" not in body["draft"]
    assert body["draft"]["name"] == "Writing helper"
    with pytest.raises(ValueError, match="questions or a draft"):
        InterviewReply(kind="plan").to_body()


def _request(clock: FakeClock, expires_in: float) -> InterviewRequest:
    return InterviewRequest(
        request_id=UUID("0192f4a0-7b1c-78f9-8074-fa94e84bab7d"),
        turns=(),
        want_draft=False,
        catalogue=(),
        limits=InterviewLimits(1, 1, 1, 1, 1, 1, 1, 1.0, ("npx",), 1),
        expires_at=clock.now() + timedelta(seconds=expires_in),
    )


async def test_the_guard_sends_nothing_after_expiry() -> None:
    clock = FakeClock()
    sent: list[InterviewReply] = []

    async def send(_: UUID, answer: InterviewReply) -> None:
        sent.append(answer)

    async def slow(_: InterviewRequest) -> InterviewReply | None:
        await clock.sleep(10)
        return InterviewReply.ask("late?")

    task = asyncio.ensure_future(
        guarded_agent_interview(slow, _request(clock, 3), send, clock=clock)
    )
    await eventually(lambda: clock.sleepers == 2, "both sleepers")
    await clock.advance(3)
    assert await task is False
    assert sent == []

    async def none(_: InterviewRequest) -> InterviewReply | None:
        return None

    assert await guarded_agent_interview(none, _request(clock, 3), send, clock=clock) is False
    assert await guarded_agent_interview(none, _request(clock, 0), send, clock=clock) is False
    assert sent == []
