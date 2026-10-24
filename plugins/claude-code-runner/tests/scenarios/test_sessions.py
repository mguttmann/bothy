# SPDX-License-Identifier: Apache-2.0
"""Sessions per thread, cost differences, restart and the end of a turn (09 18.1, 18.4, 18.7)."""

from __future__ import annotations

import json
import os
from typing import Any

import pytest
from bothy_sdk.testing.faults import Fault

from .conftest import World

pytestmark = pytest.mark.anyio


def _canary_item(world: World, text: str) -> dict[str, Any]:
    return {
        "message_id": "0192d3a0-0000-7000-8000-00000000c001",
        "thread_seq": 1,
        "created_at": "2026-10-21T09:00:00.000Z",
        "author": world.fake.world.robin.ref(),
        "role": "user",
        "origin": "human",
        "kind": "text",
        "channel_id": str(world.forum_a),
        "content_md": text,
        "attachments": [],
        "trust": "authorized",
    }


async def test_session_new_then_resume_and_cost_difference(world: World) -> None:
    fake = world.fake
    fake.inject("finishRun", Fault.stored_then_502())
    await world.start()
    first = await fake.trigger(
        "Fix it",
        channel_id=world.forum_a,
        context={"thread_context": [_canary_item(world, "CONTEXT-CANARY-1")]},
    )
    assert await world.ended(first.run_id) == "completed"

    def finishes() -> list[Any]:
        return [r for r in world.requests("finishRun") if str(first.run_id) in r.path]

    await world.until(lambda: len(finishes()) == 2)
    one_finish, two_finish = finishes()
    assert one_finish.json["costs"] == two_finish.json["costs"]
    ref = world.requests("startRun")[0].json["external_session_ref"]

    world.scenario(fixture="resume")
    second = await fake.trigger(
        "Add a test",
        thread_id=first.thread_id,
        context={"thread_context": [], "context_floor_seq": 1},
    )
    assert await world.ended(second.run_id) == "completed"
    one, two = world.turns()
    assert one["argv"][one["argv"].index("--session-id") + 1] == ref
    assert two["argv"][two["argv"].index("--resume") + 1] == ref
    assert "--session-id" not in two["argv"]
    assert len(world.requests("reportAgentSession")) == 1
    assert len(world.requests("setThreadTitle")) == 1
    assert "TITLE:" not in two["system_prompt"]
    assert "CONTEXT-CANARY-1" in one["stdin"][0]
    assert "CONTEXT-CANARY-1" not in two["stdin"][0]
    finish = [r for r in world.requests("finishRun") if str(second.run_id) in r.path][-1].json
    assert finish["costs"] == [
        {
            "provider": "anthropic",
            "model": "model-large",
            "tokens_in": 3000,
            "tokens_out": 30,
            "tokens_cache_read": 2000,
            "tokens_cache_write": 100,
            "amount_micros": 7500,
            "currency": "USD",
        }
    ]
    usage = json.loads((world.state / "sessions" / ref / "usage.json").read_text())
    assert usage["pending"]["run_id"] == str(second.run_id)


async def test_ag_19_title_manual_keeps_run_completed(world: World) -> None:
    fake = world.fake
    await world.start()
    original = fake.world.thread_meta

    class Manual(dict):  # every new thread starts with a manual title
        def __setitem__(self, key: Any, value: Any) -> None:
            value["title_source"] = "manual"
            super().__setitem__(key, value)

    fake.world.thread_meta = Manual(original)
    run = await fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert world.requests("getThread")
    assert world.requests("setThreadTitle") == []
    assert fake.thread(run.thread_id)["title"] == ""


async def test_ag_19_title_conflict_not_retried(world: World) -> None:
    fake = world.fake
    fake.inject("setThreadTitle", Fault.problem("title_manual"))
    await world.start()
    run = await fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.requests("setThreadTitle")) == 1


@pytest.mark.parametrize("variant", ["error_result", "no_result"])
async def test_error_result_and_exit_without_result(world: World, variant: str) -> None:
    if variant == "error_result":
        world.scenario(fixture="error_result")
    else:
        world.scenario(fixture="plain", no_result=True, exit_code=3, exit_without_eof=True)
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert world.fake.messages == []
    steps = [s for s in world.fake.run(run.run_id)["steps"].values() if s["kind"] == "status"]
    assert [(s["name"], s["title"], s["status"]) for s in steps] == [
        ("runtime_error", "", "failed")
    ]
    assert steps[0]["detail"] == {"exit": "0" if variant == "error_result" else "3"}
    assert world.requests("setThreadTitle") == []


@pytest.mark.parametrize("stubborn", [False, True])
async def test_cancelled_execute_terminates_group(world: World, stubborn: bool) -> None:
    world.scenario(fixture="plain", hold=True, ignore_sigint=stubborn, ignore_sigterm=stubborn)
    runner = await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.turns()) == 1)
    pid = world.turns()[0]["pid"]
    await world.fake.cancel(run.run_id)
    assert await world.ended(run.run_id) == "cancelled"
    await world.until(lambda: not runner.handler._runs)
    _assert_group_gone(pid)
    for name in (
        f"thread-{run.thread_id}",
        f"session-{world.requests('startRun')[0].json['external_session_ref']}",
    ):
        _assert_lock_free(world.state / "locks" / f"{name}.lock")


def _assert_group_gone(pgid: int) -> None:
    try:
        os.killpg(pgid, 0)
    except ProcessLookupError:
        return
    raise AssertionError("a process of the turn's group is left")


def _assert_lock_free(path: Any) -> None:
    import fcntl

    fd = os.open(path, os.O_RDWR)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    finally:
        os.close(fd)


async def test_inbox_survives_restart(world: World) -> None:
    world.scenario(fixture="plain", auth_gate=str(world.gate("auth_gate")))
    first = await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.stop(first)  # after ack, before startRun; the pre-check never answers
    assert world.requests("startRun") == []
    world.scenario(fixture="plain")
    await world.start()
    assert await world.ended(run.run_id) == "completed"
    assert len(world.requests("startRun")) == 1
    assert len(world.turns()) == 1


async def test_failed_run_reports_its_cost_once(world: World) -> None:
    world.scenario(fixture="error_result")
    await world.start()
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "failed"
    failed = [r for r in world.requests("finishRun") if str(first.run_id) in r.path][-1].json
    assert failed["reason"] == "runtime_error"
    assert [c["tokens_in"] for c in failed["costs"]] == [300]
    world.scenario(fixture="resume")
    second = await world.fake.trigger("Again", thread_id=first.thread_id)
    assert await world.ended(second.run_id) == "completed"
    done = [r for r in world.requests("finishRun") if str(second.run_id) in r.path][-1].json
    assert [c["tokens_in"] for c in done["costs"]] == [5700 - 300]


async def test_children_of_a_turn_end_with_the_run(world: World) -> None:
    world.scenario(fixture="plain", spawn_child=True)
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    child = world.calls("child")[0]["child_pid"]
    await world.until(lambda: _gone(child), 20)


def _gone(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    return False


async def test_undelivered_reply_fails_the_run(world: World) -> None:
    from bothy_sdk.testing.faults import Fault as _Fault

    world.scenario(fixture="plain")
    world.fake.inject("createMessage", _Fault.disconnect(), times=50)
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "adapter_error"
