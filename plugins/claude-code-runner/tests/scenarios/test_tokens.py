# SPDX-License-Identifier: Apache-2.0
"""The invocation token decides before any runtime starts (AZ-P6, AG-08, 09 4.5, 4.5.1)."""

from __future__ import annotations

import asyncio
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.auth import Secret
from bothy_sdk.sources import Envelope
from bothy_sdk.testing.doubles import KEEP, REMOVE, RecordingSource, TamperingSource
from bothy_sdk.testing.faults import Fault

from .conftest import World

pytestmark = pytest.mark.anyio


async def test_az_p6_runner_never_starts_runtime(world: World) -> None:
    fake = world.fake
    plan: dict[UUID, asyncio.Future[Any]] = {}

    def slot(run_id: UUID) -> asyncio.Future[Any]:
        return plan.setdefault(run_id, asyncio.get_running_loop().create_future())

    async def rewrite(env: Envelope) -> Any:
        run_id = getattr(getattr(env.event, "payload", None), "run_id", None)
        if env.type != "agent.invocation" or not isinstance(run_id, UUID):
            return KEEP
        return await slot(run_id)

    await world.start(wrap=lambda inner: TamperingSource(inner, rewrite))
    invalid: list[UUID] = []
    for variant in ("missing", "bad_signature", "aud_other_agent"):
        run = await fake.trigger(f"AZ-P6 {variant}", channel_id=world.forum_a)
        slot(run.run_id).set_result(
            REMOVE if variant == "missing" else fake.forge_token(run.run_id, variant)
        )
        invalid.append(run.run_id)
    for run_id in invalid:
        assert await world.ended(run_id) == "failed"
        assert fake.run_state(run_id).state_reason == "invalid_invocation_token"

    # The token expires while prepare waits for auth status; the renewal is refused.
    fake.inject("renewInvocationToken", Fault.problem("agent_invocation_denied"))
    gate = world.gate("auth_gate")
    world.scenario(fixture="plain", auth_gate=str(gate))
    expired = await fake.trigger("AZ-P6 expired", channel_id=world.forum_a)
    slot(expired.run_id).set_result(KEEP)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    # Step by step, so heartbeats and their answers keep up with the clock. The pre-check
    # answers only after the token has expired, however long these steps take.
    for _ in range(int(fake.timings["agent.invocation_token_ttl"]) + 20):
        await fake.pass_time(1.0)
        await asyncio.sleep(0.005)
    world.release(gate)
    await world.until(lambda: len(world.requests("renewInvocationToken")) == 1)
    await asyncio.sleep(0.5)
    assert fake.run_state(expired.run_id).state == "queued"

    world.scenario(fixture="plain")
    fake.world.billing_mode = "personal"
    personal = await fake.trigger("AZ-P6 billing", channel_id=world.forum_a)
    slot(personal.run_id).set_result(KEEP)
    assert await world.ended(personal.run_id) == "failed"
    assert fake.run_state(personal.run_id).state_reason == "billing_mode_mismatch"
    fake.world.billing_mode = "organization"

    fake.world.tool_class = "chat"
    tool = await fake.trigger("AZ-P6 tool class", channel_id=world.forum_a)
    slot(tool.run_id).set_result(KEEP)
    assert await world.ended(tool.run_id) == "failed"
    assert fake.run_state(tool.run_id).state_reason == "tool_class_mismatch"

    assert world.turns() == []
    assert world.requests("startRun") == []


async def test_az_p6_repeated_jti_gives_one_turn(world: World) -> None:
    world.scenario(fixture="plain")
    recording: dict[str, RecordingSource] = {}

    def wrap(inner: Any) -> Any:
        recording["source"] = RecordingSource(inner)
        return recording["source"]

    await world.start(wrap=wrap)
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    first = recording["source"].envelopes("agent.invocation")[0]
    assert isinstance(first.token, Secret)
    replay = await world.fake.deliver_with_token(first.id, first.token)
    await world.until(lambda: any(e.id == replay for e in recording["source"].envelopes()))
    await asyncio.sleep(0.5)
    assert len(world.turns()) == 1


async def test_expired_token_renewed_before_start(world: World) -> None:
    world.scenario(fixture="plain")
    world.fake.inject("startRun", Fault.problem("invocation_token_invalid", reason="expired"))
    await world.start()
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.requests("renewInvocationToken")) == 1
    assert len(world.requests("startRun")) == 2
    assert len(world.turns()) == 1


async def test_superseded_waits_for_a_newer_token(world: World) -> None:
    world.scenario(fixture="plain")
    world.fake.inject("startRun", Fault.problem("invocation_token_invalid", reason="superseded"))
    await world.start()
    acked, _ = world.fake.consumer_state("default")
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    await world.until(lambda: len(world.requests("startRun")) == 1)
    await asyncio.sleep(0.5)
    assert world.turns() == []
    assert world.fake.run_state(run.run_id).state == "queued"
    # A redelivery of the same event brings a newer token (a restart of the server here).
    await world.fake.restart("default", acked_seq=acked)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.turns()) == 1
