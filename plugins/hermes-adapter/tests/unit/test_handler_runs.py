# SPDX-License-Identifier: Apache-2.0
"""Runs through HermesHandler and the SDK loop on the FakeServer (Design notes 5 to 16).

The world of the FakeServer: space Support with channel Allgemein, the agent Lotse (this
adapter), Alex and Sam who trigger, Writer with write only, Robin the owner, the agent Texter.
Hermes is the fake runtime of ``adapter_kit``.
"""

from __future__ import annotations

import asyncio
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from adapter_kit import FakeRuntime, Rig, captured_logs, card_script, rig
from bothy_sdk.auth import Secret
from bothy_sdk.ids import uuid7
from bothy_sdk.inbox import EntryState
from bothy_sdk.sources import Envelope
from bothy_sdk.testing import fingerprint
from bothy_sdk.testing.doubles import KEEP
from bothy_sdk.testing.fakeserver import ContextFile, ContextMessage
from bothy_sdk.testing.faults import Fault

from bothy_hermes_adapter.runtime import (
    CardKind,
    CardRequest,
    ResolvedCredential,
    TurnOutcome,
    TurnRequest,
    Usage,
)

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _ops(r: Rig) -> list[str]:
    return [q.operation_id for q in r.fake.requests]


def _patch_paths(r: Rig, run_id: UUID) -> list[tuple[str, str]]:
    out = []
    for req in r.requests("patchRun"):
        if req.path.endswith(str(run_id)):
            out += [(op["op"], op["path"]) for op in req.json["ops"]]
    return out


def _finish(r: Rig, run_id: UUID) -> dict[str, Any]:
    finishes = [q for q in r.requests("finishRun") if str(run_id) in q.path]
    assert finishes, "no finishRun"
    body: dict[str, Any] = finishes[-1].json
    return body


# ---- happy path, live box, answer and finish ------------------------------------------------


async def test_status_draft_answer_and_finish_order(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        ev = rt.events
        ev.on_status(turn.thread_id, "Searching the files")
        ev.on_draft(turn.thread_id, "Partial")
        ev.on_status(turn.thread_id, "Reading notes")
        ev.on_status(turn.thread_id, None)
        ev.on_usage(turn.thread_id, Usage("anthropic", "model-small", 10, 5, 2, 1))
        ev.on_usage(turn.thread_id, Usage("anthropic", "model-small", 1, 1, currency="EUR"))
        long = "x" * 40
        msg = await ev.on_answer(turn.thread_id, long, str(turn.message_id))
        assert msg is not None
        assert await ev.on_edit(turn.thread_id, msg, "edited " + "y" * 40, False)
        assert not await ev.on_edit(turn.thread_id, "foreign", "nope", True)
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, max_message_bytes=30) as r:
        r.fake.enable_costs()
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id))
    view = r.fake.run_state(triggered.run_id)
    assert view.state == "completed"
    ops = _ops(r)
    # The answer, its one update, then the end.
    assert ops.index("createMessage") < ops.index("updateMessage") < ops.index("finishRun")
    assert len(r.fake.messages) == 1
    assert len(r.fake.messages[0]["content_md"].encode()) == 30
    assert r.fake.message_updates[0]["body"]["content_md"] == ("edited " + "y" * 40)[:30]
    paths = _patch_paths(r, triggered.run_id)
    adds = [p for op, p in paths if op == "add" and p.startswith("/steps/")]
    assert len(adds) == 2
    # Patches are coalesced per throttle window: the status line was set and cleared.
    assert any(p == "/status_line" for _, p in paths)
    assert ("replace", "/last_text_excerpt") in paths
    body = _finish(r, triggered.run_id)
    assert body["state"] == "completed"
    costs = {c["currency"]: c for c in body["costs"]}
    assert costs["USD"]["tokens_in"] == 10
    assert costs["USD"]["tokens_cache_read"] == 2
    assert costs["EUR"]["tokens_out"] == 1
    assert body["metrics"]["tokens_in"] == 11
    doc = r.fake.run(triggered.run_id)
    steps = list(doc["steps"].values())
    assert all(s["kind"] == "tool_call" for s in steps)
    assert all(s["status"] == "completed" for s in steps)
    assert [s["title"] for s in steps] == ["Searching the files", "Reading notes"]


async def test_tool_events_become_steps(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        ev = rt.events
        ev.on_tool_event(turn.thread_id, "read notes.txt", "call-1", False)
        ev.on_tool_event(turn.thread_id, "list files", None, False)
        ev.on_tool_event(turn.thread_id, "", None, True)
        ev.on_tool_event(turn.thread_id, "", "call-1", True)
        ev.on_tool_event(turn.thread_id, "", "unknown", True)
        await ev.on_answer(turn.thread_id, "Read.", None)
        return TurnOutcome.COMPLETED

    async with rig(tmp_path, runtime=FakeRuntime(script=script)) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id))
    steps = list(r.fake.run(triggered.run_id)["steps"].values())
    assert sorted(s["title"] for s in steps) == ["list files", "read notes.txt"]
    assert all(s["status"] == "completed" and "finished_at" in s for s in steps)
    assert all(set(s["detail"]) == {"preview"} for s in steps)
    assert len(r.fake.messages) == 1


async def test_failed_turn_and_send_without_run(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.events.on_status(turn.thread_id, "Working")
        return TurnOutcome.FAILED

    async with rig(tmp_path, runtime=FakeRuntime(script=script)) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id))
        assert await r.handler.on_answer(triggered.thread_id, "late", None) is None
        assert await r.handler.on_answer(uuid7(), "elsewhere", None) is None
        assert r.handler.active_run(triggered.thread_id) is None
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", "runtime_error")
    steps = list(r.fake.run(triggered.run_id)["steps"].values())
    assert [s["status"] for s in steps if s["kind"] == "tool_call"] == ["failed"]
    assert r.fake.messages == []


async def test_title_after_first_completed_new_session(tmp_path: Path) -> None:
    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt) as r:
        first = await r.fake.trigger(session_mode="new")
        rt.titles[first.thread_id] = "Lake trip"
        await r.until(lambda: r.finished(first.run_id))
        second = await r.fake.trigger("again", thread_id=first.thread_id, session_mode="resume")
        await r.until(lambda: r.finished(second.run_id))
        guest = await r.fake.trigger("guest")
        rt.titles[guest.thread_id] = "Guest title"
        await r.until(lambda: r.finished(guest.run_id))
        manual = await r.fake.trigger(session_mode="new")
        rt.titles[manual.thread_id] = "Agent title"
        r.fake.set_manual_title(manual.thread_id, "Mine")
        await r.until(lambda: r.finished(manual.run_id))
    assert r.fake.titles == [{"thread_id": str(first.thread_id), "title": "Lake trip"}]
    puts = r.requests("setThreadTitle")
    assert len(puts) == 1
    assert puts[0].headers["if-match"]
    # A manual title is never touched: the adapter reads the thread and stops.
    assert all(str(manual.thread_id) not in q.path for q in puts)


# ---- intake, start, restart -----------------------------------------------------------------


async def test_ack_after_inbox_before_context(tmp_path: Path) -> None:
    order: list[str] = []
    async with rig(tmp_path) as r:
        inbox_add = r.loop.inbox.add
        settle = r.source.settle

        async def add(entry: Any) -> None:
            await inbox_add(entry)
            order.append("inbox")

        async def settled(seq: int) -> None:
            order.append("ack")
            await settle(seq)

        r.loop.inbox.add = add  # type: ignore[method-assign] # record the order
        r.source.settle = settled  # type: ignore[method-assign] # record the order
        original = r.handler.prepare

        async def prepare(inv: Any, run: Any) -> Any:
            order.append("prepare")
            return await original(inv, run)

        r.handler.prepare = prepare  # type: ignore[method-assign] # record the order
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id))

        async def closed() -> bool:
            entries = await r.loop.inbox.for_run(triggered.run_id)
            return bool(entries) and entries[0].state is EntryState.CLOSED

        # The SDK closes the entry after finishRun answered, which may be after the server ended.
        await r.until(closed, "the inbox entry to close")
    assert order[:3] == ["inbox", "ack", "prepare"]
    ops = _ops(r)
    assert ops.index("getRunContext") < ops.index("startRun")
    assert r.runtime.turns
    assert r.runtime.turns[0].run_id == triggered.run_id


async def test_intake_outcomes(tmp_path: Path) -> None:
    # The forged run's only delivery carries the forged token (AG-08): its genuine delivery never
    # reaches the adapter, so no turn can race the refusal.
    forged: dict[UUID, asyncio.Future[Secret]] = {}

    async def rewrite(env: Envelope) -> Any:
        run_id = getattr(getattr(env.event, "payload", None), "run_id", None)
        if env.type != "agent.invocation" or run_id not in forged:
            return KEEP
        return await forged[run_id]

    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt, tamper=rewrite) as r:
        fake = r.fake
        queued = await fake.trigger()
        await r.until(lambda: r.finished(queued.run_id))
        # A forged token on a start event ends the run with the reason of 09 4.5.
        forged_run = fake.world.trigger("forged")
        forged[forged_run.id] = asyncio.get_running_loop().create_future()
        forged[forged_run.id].set_result(fake.forge_token(forged_run.id, "bad_signature"))
        await r.until(lambda: fake.run_state(forged_run.id).state == "failed")
        assert fake.run_state(forged_run.id).state_reason == "invalid_invocation_token"
        assert fake.run_state(forged_run.id).starts == 0
    assert [t.run_id for t in rt.turns] == [queued.run_id]


@pytest.mark.parametrize("kind", ["command", "interaction"])
async def test_prepare_refuses_command_and_interaction(tmp_path: Path, kind: str) -> None:
    from types import SimpleNamespace

    from bothy_hermes_adapter.handler import PrepareRefused

    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt) as r:
        inv = SimpleNamespace(payload=SimpleNamespace(trigger_kind=kind))
        run = SimpleNamespace(run_id=uuid7())
        with pytest.raises(PrepareRefused):
            await r.handler.prepare(inv, run)  # type: ignore[arg-type] # refused before any use
    assert rt.turns == []


async def test_allowlist_absent_fails_the_run(tmp_path: Path) -> None:
    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt) as r:
        r.fake.inject("getAgentTriggerers", Fault.problem("not_found"))
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id))
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", "adapter_error")
    assert rt.turns == []


@pytest.mark.parametrize(
    ("credential", "token_mode", "reason"),
    [
        (ResolvedCredential("anthropic", "api_key"), "personal", "billing_mode_mismatch"),
        (ResolvedCredential("anthropic", "oauth"), "organization", "billing_mode_mismatch"),
        (ResolvedCredential(None, "unknown"), "organization", "billing_mode_mismatch"),
        (RuntimeError("resolver"), "organization", "billing_mode_mismatch"),
    ],
    ids=["api-key-vs-personal", "oauth-vs-organization", "unknown", "resolver-error"],
)
async def test_actual_mode_unequal_target_starts_nothing(
    tmp_path: Path, credential: Any, token_mode: str, reason: str
) -> None:
    rt = FakeRuntime(credential=credential)
    async with rig(tmp_path, runtime=rt, billing_mode=token_mode) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id))
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", reason)
    assert rt.turns == []
    assert r.requests("startRun") == []


async def test_tool_class_above_token_starts_nothing(tmp_path: Path) -> None:
    rt = FakeRuntime(toolsets=frozenset({"terminal"}))
    async with rig(tmp_path, runtime=rt, tool_class="chat") as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id))
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", "tool_class_mismatch")
    assert rt.turns == []


async def test_start_carries_mode_class_and_evidence(tmp_path: Path) -> None:
    rt = FakeRuntime(toolsets=frozenset({"todo", "clarify"}))
    async with rig(tmp_path, runtime=rt) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id))
    body = r.requests("startRun")[0].json
    assert body["billing_mode_used"] == "organization"
    assert body["tool_class_used"] == "chat"
    assert body["billing_evidence"] == {
        "provider": "anthropic",
        "key_type": "api_key",
        "method": "resolved_credentials",
    }


async def test_redelivery_replaces_token_and_starts_once(tmp_path: Path) -> None:
    gate = asyncio.Event()
    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt) as r:
        original = r.handler.prepare

        async def slow_prepare(inv: Any, run: Any) -> Any:
            await gate.wait()
            return await original(inv, run)

        r.handler.prepare = slow_prepare  # type: ignore[method-assign] # hold the start
        # The ack is lost with the connection: the server delivers again with a new token.
        r.fake.inject("ack", Fault.disconnect())
        triggered = await r.fake.trigger()
        await r.until(lambda: len(r.source.envelopes("agent.invocation")) >= 2)
        gate.set()
        await r.until(lambda: r.finished(triggered.run_id))
    starts = r.requests("startRun")
    assert len(starts) == 1
    assert len(rt.turns) == 1
    delivered = [fingerprint(e.token) for e in r.source.envelopes("agent.invocation")]
    assert len(set(delivered)) >= 2
    assert starts[0].json["invocation_token"] == delivered[-1]


async def test_start_answers_expired_then_renewal(tmp_path: Path) -> None:
    gate = asyncio.Event()
    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt) as r:
        original = r.handler.prepare

        async def slow_prepare(inv: Any, run: Any) -> Any:
            await gate.wait()
            return await original(inv, run)

        r.handler.prepare = slow_prepare  # type: ignore[method-assign] # hold the start
        triggered = await r.fake.trigger()
        await r.until(lambda: r.loop._runs.get(triggered.run_id) is not None)
        ttl = r.fake.timings["agent.invocation_token_ttl"] + r.fake.timings["auth.clock_leeway"]
        await r.fake.pass_time(ttl + 1)
        gate.set()
        await r.until(lambda: r.finished(triggered.run_id))
    ops = _ops(r)
    assert ops.index("renewInvocationToken") < ops.index("startRun")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    assert len(rt.turns) == 1


async def test_restart_resumes_inbox(tmp_path: Path) -> None:
    from bothy_sdk.testing.fakeserver import FakeServer

    async with FakeServer() as fake:
        gate = asyncio.Event()
        first = FakeRuntime()
        async with rig(tmp_path, runtime=first, fake=fake) as r:

            async def never(inv: Any, run: Any) -> Any:
                await gate.wait()
                raise AssertionError("not reached")

            r.handler.prepare = never  # type: ignore[method-assign] # the process dies here
            triggered = await fake.trigger()
            await r.until(lambda: r.loop._runs.get(triggered.run_id) is not None)
        ttl = fake.timings["agent.invocation_token_ttl"] + fake.timings["auth.clock_leeway"]
        await fake.pass_time(ttl + 1)
        second = FakeRuntime()
        async with rig(tmp_path, runtime=second, fake=fake) as r2:
            await r2.until(lambda: r2.finished(triggered.run_id))
        assert len(second.turns) == 1
        assert first.turns == []
        renewals = [q for q in fake.requests if q.operation_id == "renewInvocationToken"]
        assert len(renewals) == 1
        assert len([q for q in fake.requests if q.operation_id == "startRun"]) == 1


async def test_jti_store_survives_restart(tmp_path: Path) -> None:
    from bothy_sdk.testing.fakeserver import FakeServer

    async with FakeServer() as fake:
        async with rig(tmp_path, fake=fake) as r:
            triggered = await fake.trigger()
            await r.until(lambda: r.finished(triggered.run_id))
            event = next(
                e
                for e in fake.hub.log
                if e.envelope["payload"].get("run_id") == str(triggered.run_id)
            )
            token = r.source.envelopes("agent.invocation")[0].token
        async with rig(tmp_path, fake=fake) as r2:
            assert token is not None
            replay = await fake.deliver_with_token(UUID(event.envelope["id"]), token)
            await r2.until(lambda: any(e.id == replay for e in r2.source.envelopes()))
            await fake.pass_time(0.5)
            assert await r2.loop.inbox.get(replay) is None
        assert len([q for q in fake.requests if q.operation_id == "startRun"]) == 1


async def test_change_of_actual_mode_during_run_aborts(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.events.on_status(turn.thread_id, "Working")
        await rt.stop_event(turn.thread_id).wait()
        rt.events.on_status(turn.thread_id, "After the switch")
        return TurnOutcome.CANCELLED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: rt.turns)
        await r.until(lambda: r.requests("patchRun"))
        patches = len(r.requests("patchRun"))
        rt.credential = ResolvedCredential("anthropic", "oauth")
        beat = r.loop.heartbeat().to_dict()
        assert beat["reported_billing_mode"] == "personal"
        await r.until(lambda: r.finished(triggered.run_id))
        assert len(r.requests("patchRun")) == patches
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", "billing_mode_mismatch")
    assert rt.interrupted == [triggered.thread_id]


# ---- follow-ups, cards, stop ----------------------------------------------------------------


async def test_followup_steer_queue_and_waiting_input(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        await rt.stop_event(turn.thread_id).wait()
        await rt.events.on_answer(turn.thread_id, "Done.", None)
        return TurnOutcome.COMPLETED

    for mode in ("steer", "queue"):
        rt = FakeRuntime(script=script)
        async with rig(tmp_path / mode, runtime=rt, busy_mode=mode) as r:
            triggered = await r.fake.trigger()
            await r.until(lambda: rt.turns)
            message = await r.fake.deliver_followup(triggered.run_id, "one more thing")
            if mode == "steer":
                await r.until(lambda: rt.steered)
                assert rt.steered == [(triggered.thread_id, "one more thing", message)]
            else:
                await r.until(lambda: r.source.envelopes("agent.followup"))
                await r.fake.pass_time(0.5)
                assert rt.steered == []
            rt.stop_event(triggered.thread_id).set()
            await r.until(lambda: r.finished(triggered.run_id))
            states = [
                op["value"]
                for q in r.requests("patchRun")
                for op in q.json["ops"]
                if op["path"].startswith("/followups/")
            ]
            assert states == (["delivered", "absorbed"] if mode == "steer" else [])
            assert r.handler.capabilities()["followups"] == mode


async def test_followup_answers_a_waiting_card(tmp_path: Path) -> None:
    question = CardRequest(CardKind.CLARIFY, "c1", "Which file?\nPick one.", ("a.txt", "b.txt"))
    for mode in ("steer", "queue"):
        rt = FakeRuntime(script=card_script(question))
        async with rig(tmp_path / mode, runtime=rt, busy_mode=mode) as r:
            triggered = await r.fake.trigger()
            await r.until(lambda: r.fake.run_state(triggered.run_id).state == "waiting_input")
            await r.fake.deliver_followup(triggered.run_id, "b.txt please")
            await r.until(lambda: r.finished(triggered.run_id))
            assert r.fake.run_state(triggered.run_id).state == "completed"
            assert rt.answered == ["b.txt please"]
            assert rt.steered == []
            assert len(r.requests("resumeRun")) == 1
            disabled = list(r.requests("updateComponent"))
            assert len(disabled) == 2
            assert all(q.json == {"state": "disabled"} for q in disabled)


async def test_cards_and_interactions(tmp_path: Path) -> None:
    requests = [
        CardRequest(CardKind.CLARIFY, "c1", "Which one?", ("Red", "Blue")),
        CardRequest(CardKind.APPROVAL, "a1", "Run rm?", ("once", "session", "always", "deny"), "k"),
        CardRequest(CardKind.SLASH_CONFIRM, "f1", "Reload?", (), "k", "/reload"),
    ]
    picks = ["cl:c1:1", "appr:a1:session", "sc:always:f1"]
    for request, pick in zip(requests, picks, strict=True):
        rt = FakeRuntime(script=card_script(request))
        async with rig(tmp_path / request.hermes_id, runtime=rt) as r:
            triggered = await r.fake.trigger()
            await r.until(lambda: r.fake.run_state(triggered.run_id).state == "waiting_input")
            card = r.fake.messages[0]
            assert card["kind"] == "card"
            comps = {c["custom_id"]: c for c in card["components"]}
            assert all(c["single_use"] is True for c in comps.values())
            await r.fake.click(triggered.run_id, pick)
            await r.until(lambda: r.finished(triggered.run_id))
            assert r.fake.run_state(triggered.run_id).state == "completed"
            assert len(rt.resolved) == 1
            responds = r.requests("respondToInteraction")
            assert len(responds) == 1
            assert responds[0].json["state"] == "acknowledged"
            assert r.requests("updateComponent") == []
        if request.kind is CardKind.CLARIFY:
            assert list(comps) == ["cl:c1:0", "cl:c1:1"]
            assert all(c["restricted_to"] == "run_starter" for c in comps.values())
            assert rt.resolved == [("cl:c1:1", "Blue")]
        elif request.kind is CardKind.APPROVAL:
            assert list(comps) == [
                "appr:a1:once",
                "appr:a1:session",
                "appr:a1:always",
                "appr:a1:deny",
            ]
            assert comps["appr:a1:always"]["restricted_to"] == "owner"
            assert comps["appr:a1:deny"].get("required_capability") is None
            assert comps["appr:a1:once"]["required_capability"] == "approve_actions"
        else:
            assert list(comps) == ["sc:once:f1", "sc:always:f1", "sc:cancel:f1"]
            assert comps["sc:always:f1"]["restricted_to"] == "owner"
            assert comps["sc:once:f1"]["restricted_to"] == "run_starter"


async def test_redelivered_interaction_resolves_nothing(tmp_path: Path) -> None:
    request = CardRequest(CardKind.CLARIFY, "c9", "Which one?", ("Red", "Blue"))
    rt = FakeRuntime(script=card_script(request))
    async with rig(tmp_path, runtime=rt) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.fake.run_state(triggered.run_id).state == "waiting_input")
        assert await r.fake.click(triggered.run_id, "cl:c9:0") is not None
        assert await r.fake.click(triggered.run_id, "cl:c9:1", author="sam") is None
        await r.until(lambda: r.finished(triggered.run_id))
        await r.fake.redeliver()
        await r.fake.pass_time(0.5)
    assert rt.resolved == [("cl:c9:0", "Red")]
    assert len(r.requests("resumeRun")) == 1


async def test_cancel_interrupts_and_drops_later_callbacks(tmp_path: Path) -> None:
    late: list[Any] = []

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.events.on_status(turn.thread_id, "Working")
        await rt.stop_event(turn.thread_id).wait()
        rt.events.on_status(turn.thread_id, "late status")
        rt.events.on_draft(turn.thread_id, "late draft")
        late.append(await rt.events.on_answer(turn.thread_id, "late answer", None))
        return TurnOutcome.CANCELLED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: rt.turns)
        await r.until(lambda: r.requests("patchRun"))
        patches = len(r.requests("patchRun"))
        await r.fake.cancel(triggered.run_id)
        await r.until(lambda: r.finished(triggered.run_id))
        await r.until(lambda: late)
        await r.fake.pass_time(1)
        assert len(r.requests("patchRun")) == patches
        # A cancel of a run still in the inbox: no turn starts.
        gate = asyncio.Event()
        original = r.handler.prepare

        async def slow(inv: Any, run: Any) -> Any:
            await gate.wait()
            return await original(inv, run)

        r.handler.prepare = slow  # type: ignore[method-assign] # keep the run in the inbox
        queued = await r.fake.trigger("second")
        await r.until(lambda: r.loop._runs.get(queued.run_id) is not None)
        await r.fake.cancel(queued.run_id)
        gate.set()
        await r.fake.pass_time(0.5)
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("cancelled", "user_cancelled")
    assert rt.interrupted == [triggered.thread_id]
    assert late == [None]
    assert r.fake.messages == []
    assert len(rt.turns) == 1
    assert r.fake.run_state(queued.run_id).state == "cancelled"


# ---- sessions ---------------------------------------------------------------------------------


async def test_session_modes(tmp_path: Path) -> None:
    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt) as r:
        fake = r.fake
        # new without agent_session: the turn creates the session, one report with started.
        first = await fake.trigger(session_mode="new")
        await r.until(lambda: r.finished(first.run_id))
        reports = fake.session_reports(first.thread_id)
        assert reports == [{"external_session_ref": "sess-1", "started": True}]
        report_requests = list(r.requests("reportAgentSession"))
        assert "if-match" not in report_requests[-1].headers
        # new with agent_session (a reset): Hermes switches, report with If-Match.
        fake.start_session(first.thread_id, "sess-old", started=False)
        reset = await fake.trigger("reset", thread_id=first.thread_id, session_mode="new")
        await r.until(lambda: r.finished(reset.run_id))
        assert rt.switches[-1] == (first.thread_id, "new", None)
        assert "if-match" in r.requests("reportAgentSession")[-1].headers
        # resume with a different Hermes session: bind to the server's reference.
        rt.known.add("sess-server")
        fake.start_session(first.thread_id, "sess-server")
        resumed = await fake.trigger("go on", thread_id=first.thread_id, session_mode="resume")
        await r.until(lambda: r.finished(resumed.run_id))
        assert rt.switches[-1] == (first.thread_id, "bind", "sess-server")
        # bind with a known session: one report, no bind_failed.
        other = await fake.trigger("import", session_mode="new")
        await r.until(lambda: r.finished(other.run_id))
        rt.known.add("sess-import")
        fake.start_session(other.thread_id, "sess-import", started=False)
        bound = await fake.trigger("bound", thread_id=other.thread_id, session_mode="bind")
        await r.until(lambda: r.finished(bound.run_id))
        last = fake.session_reports(other.thread_id)[-1]
        assert last == {"external_session_ref": "sess-import"}
        # bind with a missing session: fail open with bind_failed.
        third = await fake.trigger("missing", session_mode="new")
        await r.until(lambda: r.finished(third.run_id))
        fake.start_session(third.thread_id, "sess-gone", started=True)
        missing = await fake.trigger("again", thread_id=third.thread_id, session_mode="bind")
        await r.until(lambda: r.finished(missing.run_id))
        last = fake.session_reports(third.thread_id)[-1]
        assert last["bind_failed"] == "session_missing"
        assert last["started"] is True
        # a resume turn that compresses reports the new reference.
        fake.start_session(third.thread_id, rt.sessions[third.thread_id])

        async def compress(turn: TurnRequest, rt_: FakeRuntime) -> TurnOutcome:
            rt_.parents[turn.thread_id] = rt_.sessions[turn.thread_id]
            rt_.sessions[turn.thread_id] = "sess-compressed"
            await rt_.events.on_answer(turn.thread_id, "Short.", None)
            return TurnOutcome.COMPLETED

        rt.script = compress
        compressed = await fake.trigger("long", thread_id=third.thread_id, session_mode="resume")
        await r.until(lambda: r.finished(compressed.run_id))
        assert fake.session_reports(third.thread_id)[-1] == {
            "external_session_ref": "sess-compressed"
        }
        # guest: a fresh session and no report.
        rt.script = FakeRuntime().script
        guest = await fake.trigger("guest")
        rt.sessions[guest.thread_id] = "sess-guest-old"
        before = len(r.requests("reportAgentSession"))
        guest2 = await fake.trigger("guest again", thread_id=guest.thread_id)
        await r.until(lambda: r.finished(guest2.run_id))
        assert len(r.requests("reportAgentSession")) == before
        assert rt.switches[-1] == (guest.thread_id, "new", None)


# ---- heartbeat, SR9 header, logs ---------------------------------------------------------------


async def test_heartbeat_content(tmp_path: Path) -> None:
    from bothy_sdk.heartbeat import CODE_FORM, NAME_FORM, VERSION_FORM

    cases = [
        ("steer", ResolvedCredential("anthropic", "api_key"), []),
        ("steer", ResolvedCredential(None, "unknown"), ["billing_evidence_unknown"]),
        ("interrupt", ResolvedCredential("anthropic", "oauth"), ["busy_mode_invalid"]),
    ]
    for mode, credential, reasons in cases:
        rt = FakeRuntime(credential=credential)
        async with rig(
            tmp_path / f"{mode}{len(reasons)}{credential.key_type}", runtime=rt, busy_mode=mode
        ) as r:
            beat = r.loop.heartbeat().to_dict()
        details = beat["details"]
        assert beat["state"] == ("degraded" if reasons else "online")
        assert details.get("degraded_reasons", []) == reasons
        assert details["adapter"] == {"name": "hermes-adapter", "version": "0.0.0"}
        assert details["runtime"] == {"name": "hermes-agent"}
        assert beat["runtime_version"] == "0.21.5"
        assert beat["reported_tool_class"] == "chat"
        assert details["capabilities"]["components"] is True
        assert details["capabilities"]["title"] is True
        assert ("followups" in details["capabilities"]) == (mode != "interrupt")
        assert "rate_limit" not in beat
        assert "project_readiness" not in beat
        if credential.key_type == "unknown":
            assert "reported_billing_mode" not in beat
            assert "billing_modes_supported" not in beat
            assert beat["billing_evidence"]["key_type"] == "unknown"
        else:
            mode_value = "organization" if credential.key_type == "api_key" else "personal"
            assert beat["reported_billing_mode"] == mode_value
            assert beat["billing_modes_supported"] == [mode_value]
        assert "null" not in str(beat).lower().replace("'", "")
        import re

        assert re.fullmatch(NAME_FORM[1:-1], details["adapter"]["name"])
        assert re.fullmatch(VERSION_FORM[1:-1], beat["runtime_version"])
        assert re.fullmatch(CODE_FORM[1:-1], beat["billing_evidence"]["method"])


async def test_invocation_token_header_inside_runs_only(tmp_path: Path) -> None:
    rt = FakeRuntime()
    files = (ContextFile(uuid7(), "notes.txt", "text/plain", b"hello"),)
    async with rig(tmp_path, runtime=rt) as r:
        r.fake.set_channel("Be brief.")
        first = await r.fake.trigger(session_mode="new", files=files)
        rt.titles[first.thread_id] = "A title"
        await r.until(lambda: r.finished(first.run_id))
        await r.loop.outbox.flush()
    inside = {
        "getRunContext",
        "downloadFile",
        "patchRun",
        "createMessage",
        "reportAgentSession",
        "getThread",
        "setThreadTitle",
        "getServedChannelConfig",
        "finishRun",
    }
    outside = {"getAgentTriggerers", "getJwks", "getDiscovery", "sendHeartbeat"}
    seen = {q.operation_id for q in r.fake.requests}
    assert inside <= seen
    for q in r.fake.requests:
        if q.operation_id in inside:
            assert "invocation-token" in q.headers, q.operation_id
        if q.operation_id in outside:
            assert "invocation-token" not in q.headers, q.operation_id
    for q in r.requests("startRun"):
        assert "invocation-token" not in q.headers


async def test_logs_carry_no_content(tmp_path: Path) -> None:
    canaries = {
        "content": "canary-content-" + "c" * 12,
        "name": "Canaryname",
        "file": "canary-file-name.txt",
        "transcript": "canary-transcript-" + "t" * 8,
        "answer": "canary-answer-" + "a" * 10,
    }

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.events.on_status(turn.thread_id, "canary-status-" + "s" * 8)
        await rt.events.on_answer(turn.thread_id, canaries["answer"], None)
        return TurnOutcome.COMPLETED

    files = (
        ContextFile(uuid7(), canaries["file"], "text/plain", canaries["content"].encode()),
        ContextFile(uuid7(), "v.m4a", "audio/mp4", b"\x00", transcript=canaries["transcript"]),
    )
    rt = FakeRuntime(script=script)
    with captured_logs() as logs:
        async with rig(tmp_path, runtime=rt) as r:
            r.fake.world.sam.display_name = canaries["name"]
            first = await r.fake.trigger(
                canaries["content"],
                context=(ContextMessage("sam", canaries["content"], trust="authorized"),),
                files=files,
            )
            await r.until(lambda: r.finished(first.run_id))
    text = logs.text()
    assert logs.records
    for canary in (*canaries.values(), "canary-status"):
        assert canary not in text


async def test_same_callback_id_after_a_restart_resolves_again(tmp_path: Path) -> None:
    """A new process may reuse Hermes' callback IDs; the claim of an earlier run must not block."""
    from bothy_sdk.testing.fakeserver import FakeServer

    request = CardRequest(CardKind.APPROVAL, "a1", "Run it?", ("once", "deny"), "k")
    async with FakeServer() as fake:
        for _ in range(2):  # the second round is the restarted adapter on the same state
            rt = FakeRuntime(script=card_script(request))
            async with rig(tmp_path, runtime=rt, fake=fake) as r:
                triggered = await fake.trigger()
                await r.until(lambda: fake.run_state(triggered.run_id).state == "waiting_input")
                await fake.click(triggered.run_id, "appr:a1:once")
                await r.until(lambda: r.finished(triggered.run_id))
            assert rt.resolved == [("appr:a1:once", "once")]
            assert fake.run_state(triggered.run_id).state == "completed"


@pytest.mark.parametrize("busy", ["steer", "queue"])
async def test_followup_answers_each_card_kind(tmp_path: Path, busy: str) -> None:
    """A typed answer resolves a clarify with its text; approval and confirmation are refused.

    Only at ``steer`` the text also reaches the running turn; at ``queue`` Hermes would run it as a
    new turn without a run, so it is not passed.
    """
    expected = {
        CardKind.CLARIFY: "b.txt please",
        CardKind.APPROVAL: "deny",
        CardKind.SLASH_CONFIRM: "cancel",
    }
    for kind, answer in expected.items():
        request = CardRequest(
            kind, "c1", "Which?", ("a.txt", "b.txt") if kind is CardKind.CLARIFY else (), "k"
        )
        rt = FakeRuntime(script=card_script(request))
        async with rig(tmp_path / kind.value, runtime=rt, busy_mode=busy) as r:
            triggered = await r.fake.trigger()
            await r.until(lambda: r.fake.run_state(triggered.run_id).state == "waiting_input")
            await r.fake.deliver_followup(triggered.run_id, "b.txt please")
            await r.until(lambda: r.finished(triggered.run_id))
        assert rt.answered == [answer]
        steered = [text for _, text, _ in rt.steered]
        assert steered == (
            ["b.txt please"] if busy == "steer" and kind is not CardKind.CLARIFY else []
        )


async def test_no_answer_after_a_mode_abort(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        await rt.stop_event(turn.thread_id).wait()
        rt.late = ("answered", await rt.events.on_answer(turn.thread_id, "after the switch", None))
        return TurnOutcome.CANCELLED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: rt.turns)
        rt.credential = ResolvedCredential("anthropic", "oauth")
        r.loop.heartbeat()
        await r.until(lambda: r.finished(triggered.run_id))
        await r.until(lambda: rt.late is not None)
    assert rt.late == ("answered", None)
    assert r.fake.messages == []


async def test_discovery_instance_mismatch_stops(tmp_path: Path) -> None:
    from adapter_kit import settings_for
    from bothy_sdk.adapter import AdapterConfig
    from bothy_sdk.testing.fakeserver import FakeServer

    from bothy_hermes_adapter.handler import HermesHandler
    from bothy_hermes_adapter.heartbeat import HermesAdapterLoop
    from bothy_hermes_adapter.store import CallbackStore

    async with FakeServer() as fake:
        await fake.trigger()
        store = CallbackStore(tmp_path / "hermes.sqlite3")
        handler = HermesHandler(
            FakeRuntime(), settings_for(fake, tmp_path), store, busy_mode="steer"
        )
        config = AdapterConfig(
            base_url=fake.base_url,
            instance_id=uuid7(),  # another instance than the server's
            agent_id=fake.agent_id,
            credentials=fake.credentials,
            consumer="mismatch",
            allowlist=True,
            settings=fake.sdk_settings,
        )
        reason = await HermesAdapterLoop(config, handler, clock=fake.clock).run()
        await store.close()
    assert reason.reason == "instance_mismatch"
    assert not [f for f in fake.frames if f.get("op") in ("identify", "ack")]
    assert [r.operation_id for r in fake.requests] == ["getDiscovery"]


@pytest.mark.parametrize("answer", [True, False])
async def test_followup_after_the_final_reply_is_not_steered(tmp_path: Path, answer: bool) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        # A model reply without tool calls: Hermes drains its last steer after it, sent or not.
        rt.events.on_model_reply(turn.thread_id, 0)
        if answer:
            await rt.events.on_answer(turn.thread_id, "Done.", None)
        replied.set()
        await rt.stop_event(turn.thread_id).wait()
        return TurnOutcome.COMPLETED

    replied = asyncio.Event()
    handled = asyncio.Event()
    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, busy_mode="steer") as r:
        original = r.handler.on_followup

        async def on_followup(inv: Any, run: Any) -> None:
            await original(inv, run)
            handled.set()

        r.handler.on_followup = on_followup  # type: ignore[method-assign] # wait for its end
        triggered = await r.fake.trigger()
        await r.until(replied.is_set, "the final reply")
        await r.fake.deliver_followup(triggered.run_id, "one more thing")
        # The turn still runs (the script waits for the stop), so only the gate refuses.
        await r.until(handled.is_set, "the follow-up to be handled")
        rt.stop_event(triggered.thread_id).set()
        await r.until(lambda: r.finished(triggered.run_id))
    assert rt.steered == []
    states = [
        op["path"]
        for q in r.requests("patchRun")
        for op in q.json["ops"]
        if op["path"].startswith("/followups/")
    ]
    assert states == []


async def test_followup_after_interim_commentary_is_steered(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        # Hermes sends the text of a reply with a tool call the same way as its answer.
        rt.events.on_model_reply(turn.thread_id, 1)
        await rt.events.on_answer(turn.thread_id, "Let me check.", None)
        await rt.stop_event(turn.thread_id).wait()
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, busy_mode="steer") as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.fake.messages)
        await r.fake.deliver_followup(triggered.run_id, "one more thing")
        await r.until(lambda: rt.steered, "the follow-up to be steered")
        rt.stop_event(triggered.thread_id).set()
        await r.until(lambda: r.finished(triggered.run_id))
    assert [text for _, text, _ in rt.steered] == ["one more thing"]
