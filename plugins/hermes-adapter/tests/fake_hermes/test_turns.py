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


async def test_slash_text_never_runs_a_hermes_command(world: HermesWorld) -> None:
    script = [ScriptTurn(text=("Taken ", "as text."), paced=True)]
    async with hermes_rig(world, script, texts=["/new"]) as rig:
        triggered = await rig.fake.trigger("/new")
        try:
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert rig.fake.run_state(triggered.run_id).state == "completed", gateway_log(rig)
        # The text reached the model as conversation; no confirmation card of /new appeared.
        assert [r for r in rig.model_requests(scripted=True) if "/new" in r["scenario_texts"]]
        assert [m["kind"] for m in rig.fake.messages] == ["text"]


async def test_new_session_after_a_restart(world: HermesWorld) -> None:
    from fake_hermes_kit import _start_gateway

    script = [ScriptTurn(text=(f"Answer {n}.",), paced=True) for n in range(2)]
    async with hermes_rig(world, script) as rig:
        first = await rig.fake.trigger("Remember the boat.", session_mode="new")
        await rig.until(lambda: rig.finished(first.run_id), "the first run")
        before = rig.fake.session_reports(first.thread_id)[-1]["external_session_ref"]
        assert rig.gateway is not None
        rig.gateway.stop()
        identifies = sum(1 for f in rig.fake.frames if f.get("op") == "identify")
        await _start_gateway(rig)
        await rig.until(
            lambda: sum(1 for f in rig.fake.frames if f.get("op") == "identify") > identifies,
            "the new identify",
        )
        reset = await rig.fake.trigger("Start over.", thread_id=first.thread_id, session_mode="new")
        try:
            await rig.until(lambda: rig.finished(reset.run_id), "the second run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert rig.fake.run_state(reset.run_id).state == "completed", gateway_log(rig)
        after = rig.fake.session_reports(first.thread_id)[-1]
        assert after["started"] is True
        assert after["external_session_ref"] != before, gateway_log(rig)


async def test_bind_takes_a_session_over_or_fails_open(world: HermesWorld) -> None:
    script = [ScriptTurn(text=(f"Answer {n}.",), paced=True) for n in range(4)]
    async with hermes_rig(world, script) as rig:
        fake = rig.fake
        source = await fake.trigger("Remember the boat.", session_mode="new")
        await rig.until(lambda: rig.finished(source.run_id), "the first run")
        taken = fake.session_reports(source.thread_id)[-1]["external_session_ref"]
        # Another thread binds the first thread's session (an import or a rebinding).
        other = await fake.trigger("Hello.", session_mode="new")
        await rig.until(lambda: rig.finished(other.run_id), "the second run")
        fake.start_session(other.thread_id, taken, started=False)
        bound = await fake.trigger("Go on.", thread_id=other.thread_id, session_mode="bind")
        await rig.until(lambda: rig.finished(bound.run_id), "the bound run")
        report = fake.session_reports(other.thread_id)[-1]
        assert report == {"external_session_ref": taken}, gateway_log(rig)
        # A session Hermes does not know: a new session, reported with bind_failed.
        fake.start_session(other.thread_id, "20260101_000000_deadbeef", started=False)
        missing = await fake.trigger("Again.", thread_id=other.thread_id, session_mode="bind")
        await rig.until(lambda: rig.finished(missing.run_id), "the run of a missing session")
        report = fake.session_reports(other.thread_id)[-1]
        assert report["bind_failed"] == "session_missing", gateway_log(rig)
        assert report["external_session_ref"] not in (taken, "20260101_000000_deadbeef")
        assert fake.run_state(missing.run_id).state == "completed"
