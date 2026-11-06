# SPDX-License-Identifier: Apache-2.0
"""The runner hosts a built-in agent against the real server of this tree (A1-04, 26 3).

The runner (Lotse) starts with ``runner.hosted_agents``; its heartbeat reports ``hosts_agents``,
so the owner can create a built-in agent hosted by it. The server tells the runner with
``agent.hosting_changed``; the runner fetches the agent's token (bound to its own DPoP key),
opens the agent's connection, and the agent turns online. A message to the agent ends its run
``failed`` with ``adapter_error`` and the cause ``definition_missing`` (26 5.6 item 4).
"""

from __future__ import annotations

import asyncio
import time
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.ids import uuid7
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.sdkserver import ApiAnswer, RealServer

from claude_code_runner.causes import cause

from .conftest import RealWorld

pytestmark = [pytest.mark.anyio, pytest.mark.e2e]


def _ok(answer: ApiAnswer, *statuses: int) -> Any:
    assert answer.status in (statuses or (200, 201)), f"status {answer.status}: {answer.body}"
    return answer.body


async def _until(what: str, check: Any) -> Any:
    deadline = time.monotonic() + PATIENCE
    while True:
        value = await check()
        if value:
            return value
        if time.monotonic() > deadline:
            raise AssertionError(f"not within {PATIENCE} s: {what}")
        await asyncio.sleep(0.1)


async def _create_hosted(real: RealServer) -> UUID:
    """The owner creates a built-in agent hosted by Lotse, once Lotse reports ``hosts_agents``."""
    body = {
        "handle": f"helper-{uuid7().hex[-8:]}",
        "display_name": "Writing Helper",
        "adapter_kind": "claude_agent",
        "hosted_by": str(real.agent_id),
    }

    async def create() -> UUID | None:
        answer = await real.call("createAgent", as_="owner", body=body)
        if answer.status == 400:  # host_not_capable until the heartbeat with the capability
            return None
        return UUID(_ok(answer, 201)["id"])

    created: UUID = await _until("a built-in agent hosted by the runner", create)
    return created


async def _presence(real: RealServer, agent_id: UUID) -> str:
    agent = _ok(await real.call("getAgent", as_="owner", path={"agent_id": agent_id}))
    return str((agent.get("presence") or {}).get("state"))


async def _operated_forum(real: RealServer, agent_id: UUID) -> UUID:
    """A forum without a binding in Support whose operator is the built-in agent."""
    forum = UUID(
        _ok(
            await real.call(
                "createChannel",
                as_="owner",
                path={"space_id": real.world.space_id},
                body={"kind": "forum", "name": f"Drafts {uuid7().hex[-6:]}"},
            ),
            201,
        )["id"]
    )
    current = _ok(await real.call("getChannel", as_="owner", path={"channel_id": forum}))
    _ok(
        await real.call(
            "setChannelOperator",
            as_="owner",
            path={"channel_id": forum},
            body={"mode": "agent", "agent_id": str(agent_id), "reason": "e2e"},
            header={"If-Match": f'"{current["channel"]["version"]}"'},
        )
    )
    return forum


async def test_runner_hosts_a_built_in_agent(world: RealWorld) -> None:
    real = world.real
    await world.start(world.config(hosted_agents=True, hosted_max_tool_class="scoped"))
    agent_id = await _create_hosted(real)

    async def online() -> bool:
        return await _presence(real, agent_id) == "online"

    # Token issued and the agent's own connection open: its heartbeat made it present.
    await _until("the hosted agent online", online)
    hosted = _ok(await real.call("getAgent", as_="owner", path={"agent_id": agent_id}))
    assert hosted["hosted_by"] == str(real.agent_id)

    forum = await _operated_forum(real, agent_id)
    body = {"title": "Draft", "first_message": {"client_msg_id": str(uuid7()), "content_md": "Hi"}}
    created = _ok(await real.call("createThread", path={"channel_id": forum}, body=body), 201)
    run_id, _ = await real.wait_run(UUID(created["first_message"]["id"]))

    async def ended() -> Any:
        view = await real.run_state(run_id)
        return view if view.state in ("completed", "failed", "cancelled") else None

    view = await _until("the end of the hosted run", ended)
    assert (view.state, view.state_reason, view.starts) == ("failed", "adapter_error", 0)
    run = _ok(await real.call("getRun", as_="owner", path={"run_id": run_id}))
    assert run.get("summary") == cause("definition_missing")
    assert world.turns() == []  # no turn of Claude Code for a hosted agent yet
