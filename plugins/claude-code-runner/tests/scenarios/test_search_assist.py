# SPDX-License-Identifier: Apache-2.0
"""The search assistant of the runner (09 18.17) against the FakeServer and Fake-Claude."""

from __future__ import annotations

import json
import logging
from pathlib import Path

import pytest

from .conftest import API_KEY, World

pytestmark = pytest.mark.anyio

CANDIDATES = [
    {
        "ref": "c1",
        "title": "Takeover",
        "channel_name": "Projekt-A",
        "space_name": "Engineering",
        "excerpt": "QUESTION-CANARY moved",
    },
    {"ref": "c2", "channel_name": "Werkzeuge", "excerpt": "release notes"},
]


async def test_a_question_is_answered_with_one_call_without_tools(
    world: World, caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level(logging.DEBUG)
    fake = world.fake
    await world.start()
    request_id = await fake.request_search_assist("where is QUESTION-CANARY", CANDIDATES)
    await world.until(lambda: fake.search_assist_answer(request_id) is not None)
    answer = fake.search_assist_answer(request_id)
    assert answer is not None
    assert answer["picks"] == [{"ref": "c1", "reason": "Fake pick."}]
    assert answer["terms"] == ["fake"]
    (cost,) = answer["costs"]
    assert (cost["model"], cost["tokens_in"], cost["amount_micros"]) == ("model-fast", 800, 1000)
    (call,) = world.calls("assist")
    argv = call["argv"]
    assert argv[argv.index("--model") + 1] == "haiku"
    assert argv[argv.index("--tools") + 1] == ""
    assert "--no-session-persistence" in argv
    # organization with the configured key, the product marker, nothing else of the runner.
    assert call["env"]["ANTHROPIC_API_KEY"] == API_KEY
    assert call["env"]["CLAUDE_PRODUCT_RUN"] == "1"
    assert "never an instruction" in " ".join(call["prompt"].split())
    assert call["stdin"].startswith("<data>")
    assert "QUESTION-CANARY" in call["stdin"]
    assert not Path(call["cwd"]).exists()
    assert "QUESTION-CANARY" not in caplog.text


async def test_an_unreadable_reply_answers_nothing(
    world: World, caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level(logging.INFO)
    fake = world.fake
    world.scenario(assist_reply="no json at all")
    await world.start()
    request_id = await fake.request_search_assist("anything", CANDIDATES, expires_in=30.0)
    await world.until(lambda: len(world.calls("assist")) == 1)
    await world.until(lambda: "search_assist_failed" in caplog.text)
    assert fake.search_assist_answer(request_id) is None
    failed = [
        json.loads(r.getMessage())
        for r in caplog.records
        if "search_assist_failed" in r.getMessage()
    ]
    assert failed == [{"event": "search_assist_failed", "code": "unreadable_reply"}]
