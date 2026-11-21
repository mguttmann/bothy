# SPDX-License-Identifier: Apache-2.0
"""Team channel runs against the pinned Hermes (09 3.9.9, 17.6; FEAT-TEAM-CHANNEL-ADAPTER).

A member's input (a message of the agent Texter, the member ``@dev``) and its follow-ups reach
Hermes behind the member label, an owner's follow-up as it is; the turn knows its handle and the
members, and the answer that mentions ``@dev`` goes out with the run's own token.
"""

from __future__ import annotations

import asyncio
import json
from typing import Any

import pytest
from bothy_sdk.ids import uuid7
from fake_hermes_kit import HermesRig, HermesWorld, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

from bothy_hermes_adapter.team import member_label

pytestmark = pytest.mark.anyio

TODO = {
    "name": "todo_list",
    "arguments": {"todos": [{"content": "Test", "id": "1", "status": "pending"}]},
}
ASK = "Please test the login form."
MEMBER_FOLLOWUP = "Also try the reset link."
OWNER_FOLLOWUP = "Keep it short."
ANSWER = "Tests pass. @dev please deploy."


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _team(rig: HermesRig, source: str) -> dict[str, Any]:
    world = rig.fake.world
    own = {
        "member_id": str(uuid7()),
        "handle": "qa",
        "agent_id": str(world.lotse.id),
        "home_channel_id": str(world.channel_id),
    }
    dev = {
        "member_id": str(uuid7()),
        "handle": "dev",
        "agent_id": str(world.texter.id),
        "home_channel_id": str(uuid7()),
    }
    team: dict[str, Any] = {
        "channel_id": str(uuid7()),
        "member_id": own["member_id"],
        "handle": "qa",
        "source": source,
        "addressed": True,
        "members": [own, dev],
    }
    if source == "member":
        team["source_handle"] = "dev"
    return team


def _states(rig: HermesRig, message_id: Any) -> list[str]:
    return [
        op["value"]
        for q in rig.requests("patchRun")
        for op in q.json["ops"]
        if op["path"] == f"/followups/{message_id}/state"
    ]


async def _end(rig: HermesRig, run_id: Any) -> None:
    try:
        await rig.until(lambda: rig.finished(run_id), "the end of the run")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None


def _own_token_answers(rig: HermesRig, run_id: Any) -> list[str]:
    """The answers of the run; each carries a token of exactly this run (09 3.9.4)."""
    out = []
    for q in rig.requests("createMessage"):
        if (q.json or {}).get("run_id") != str(run_id):
            continue
        header = q.headers.get("invocation-token")
        assert header is not None, "an answer without the run's token"
        assert rig.fake.token_run(header) == run_id, "an answer with the token of another run"
        out.append(q.json["content_md"])
    return out


@pytest.mark.parametrize("source", ["member", "owner"])
async def test_team_run_and_followups_reach_hermes(world: HermesWorld, source: str) -> None:
    script = [
        ScriptTurn(tool_calls=(TODO,), hold="first"),
        ScriptTurn(text=("Tests pass. ", "@dev please deploy."), paced=True),
    ]
    texts = [ASK, MEMBER_FOLLOWUP, OWNER_FOLLOWUP]
    async with hermes_rig(world, script, texts=texts) as rig:
        alex = str(rig.fake.world.alex.id)
        author = "texter" if source == "member" else "alex"
        triggered = await rig.fake.trigger(
            ASK, trigger_kind="team", author=author, authority="alex", team=_team(rig, source)
        )
        await rig.until(rig.model.requests, "the first model request")
        await asyncio.to_thread(rig.model.wait_held, "first")
        member = await rig.fake.deliver_followup(
            triggered.run_id, MEMBER_FOLLOWUP, author="texter", claims={"sub": alex}
        )
        await rig.until(lambda: _states(rig, member) == ["delivered"], "the member's follow-up")
        owner = await rig.fake.deliver_followup(triggered.run_id, OWNER_FOLLOWUP, author="alex")
        await rig.until(lambda: _states(rig, owner) == ["delivered"], "the owner's follow-up")
        rig.model.release("first")
        await _end(rig, triggered.run_id)
        assert rig.fake.run_state(triggered.run_id).state == "completed", gateway_log(rig)
        bodies = json.dumps(rig.model.raw)
        label = json.dumps(member_label("dev"))[1:-1]
        assert "you are the member @qa" in bodies
        assert "Members: @qa (you), @dev." in bodies
        # The member's follow-up is labelled where it stands; the owner's is not.
        assert f"{label}\\n\\n{MEMBER_FOLLOWUP}" in bodies
        assert f"{label}\\n\\n{OWNER_FOLLOWUP}" not in bodies
        assert OWNER_FOLLOWUP in bodies
        if source == "member":
            assert f"{label}\\n\\n{ASK}" in bodies
            assert "request of the member @dev" in bodies
        else:
            assert f"{label}\\n\\n{ASK}" not in bodies
            assert "a message the owner wrote" in bodies
        assert _own_token_answers(rig, triggered.run_id) == [ANSWER]
        # The answer stays as written: an edited answer would hand nothing on (09 3.9.4).
        assert rig.requests("updateMessage") == []
