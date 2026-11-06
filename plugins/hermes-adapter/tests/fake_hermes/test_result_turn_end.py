# SPDX-License-Identifier: Apache-2.0
"""A result turn that fails, with the pinned Hermes (09 17.12; FIX-hermes-result-turn-end).

When the turn Hermes starts by itself with a delegation's result ends with an error (here the
model refuses the request with HTTP 400), the pinned version returns from its agent loop without
``finalize_turn``, so the plugin hook ``on_session_end`` never comes. The run must still end: it
ends ``failed`` with ``runtime_error``, and Hermes' error text is a message of the run.
"""

from __future__ import annotations

import asyncio
import json
from typing import Any

import pytest
from fake_hermes_kit import (
    BoundTurn,
    HermesRig,
    HermesWorld,
    Profile,
    _RawHandler,
    gateway_log,
    hermes_rig,
)

pytestmark = pytest.mark.anyio

GOAL = "Count the boats on the lake."
DELEGATE = {"name": "delegate_task", "arguments": {"tasks": [{"goal": GOAL, "context": "none"}]}}
STARTED = "Started a team."
PROFILE = Profile(toolsets=("todo", "delegation", "terminal"))
REFUSAL = {"type": "error", "error": {"type": "invalid_request_error", "message": "bad parameter"}}


class _RefusingHandler(_RawHandler):
    """Refuses every request of the thread's own session (it offers ``delegate_task``) once the
    server's ``refuse`` is set; the subagent's requests offer no ``delegate_task``."""

    def do_POST(self) -> None:  # noqa: N802 - http.server dispatch name
        owner = self.server.owner  # type: ignore[attr-defined]
        if not getattr(owner, "refuse", False):
            super().do_POST()
            return
        raw = self.read_body()
        try:
            body = json.loads(raw) if raw else {}
        except ValueError:
            body = {}
        tools = [
            t.get("name") or (t.get("function") or {}).get("name")
            for t in (body.get("tools") or [])
            if isinstance(t, dict)
        ]
        if "delegate_task" in tools:
            owner.refused += 1
            self.reply(400, json.dumps(REFUSAL).encode(), "application/json")
            return
        self.read_body = lambda: raw  # type: ignore[method-assign]  # the body was read above
        super().do_POST()


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _steps(rig: HermesRig, run_id: Any, kind: str) -> list[dict[str, Any]]:
    return [s for s in rig.fake.run(run_id)["steps"].values() if s["kind"] == kind]


async def test_failed_result_turn_ends_the_run_failed(world: HermesWorld) -> None:
    script = [
        BoundTurn(tool_calls=(DELEGATE,), requires_tool="delegate_task"),
        BoundTurn(text=(STARTED,), paced=True, requires_tool="delegate_task"),
        BoundTurn(text=("Seven boats.",), hold="child", without_tool="delegate_task"),
    ]
    async with hermes_rig(world, script, profile=PROFILE, tool_class="host") as rig:
        model = rig.model
        model.refuse = False  # type: ignore[attr-defined]
        model.refused = 0  # type: ignore[attr-defined]
        model._server.RequestHandlerClass = _RefusingHandler
        triggered = await rig.fake.trigger("Count the boats with a team.", session_mode="new")
        try:
            await rig.until(lambda: rig.fake.messages, "the first answer")
            await asyncio.to_thread(model.wait_held, "child")
            await rig.until(lambda: _steps(rig, triggered.run_id, "subagent"), "the subagent step")
            model.refuse = True  # type: ignore[attr-defined]
            model.release("child")
            await rig.until(lambda: model.refused > 0, "the refused result turn")  # type: ignore[attr-defined]
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        view = rig.fake.run_state(triggered.run_id)
        assert (view.state, view.state_reason) == ("failed", "runtime_error"), gateway_log(rig)
        [step] = _steps(rig, triggered.run_id, "subagent")
        assert step["status"] == "completed"
        texts = [(m["content_md"], m["run_id"]) for m in rig.fake.messages]
        assert texts[0] == (STARTED, str(triggered.run_id))
        # Hermes' error text of the failed turn is shown in the run, not as a report.
        shown = [text for text, run in texts if run == str(triggered.run_id)]
        assert any("HTTP 400" in text for text in shown), texts
