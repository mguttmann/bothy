# SPDX-License-Identifier: Apache-2.0
"""The support desk ticket flow against the pinned Hermes: title, done, an answer, reopen.

One forum post is one ticket. The agent shows the ticket's status in the post's title and marks
the post done with the command line a skill or a support desk backend of the profile runs
(``python -m bothy_hermes_adapter thread ...`` with the profile's Python). A person's answer in
the done post reaches Hermes like any message (the server reopens the post with it), so the
skill bound to the forum decides whether the answer needs work; the agent reopens a done post
explicitly as well.
"""

from __future__ import annotations

import asyncio
import json
from typing import Any
from uuid import UUID

import pytest
from fake_hermes_kit import HermesRig, HermesWorld, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

pytestmark = pytest.mark.anyio

FIRST = "The printer on floor 2 jams."
ANSWER = "It jams again after the fix."


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


async def _thread_cli(rig: HermesRig, *args: str) -> dict[str, Any]:
    # In a worker thread: the FakeServer answers the command on this test's event loop.
    done = await asyncio.to_thread(rig.installer, "thread", *args, "--profile-home", str(rig.home))
    assert done.stdout.strip(), done.stderr[-2000:]
    result: dict[str, Any] = json.loads(done.stdout.strip().splitlines()[-1])
    assert (done.returncode == 0) is bool(result.get("ok")), done.stderr[-2000:]
    return result


async def _done(rig: HermesRig, run_id: Any) -> None:
    try:
        await rig.until(lambda: rig.finished(run_id), "the end of the run")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None
    assert rig.fake.run_state(run_id).state == "completed", gateway_log(rig)


def _asked(rig: HermesRig, text: str) -> bool:
    return text in json.dumps(rig.model.raw)


async def test_title_done_answer_in_the_done_post_and_reopen(world: HermesWorld) -> None:
    script = [ScriptTurn(text=(f"Answer {n}.",), paced=True) for n in range(2)]
    async with hermes_rig(world, script, texts=[FIRST, ANSWER]) as rig:
        forum = rig.fake.add_forum("tickets", None)
        first = await rig.fake.trigger(FIRST, channel_id=forum)
        await _done(rig, first.run_id)
        thread = str(first.thread_id)

        titled = await _thread_cli(rig, "title", thread, "--title", "[IN PROGRESS] Printer")
        assert (titled["ok"], titled["title"], titled["title_source"]) == (
            True,
            "[IN PROGRESS] Printer",
            "auto",
        )
        assert (await _thread_cli(rig, "title", thread, "--title", "[DONE] Printer"))["ok"] is True
        closed = await _thread_cli(rig, "close", thread)
        assert (closed["ok"], closed["state"]) == (True, "closed")

        # The answer in the done post: reopened by the message and delivered to Hermes.
        answer = await rig.fake.trigger(ANSWER, channel_id=forum, thread_id=first.thread_id)
        assert rig.fake.reopened_by_message == [first.thread_id]
        await _done(rig, answer.run_id)
        assert _asked(rig, ANSWER), gateway_log(rig)

        # Done again, then reopened by the agent.
        assert (await _thread_cli(rig, "close", thread))["state"] == "closed"
        reopened = await _thread_cli(rig, "reopen", thread)
        assert (reopened["ok"], reopened["state"]) == (True, "open")
        assert rig.fake.thread(UUID(thread))["title"] == "[DONE] Printer"

        # A title a person set stays; the refusal names the server's code.
        rig.fake.set_manual_title(first.thread_id, "Printer floor 2")
        refused = await _thread_cli(rig, "title", thread, "--title", "[OPEN] Printer")
        assert (refused["ok"], refused.get("code")) == (False, "title_manual")
