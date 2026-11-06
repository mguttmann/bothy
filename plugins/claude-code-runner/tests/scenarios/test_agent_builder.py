# SPDX-License-Identifier: Apache-2.0
"""The builder of the interview (26 6.2) against the FakeServer and Fake-Claude (BA-T15)."""

from __future__ import annotations

import json
import logging
from pathlib import Path

import pytest

from .conftest import API_KEY, World

pytestmark = pytest.mark.anyio

TURNS = [
    {"role": "owner", "text": "A writing helper for example.com"},
    {"role": "builder", "text": "Who reads the texts?"},
    {"role": "owner", "text": "Customers. ANSWER-CANARY"},
]

DRAFT = {
    "instructions": "You help Alex write short texts for example.com.\n",
    "skills": [],
    "mcp_servers": [],
    "settings": {"tool_class_wish": "chat"},
    "origin": {"kind": "interview"},
    "name": "Writing helper",
    "handle": "writing-helper",
    "description": "Writes short texts",
}


async def test_the_builder_answers_with_one_call_without_tools(
    world: World, caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level(logging.DEBUG)
    fake = world.fake
    await world.start(world.config(agent_builder=True))
    request_id = await fake.request_agent_interview(TURNS)
    await world.until(lambda: fake.agent_interview_answer(request_id) is not None)
    answer = fake.agent_interview_answer(request_id)
    assert answer is not None
    assert answer["kind"] == "questions"
    assert answer["questions"] == ["Fake question?"]
    (cost,) = answer["costs"]
    assert (cost["model"], cost["tokens_in"]) == ("model-medium", 900)
    (call,) = world.calls("builder")
    argv = call["argv"]
    assert argv[argv.index("--model") + 1] == "sonnet"
    assert argv[argv.index("--tools") + 1] == ""
    assert argv[argv.index("--disallowedTools") + 1] == "mcp__*"
    assert "--no-session-persistence" in argv
    assert "--resume" not in argv
    assert "--session-id" not in argv
    # organization with the configured key, the product marker, nothing else of the runner.
    assert call["env"]["ANTHROPIC_API_KEY"] == API_KEY
    assert call["env"]["CLAUDE_PRODUCT_RUN"] == "1"
    assert "untrusted content" in " ".join(call["prompt"].split())
    assert call["stdin"].startswith("<data>")
    assert "ANSWER-CANARY" in call["stdin"]
    assert not Path(call["cwd"]).exists()
    assert "ANSWER-CANARY" not in caplog.text


async def test_a_draft_goes_to_the_server(world: World) -> None:
    fake = world.fake
    world.scenario(builder_reply=json.dumps({"kind": "draft", "draft": DRAFT}))
    await world.start(world.config(agent_builder=True))
    request_id = await fake.request_agent_interview(TURNS, want_draft=True)
    await world.until(lambda: fake.agent_interview_answer(request_id) is not None)
    answer = fake.agent_interview_answer(request_id)
    assert answer is not None
    assert answer["kind"] == "draft"
    assert answer["draft"]["handle"] == "writing-helper"
    (call,) = world.calls("builder")
    assert '"want_draft": true' in call["stdin"]


async def test_without_the_key_the_runner_does_not_answer(world: World) -> None:
    fake = world.fake
    await world.start()
    request_id = await fake.request_agent_interview(TURNS, expires_in=30.0)
    await fake.pass_time(1)
    assert fake.agent_interview_answer(request_id) is None
    assert world.calls("builder") == []


@pytest.mark.parametrize(
    ("reply", "rule"),
    [
        ("no json at all", None),
        (
            json.dumps({"kind": "draft", "draft": {**DRAFT, "hooks": {"PreToolUse": []}}}),
            "definition_rules",
        ),
        (
            json.dumps({"kind": "draft", "draft": {**DRAFT, "instructions": "Run !`id` first."}}),
            "definition_rules",
        ),
        (json.dumps({"kind": "questions", "questions": ["a?", "b?", "c?"]}), "question_count"),
    ],
)
async def test_an_answer_that_fails_the_check_is_not_sent(
    world: World, caplog: pytest.LogCaptureFixture, reply: str, rule: str | None
) -> None:
    caplog.set_level(logging.INFO)
    fake = world.fake
    world.scenario(builder_reply=reply)
    await world.start(world.config(agent_builder=True))
    request_id = await fake.request_agent_interview(TURNS, expires_in=30.0)
    await world.until(lambda: len(world.calls("builder")) == 1)
    await world.until(lambda: "agent_builder_failed" in caplog.text)
    assert fake.agent_interview_answer(request_id) is None
    failed = [
        json.loads(r.getMessage())
        for r in caplog.records
        if "agent_builder_failed" in r.getMessage()
    ]
    want = {
        "event": "agent_builder_failed",
        "code": "invalid_answer" if rule else "unreadable_reply",
    }
    if rule:
        want["rule"] = rule
    assert failed == [want]
    assert "ANSWER-CANARY" not in caplog.text


async def test_the_time_limit_ends_the_call(world: World, caplog: pytest.LogCaptureFixture) -> None:
    caplog.set_level(logging.INFO)
    fake = world.fake
    gate = world.gate("builder_gate")
    world.scenario(builder_gate=str(gate))
    await world.start(world.config(agent_builder=True, agent_builder_timeout="1s"))
    request_id = await fake.request_agent_interview(TURNS, expires_in=30.0)
    await world.until(lambda: "agent_builder_failed" in caplog.text)
    world.release(gate)
    assert fake.agent_interview_answer(request_id) is None
    assert '"code":"timeout"' in caplog.text
    assert not any((world.state / "agent-builder").iterdir())
