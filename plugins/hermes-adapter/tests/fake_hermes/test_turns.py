# SPDX-License-Identifier: Apache-2.0
"""Turns of the pinned Hermes through the adapter: discovery, live box, answer, end (D-L7)."""

from __future__ import annotations

import pytest
from fake_hermes_kit import HermesWorld, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

pytestmark = pytest.mark.anyio

ASK = "Please plan the trip to the lake."
ANSWER = "The trip to the lake is planned for Saturday morning."
TODO = {
    "name": "todo_list",
    "arguments": {"todos": [{"content": "Book", "id": "1", "status": "pending"}]},
}


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _ops(rig: object) -> list[str]:
    return [r.operation_id for r in rig.fake.requests]  # type: ignore[attr-defined]


async def test_plugin_found_by_real_discovery(world: HermesWorld) -> None:
    async with hermes_rig(world, []) as rig:
        identify = next(f for f in rig.fake.frames if f.get("op") == "identify")
        assert identify["d"]["consumer"] == "default"
        assert (rig.home / "plugins" / "bothy" / "plugin.yaml").exists()
        assert "installed" in rig.installed_output
        beat = rig.fake.heartbeats[-1] if rig.fake.heartbeats else None
        assert beat is None or beat["state"] in ("online", "degraded")


async def test_happy_path_steps_drafts_answer(world: HermesWorld) -> None:
    script = [
        ScriptTurn(tool_calls=(TODO,)),
        ScriptTurn(
            text=("The trip to the lake ", "is planned for ", "Saturday morning."), paced=True
        ),
    ]
    async with hermes_rig(world, script, texts=[ASK, ANSWER]) as rig:
        rig.fake.enable_costs()
        triggered = await rig.fake.trigger(ASK, session_mode="new")
        try:
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        view = rig.fake.run_state(triggered.run_id)
        assert view.state == "completed", gateway_log(rig)
        ops = _ops(rig)
        assert ops.index("startRun") < ops.index("createMessage") < ops.index("finishRun")
        assert [m["content_md"] for m in rig.fake.messages] == [ANSWER]
        assert rig.fake.messages[0]["run_id"] == str(triggered.run_id)
        doc = rig.fake.run(triggered.run_id)
        steps = [s for s in doc["steps"].values() if s["kind"] == "tool_call"]
        assert steps
        assert all(s["status"] == "completed" for s in steps)
        assert doc.get("last_text_excerpt")
        finish = rig.requests("finishRun")[-1].json
        assert finish["state"] == "completed"
        assert finish["costs"]
        assert all(c["currency"] == "USD" for c in finish["costs"])
        reports = rig.fake.session_reports(triggered.thread_id)
        assert reports
        assert reports[0]["started"] is True
        assert reports[0]["external_session_ref"]
