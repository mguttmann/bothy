# SPDX-License-Identifier: Apache-2.0
"""The remaining ops of the Fake-Agent against the FakeServer: gateway, context, messages, cards,
titles, sessions, commands, interactions and token handover."""

# ruff: noqa: PT018, S106 - compound checks of one reply; token names are no secrets

from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Any
from uuid import UUID

import pytest
from _fake_agent_driver import Driver, private_jwk

from bothy_sdk.testing._patience import eventually
from bothy_sdk.testing.fakeserver import FakeServer

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


@pytest.fixture
async def driver(fake: FakeServer) -> AsyncIterator[Driver]:
    d = Driver(fake)
    yield d
    await d.close()


async def _started(fake: FakeServer, driver: Driver) -> str:
    await driver.hello()
    trig = await fake.trigger()
    await driver.op("receive", type="agent.invocation")
    started = await driver.op("start", run=str(trig.run_id))
    assert started["status"] == 200
    return str(trig.run_id)


async def test_gateway_receive_patch_and_autocomplete(fake: FakeServer) -> None:
    driver = Driver(fake)
    try:
        assert (
            await driver.op(
                "hello",
                base_url=fake.base_url,
                gateway_url="/api/v1/gateway",
                instance_id=str(fake.instance_id),
                agent_id=str(fake.agent_id),
                consumer="default",
                transport="gateway",
                credentials={
                    "token": fake.credentials.api_token.reveal(),
                    "dpop_private_jwk": private_jwk(fake),
                },
                runtime={"billing_mode": "organization", "tool_class": "scoped"},
                settings={
                    "sync.reconnect_backoff_base": "10ms",
                    "sync.reconnect_backoff_cap": "50ms",
                },
            )
            == {}
        )
        assert "head_seq" in await driver.op("connect")
        trig = await fake.trigger()
        got = await driver.op("receive", type="agent.invocation")
        assert got["verdict"] == "valid" and got["run_id"] == str(trig.run_id)
        started = await driver.op("start", run=str(trig.run_id))
        version = started["run_version"]
        ops = [{"op": "replace", "path": "/status_line", "value": "Working"}]
        stored = await driver.op("patch", run=str(trig.run_id), ops=ops)
        assert stored == {"status": 200, "current_version": version + 1}
        stale = await driver.op("patch", run=str(trig.run_id), base_version="stale", ops=ops)
        assert stale["code"] == "base_version_conflict"
        assert stale["rebased"]["status"] == 200
        command = await fake.register_command("ticket")
        choices = [{"name": "Ticket 47", "value": "47"}]
        assert await driver.op("autocomplete_mode", mode="answer", choices=choices) == {}
        request = await fake.request_autocomplete(command, "4")
        asked = await driver.op("receive", type="command.autocomplete_requested")
        assert asked["request_id"] == str(request)
        await eventually(lambda: fake.autocomplete_answer(request) is not None, "the answer")
        answered = fake.autocomplete_answer(request)
        assert answered is not None and [c.to_dict() for c in answered] == choices
    finally:
        await driver.close()


async def test_context_reports_trust_and_marked_blocks(fake: FakeServer, driver: Driver) -> None:
    run = await _started(fake, driver)
    got = await driver.op("context", run=run)
    assert got["status"] == 200 and got["trigger_complete"] is True
    assert got["elements"][-1]["trust"] == "trigger"
    assert got["by_message"][got["elements"][-1]["message_id"]]["trust"] == "trigger"
    assert got["context_bytes"] == sum(e["bytes"] for e in got["elements"][:-1])


async def test_messages_cards_and_await(fake: FakeServer, driver: Driver) -> None:
    run = await _started(fake, driver)
    thread = str(fake.run(UUID(run))["thread_id"])
    plain = await driver.op("post", thread=thread, run=run, content="Looking into it")
    assert plain["status"] == 201 and plain["message_id"]
    card = await driver.op("post", thread=thread, run=run, content="May I?", card="approval")
    assert card["status"] == 201 and "approval.once" in card["component_ids"]
    buttons = await driver.op(
        "post",
        thread=thread,
        run=run,
        content="Which month?",
        card="buttons",
        single_use=True,
        restricted_to="any",
        buttons=[
            {"custom_id": "march", "label": "March"},
            {"custom_id": "april", "label": "April"},
        ],
    )
    assert set(buttons["component_ids"]) == {"march", "april"}
    component = buttons["component_ids"]["march"]
    updated = await driver.op("update_component", component=component, version=1, state="disabled")
    assert updated["status"] == 200
    waiting = await driver.op("await_input", run=run, status_line="Waiting")
    assert waiting["status"] == 200
    interaction = await fake.deliver_interaction(UUID(run), "approval.once")
    responded = await driver.op("respond", interaction=str(interaction), text="Thanks")
    assert responded["status"] == 200


async def test_title_session_commands_and_tokens(fake: FakeServer, driver: Driver) -> None:
    run = await _started(fake, driver)
    thread = str(fake.run(UUID(run))["thread_id"])
    titled = await driver.op("set_title", thread=thread, title="Export repair")
    assert titled["status"] == 200 and titled["title_source"] == "auto"
    reported = await driver.op(
        "report_session", thread=thread, run=run, external_session_ref="ref-a", started=True
    )
    assert reported["status"] == 200
    commands: list[dict[str, Any]] = [
        {
            "name": "ticket",
            "description": "Open a ticket",
            "scope_kind": "channel",
            "scope_id": str(fake.world.channel_id),
        },
    ]
    registered = await driver.op("register_commands", commands=commands)
    assert registered["status"] == 201
    assert [r["name"] for r in registered["registered"]] == ["ticket"]
    assert registered["command_id"] == registered["registered"][0]["id"]
    exported = await driver.op("export_token", run=run)
    imported = await driver.op("import_token", token=exported["token"], name="copy", run=run)
    assert imported["jti_hash"] == exported["jti_hash"]
    replay = await driver.op("verify", token="copy", run=run)
    assert replay["verdict"] == "replay"
    finished = await driver.op("finish", run=run, state="completed", summary="Done")
    assert finished["status"] == 200
    assert (await driver.op("crash_not")) == {"error": "usage", "detail": "unknown op crash_not"}
