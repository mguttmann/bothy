# SPDX-License-Identifier: Apache-2.0
"""Follow-ups of people into a running turn (09 7, 18.5; design notes 2 to 5, AG-13).

Alex triggers runs in a thread of the forum; Sam writes follow-ups. A follow-up is read with
``getMessage``, written as one further stdin line with the lead-in and its own ``uuid``, patched
``delivered``, and ``absorbed`` once Fake-Claude echoes the line.
"""

from __future__ import annotations

import json
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.app import Runner

from .conftest import World

pytestmark = pytest.mark.anyio


@pytest.fixture
def world_timings() -> dict[str, float]:
    # Tokens and cancellations never expire on the server's timers inside a scenario.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def text_of(line: str) -> str:
    return str(json.loads(line)["message"]["content"][0]["text"])


def finishes(world: World, run_id: UUID) -> list[dict[str, Any]]:
    return [r.json for r in world.requests("finishRun") if str(run_id) in r.path]


def followup_patches(world: World) -> list[Any]:
    return [r.json for r in world.requests("patchRun") if "/followups/" in json.dumps(r.json)]


async def settled(world: World, runner: Runner) -> None:
    """Wait until the runner acknowledged every event the server has (bounded by patience)."""
    consumer = runner.config.consumer

    def done() -> bool:
        acked, head = world.fake.consumer_state(consumer)
        return acked >= head

    await world.until(done)


async def gated_turn(world: World, **scenario: Any) -> tuple[Runner, UUID, Any]:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate), **scenario)
    runner = await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    return runner, run.run_id, gate


async def test_followup_reaches_running_turn(world: World) -> None:
    _, run_id, gate = await gated_turn(world)
    message_id = await world.fake.deliver_followup(run_id, "Also check the footer", author="sam")
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "absorbed")
    assert len(world.requests("getMessage")) == 1
    stdin = world.turns()[0]["stdin"]
    assert len(stdin) == 2
    line = json.loads(stdin[1])
    assert line["type"] == "user"
    UUID(line["uuid"])
    assert text_of(stdin[1]) == "Follow-up from Sam (@sam):\n\nAlso check the footer"
    states = [
        op["value"]
        for body in followup_patches(world)
        for op in body["ops"]
        if op["path"] == f"/followups/{message_id}/state"
    ]
    assert states == ["delivered", "absorbed"]
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    beat = world.fake.heartbeats[-1]
    assert beat["details"]["capabilities"]["followups"] == "steer"


async def test_followups_held_until_turn_starts(world: World) -> None:
    gate = world.gate("auth_gate")
    world.scenario(fixture="plain", auth_gate=str(gate))
    runner = await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)  # prepare waits: not started
    first = await world.fake.deliver_followup(run.run_id, "First", author="sam")
    second = await world.fake.deliver_followup(run.run_id, "Second", author="sam")
    await settled(world, runner)
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    stdin = world.turns()[0]["stdin"]
    assert [text_of(x).split("\n\n", 1)[1] for x in stdin[1:]] == ["First", "Second"]
    for message_id in (first, second):
        assert world.fake.followup_state(run.run_id, message_id) == "absorbed"


async def test_followup_grace_keeps_stdin_until_absorbed(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate), echo="after_result")
    await world.start(world.config(followup_grace=f"{int(PATIENCE)}s"))
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    message_id = await world.fake.deliver_followup(run.run_id, "One more", author="sam")
    await world.until(lambda: world.fake.followup_state(run.run_id, message_id) == "delivered")
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert world.fake.followup_state(run.run_id, message_id) == "absorbed"
    assert [m["content_md"] for m in world.fake.messages] == ["Done.", "Follow-up 1 handled."]
    [finish] = finishes(world, run.run_id)
    assert [c["tokens_in"] for c in finish["costs"]] == [1100]  # from the second result


async def test_followup_grace_expires_leaves_delivered(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate), echo="never")
    await world.start(world.config(followup_grace="200ms"))
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    message_id = await world.fake.deliver_followup(run.run_id, "One more", author="sam")
    await world.until(lambda: world.fake.followup_state(run.run_id, message_id) == "delivered")
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert world.fake.followup_state(run.run_id, message_id) == "delivered"
    assert len(world.fake.messages) == 1


async def test_followup_after_stdin_closed_is_not_written(world: World, logs: Any) -> None:
    end = world.gate("end_gate")
    world.scenario(fixture="plain", end_gate=str(end))
    runner = await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)  # result out, stdin closed
    message_id = await world.fake.deliver_followup(run.run_id, "Too late", author="sam")
    await settled(world, runner)
    await world.until(lambda: "followup_not_deliverable" in logs.text)
    world.release(end)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.turns()[0]["stdin"]) == 1
    assert world.fake.followup_state(run.run_id, message_id) == "queued"
    assert followup_patches(world) == []


async def test_followup_redelivery_written_once(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate), echo="never")
    runner = await world.start(world.config(followup_grace="200ms"))
    run_id = (await world.fake.trigger("Fix the form", channel_id=world.forum_a)).run_id
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    message_id = await world.fake.deliver_followup(run_id, "Once", author="sam")
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "delivered")
    await world.fake.redeliver()
    await settled(world, runner)
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    assert len(world.turns()[0]["stdin"]) == 2


async def test_followup_deleted_or_empty_not_written(world: World) -> None:
    gate = world.gate("auth_gate")
    world.scenario(fixture="plain", auth_gate=str(gate))
    runner = await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    deleted = await world.fake.deliver_followup(run.run_id, "Gone", author="sam")
    empty = await world.fake.deliver_followup(run.run_id, "Blank", author="sam")
    unknown = await world.fake.deliver_followup(run.run_id, "Hidden", author="sam")
    world.fake.edit_message(deleted, deleted=True)
    world.fake.edit_message(empty, content_md="  ")
    world.fake.hide_message(unknown)
    await settled(world, runner)
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.turns()[0]["stdin"]) == 1
    for message_id in (deleted, empty, unknown):
        assert world.fake.followup_state(run.run_id, message_id) == "queued"
    assert followup_patches(world) == []
    assert len(world.requests("getMessage")) == 3


async def test_followup_invalid_token_not_written_run_continues(world: World) -> None:
    runner, run_id, gate = await gated_turn(world)
    fake = world.fake
    alex = str(fake.world.alex.id)
    not_author = await fake.deliver_followup(
        run_id, "Sub is not me", author="sam", claims={"sub": alex}
    )
    other_run = await fake.deliver_followup(
        run_id, "Other run", author="sam", claims={"run_id": "0192d3a0-0000-7000-8000-0000000000f1"}
    )
    entry = next(
        e
        for e in fake.hub.log
        if e.type == "agent.followup" and e.envelope["payload"]["message_id"] == str(other_run)
    )
    await fake.deliver_with_token(
        UUID(entry.envelope["id"]), fake.forge_token(run_id, "bad_signature")
    )
    await settled(world, runner)
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    assert len(world.turns()[0]["stdin"]) == 1
    assert followup_patches(world) == []
    for message_id in (not_author, other_run):
        assert fake.followup_state(run_id, message_id) == "queued"


async def test_followup_target_change_not_written_run_continues(world: World) -> None:
    runner, run_id, gate = await gated_turn(world)
    personal = await world.fake.deliver_followup(
        run_id, "Use my plan", author="sam", claims={"billing_mode": "personal"}
    )
    chat = await world.fake.deliver_followup(
        run_id, "Chat only", author="sam", claims={"tool_class": "chat"}
    )
    await settled(world, runner)
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    assert len(world.turns()[0]["stdin"]) == 1
    assert followup_patches(world) == []
    for message_id in (personal, chat):
        assert world.fake.followup_state(run_id, message_id) == "queued"


async def test_followup_text_with_slash_is_plain(world: World) -> None:
    _, run_id, gate = await gated_turn(world)
    message_id = await world.fake.deliver_followup(run_id, "/compact", author="sam")
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "absorbed")
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    stdin = world.turns()[0]["stdin"]
    assert text_of(stdin[1]) == "Follow-up from Sam (@sam):\n\n/compact"
    assert world.calls("compact") == []
    assert len(world.turns()) == 1


async def test_unechoed_followups_rewritten_after_compaction(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(
        fixture="happy",
        per_process={
            "0": {
                "turn_gate": str(gate),
                "echo": "never",
                "api_error": "context_full",
                "api_error_after": 1,
            }
        },
    )
    await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    message_id = await world.fake.deliver_followup(run.run_id, "Keep the tests", author="sam")
    await world.until(lambda: world.fake.followup_state(run.run_id, message_id) == "delivered")
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    first, _, retry = world.turns()
    assert retry["stdin"][1] == first["stdin"][1]  # the same line with its original uuid
    assert world.fake.followup_state(run.run_id, message_id) == "absorbed"


async def test_context_full_on_a_followup_retries_only_unanswered_inputs(world: World) -> None:
    # The prompt was answered, the follow-up filled the context: the retry carries the
    # follow-up only, never the answered prompt again (D-M2-35-19).
    gate = world.gate("turn_gate")
    first = {"turn_gate": str(gate), "api_error_on_followup": "context_full"}
    world.scenario(fixture="plain", per_process={"0": first})
    await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    message_id = await world.fake.deliver_followup(run.run_id, "Also the footer", author="sam")
    await world.until(lambda: world.fake.followup_state(run.run_id, message_id) == "absorbed")
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    failed, compaction, retry = world.turns()
    assert [text_of(x) for x in compaction["stdin"]] == ["/compact"]
    assert retry["stdin"] == [failed["stdin"][1]]  # the follow-up with its uuid, no prompt
    assert len(world.fake.messages) == 2  # the prompt's answer, then the follow-up's


async def test_context_full_after_a_folded_followup_retries_only_the_unanswered(
    world: World,
) -> None:
    # The CLI folded the first follow-up into the running turn (its result names both inputs
    # in user_message_uuids); the second filled the context: the retry carries the second only.
    gate = world.gate("turn_gate")
    first = {"turn_gate": str(gate), "fold_queued": 1, "api_error_on_followup": "context_full"}
    world.scenario(fixture="plain", per_process={"0": first})
    await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    m1 = await world.fake.deliver_followup(run.run_id, "Also the footer", author="sam")
    await world.until(lambda: world.fake.followup_state(run.run_id, m1) == "absorbed")
    m2 = await world.fake.deliver_followup(run.run_id, "And the header", author="sam")
    await world.until(lambda: world.fake.followup_state(run.run_id, m2) == "absorbed")
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.calls("folded")) == 1
    failed, compaction, retry = world.turns()
    assert [text_of(x) for x in compaction["stdin"]] == ["/compact"]
    assert retry["stdin"] == [failed["stdin"][2]]  # the second follow-up only
    assert len(world.fake.messages) == 2  # the folded turn's answer, then the second's
