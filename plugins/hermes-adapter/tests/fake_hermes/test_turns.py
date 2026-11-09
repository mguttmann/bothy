# SPDX-License-Identifier: Apache-2.0
"""Turns of the pinned Hermes through the adapter: discovery, live box, answer, end (D-L7)."""

from __future__ import annotations

from typing import Any

import pytest
from fake_hermes_kit import ErrorTurn, HermesWorld, Profile, gateway_log, hermes_rig
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


INTERIM = "Now I remove the leftover line, fix the docstring, update the route description."


def _excerpts(rig: Any, run_id: Any) -> list[str]:
    return [
        str(op["value"])
        for q in rig.requests("patchRun")
        if str(run_id) in q.path
        for op in q.json["ops"]
        if op["path"] == "/last_text_excerpt"
    ]


@pytest.mark.parametrize("streaming", [True, False], ids=["streamed", "commentary"])
async def test_interim_text_before_a_tool_call_is_no_message(
    world: HermesWorld, streaming: bool
) -> None:
    """Text before a tool call is progress in the live box, never a message (D-ADAPTER-INTERIM-1).

    Streamed, Hermes finalizes the text in front of a tool boundary with a plain ``send``; not
    streamed, it sends the text as commentary marked ``_interim_send``. Both only show progress.
    """
    script = [
        ScriptTurn(text=(INTERIM,), tool_calls=(TODO,), paced=True),
        ScriptTurn(
            text=("The trip to the lake ", "is planned for ", "Saturday morning."), paced=True
        ),
    ]
    profile = Profile(config={"streaming": {"enabled": streaming}})
    async with hermes_rig(world, script, profile=profile, texts=[ASK, ANSWER]) as rig:
        triggered = await rig.fake.trigger(ASK, session_mode="new")
        try:
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert rig.fake.run_state(triggered.run_id).state == "completed", gateway_log(rig)
        assert [m["content_md"] for m in rig.fake.messages] == [ANSWER], gateway_log(rig)
        assert rig.fake.messages[0]["run_id"] == str(triggered.run_id)
        excerpts = _excerpts(rig, triggered.run_id)
        assert any(INTERIM in e for e in excerpts), excerpts


@pytest.mark.parametrize("after_tool", [False, True], ids=["first-request", "after-tool"])
async def test_error_text_of_a_failed_request_is_the_message(
    world: HermesWorld, after_tool: bool
) -> None:
    """A model request Hermes gives up on ends the loop without ``on_session_end``; the error
    text the gateway then sends is the turn's message, never interim (D-ADAPTER-INTERIM-1)."""
    refused = ErrorTurn(message="the fake provider refused the request")
    tool = ScriptTurn(text=(INTERIM,), tool_calls=(TODO,), paced=True)
    script = ([tool] if after_tool else []) + [refused] * 5
    async with hermes_rig(world, script, texts=[ASK]) as rig:
        triggered = await rig.fake.trigger(ASK, session_mode="new")
        try:
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        finals = [
            m["content_md"]
            for m in rig.fake.messages
            if "the fake provider refused the request" in m["content_md"]
            and "retrying won't help" not in m["content_md"]
        ]
        assert len(finals) == 1, gateway_log(rig)
        assert all(INTERIM not in m["content_md"] for m in rig.fake.messages), gateway_log(rig)


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
    from fake_hermes_kit import _start_gateway

    script = [ScriptTurn(text=(f"Answer {n}.",), paced=True) for n in range(5)]
    async with hermes_rig(world, script) as rig:
        fake = rig.fake

        async def finish(triggered: Any, what: str) -> None:
            try:
                await rig.until(lambda: rig.finished(triggered.run_id), what)
            except AssertionError:
                raise AssertionError(gateway_log(rig)) from None
            assert fake.run_state(triggered.run_id).state == "completed", gateway_log(rig)

        source = await fake.trigger("Remember the boat.", session_mode="new")
        await finish(source, "the first run")
        taken = fake.session_reports(source.thread_id)[-1]["external_session_ref"]
        # A session that ran in another thread of this gateway process: Hermes v0.22.5 keeps it
        # on that thread's route, and a8b0bc moves it, but the other thread's next /new ends it
        # there, so the adapter fails open instead of binding on every version (09 17.4,
        # D-HERMES-MAIN-4).
        other = await fake.trigger("Hello.", session_mode="new")
        await finish(other, "the second run")
        fake.start_session(other.thread_id, taken, started=True)
        refused = await fake.trigger("Go on.", thread_id=other.thread_id, session_mode="bind")
        await finish(refused, "the bind of a live session")
        report = fake.session_reports(other.thread_id)[-1]
        assert report["bind_failed"] == "session_missing", gateway_log(rig)
        assert report["external_session_ref"] != taken
        # After a restart the session is live nowhere: another thread takes it over (an import
        # or a rebinding).
        assert rig.gateway is not None
        rig.gateway.stop()
        identifies = sum(1 for f in fake.frames if f.get("op") == "identify")
        await _start_gateway(rig)
        await rig.until(
            lambda: sum(1 for f in fake.frames if f.get("op") == "identify") > identifies,
            "the new identify",
        )
        fake.start_session(other.thread_id, taken, started=False)
        bound = await fake.trigger("Go on.", thread_id=other.thread_id, session_mode="bind")
        await finish(bound, "the bound run")
        report = fake.session_reports(other.thread_id)[-1]
        assert report == {"external_session_ref": taken}, gateway_log(rig)
        # A session Hermes does not know: a new session, reported with bind_failed. The server
        # takes bind_failed only for a started row (09 9.5, else 409 session_not_bound), as
        # for an imported session that ran before.
        fake.start_session(other.thread_id, "20260101_000000_deadbeef", started=True)
        missing = await fake.trigger("Again.", thread_id=other.thread_id, session_mode="bind")
        await finish(missing, "the run of a missing session")
        report = fake.session_reports(other.thread_id)[-1]
        assert report["bind_failed"] == "session_missing", gateway_log(rig)
        assert report["external_session_ref"] not in (taken, "20260101_000000_deadbeef")


_SWITCH = """
import asyncio, json, sys, uuid
from types import SimpleNamespace
from gateway.config import Platform, PlatformConfig
import bothy_hermes_adapter.platform as module
# The gateway registers the platform at plugin discovery; this process runs no gateway.
Platform._add_pseudo_member(sys.argv[1])
module._session_known = lambda session_id: True
platform = module.ServerPlatform(PlatformConfig(enabled=True), sys.argv[1])
sent = []
async def control(thread_id, text):
    sent.append(text)
platform._control_command = control
holder, thread = uuid.uuid4(), uuid.uuid4()
platform._ran_here.update({"elsewhere": holder, "here": thread})
turn = SimpleNamespace(
    thread_id=thread, channel_id=uuid.uuid4(), user_id=uuid.uuid4(), user_name="Alex"
)
other = asyncio.run(platform.switch_session(turn, "bind", "elsewhere"))
own = asyncio.run(platform.switch_session(turn, "bind", "here"))
print(json.dumps({"other": str(other), "own": str(own), "sent": sent}))
"""


async def test_switch_session_fails_open_for_a_session_live_in_another_thread(
    world: HermesWorld,
) -> None:
    """D-HERMES-MAIN-4 on the method itself: ``_ran_here`` naming another thread sends no
    ``/resume`` and gives a new session; the thread's own session still binds."""
    import asyncio
    import json
    import subprocess

    from fake_hermes_kit import PLATFORM_KEY

    async with hermes_rig(world, [], start=False) as rig:
        done = await asyncio.to_thread(
            subprocess.run,
            [str(world.python), "-c", _SWITCH, PLATFORM_KEY],
            env=rig.env,
            cwd=rig.homes.work,
            capture_output=True,
            text=True,
            check=False,
        )
    assert done.returncode == 0, done.stderr
    result = json.loads(done.stdout.strip().splitlines()[-1])
    assert result == {"other": "new", "own": "bound", "sent": ["/resume here --all"]}
