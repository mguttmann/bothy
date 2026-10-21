# SPDX-License-Identifier: Apache-2.0
"""FakeServer routes and world parts the runner scenarios need (M2-34, 09 16.1).

Forums with a project binding, ``agent.config_changed``, ``getMe``, ``downloadFile``,
``getThread``, ``setThreadTitle``, ``reportAgentSession``, ``startRun`` with
``external_session_ref``, ``finishRun`` with ``costs`` behind a switch, triggers with
attachments, an agent session or another trigger kind, and a context per run. Every answer is
parsed with the generated core of docs/api/openapi.yaml.
"""

from __future__ import annotations

import base64
import json
from collections.abc import AsyncIterator
from typing import Any
from uuid import UUID

import httpx
import pytest

from bothy_sdk.auth import DpopAuth
from bothy_sdk.generated import models
from bothy_sdk.testing.fakeserver import FakeServer

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    """The FakeServer runs on asyncio."""
    return "asyncio"


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer(costs=True) as server:
        async with _api(server) as api:
            await _events(api)  # the first poll creates the cursor at the head
        yield server


def _api(fake: FakeServer) -> httpx.AsyncClient:
    return httpx.AsyncClient(
        base_url=fake.base_url, auth=DpopAuth(fake.credentials, clock=fake.clock)
    )


def _claims(token: str) -> dict[str, Any]:
    segment = token.split(".")[1]
    decoded: dict[str, Any] = json.loads(
        base64.urlsafe_b64decode(segment + "=" * (-len(segment) % 4))
    )
    return decoded


async def _events(api: httpx.AsyncClient, consumer: str = "rest") -> list[dict[str, Any]]:
    r = await api.get(f"/agents/me/consumers/{consumer}/events")
    assert r.status_code == 200, r.text
    events: list[dict[str, Any]] = r.json()["events"]
    return events


async def _invocation(api: httpx.AsyncClient, run_id: UUID) -> dict[str, Any]:
    for event in await _events(api):
        if event["type"] == "agent.invocation" and event["payload"]["run_id"] == str(run_id):
            models.AgentInvocationEvent.from_dict(
                {k: v for k, v in event.items() if k != "invocation_token"}
            )
            return event
    raise AssertionError("no invocation")


def _start(token: str, **extra: Any) -> dict[str, Any]:
    return {
        "invocation_token": token,
        "billing_mode_used": "organization",
        "tool_class_used": "scoped",
        **extra,
    }


async def test_forums_with_binding_and_config_changed(fake: FakeServer) -> None:
    forum = fake.add_forum("Projekt-A", "website", model="model-small", allowed_tools=["Read"])
    async with _api(fake) as api:
        listed = models.AgentChannelConfigList.from_dict(
            (await api.get("/agents/me/channels")).json()
        )
        by_id = {c.channel_id: c for c in listed.items}
        assert fake.world.channel_id in by_id
        binding = by_id[forum].behavior.project_binding
        assert isinstance(binding, models.ProjectBinding)
        assert binding.workspace_ref == "website"
        assert binding.runner_config.to_dict() == {
            "model": "model-small",
            "allowed_tools": ["Read"],
        }
        one = await api.get(f"/agents/me/channels/{forum}/config")
        assert one.headers["ETag"] == '"1"'
        fake.update_forum(forum, workspace_ref="website-2")
        events = [e for e in await _events(api) if e["type"] == "agent.config_changed"]
        assert events[-1]["payload"] == {"channel_id": str(forum), "settings_version": 2}
        models.AgentConfigChangedEvent.from_dict(events[-1])
        again = models.AgentChannelConfig.from_dict(
            (await api.get(f"/agents/me/channels/{forum}/config")).json()
        )
        assert again.behavior.settings_version == 2
        fake.remove_channel(forum)
        assert (await api.get(f"/agents/me/channels/{forum}/config")).status_code == 404


async def test_get_me_is_the_agent(fake: FakeServer) -> None:
    async with _api(fake) as api:
        me = models.Me.from_dict((await api.get("/me")).json())
    assert me.principal.id == fake.agent_id
    assert str(me.principal.kind) == "agent"


async def test_trigger_with_attachments_context_and_files(fake: FakeServer) -> None:
    forum = fake.add_forum("Projekt-A", "website")
    clean = fake.add_file(b"file canary", name="notes.txt", mime="text/plain")
    pending = fake.add_file(b"x", name="later.txt", mime="text/plain", status="pending")
    blocked = fake.add_file(b"x", name="bad.exe", mime="application/octet-stream", status="blocked")
    run = await fake.trigger("Please look", channel_id=forum, attachments=[clean, pending, blocked])
    async with _api(fake) as api:
        event = await _invocation(api, run.run_id)
        assert event["payload"]["channel_id"] == str(forum)
        claims = _claims(event["invocation_token"])
        assert claims["channel_id"] == str(forum)
        headers = {"Invocation-Token": event["invocation_token"]}
        ctx = models.RunContext.from_dict(
            (await api.get(f"/runs/{run.run_id}/context", headers=headers)).json()
        )
        statuses = {a.file_id: a.scan_status for a in ctx.trigger_message.attachments}
        assert statuses == {clean: "clean", pending: "pending", blocked: "blocked"}
        got = await api.get(f"/files/{clean}/content", headers=headers)
        assert (got.status_code, got.content) == (200, b"file canary")
        for file_id, code in ((pending, "file_not_ready"), (blocked, "file_blocked")):
            r = await api.get(f"/files/{file_id}/content", headers=headers)
            assert (r.status_code, r.json()["code"]) == (409, code)
        assert (await api.get(f"/files/{UUID(int=5)}/content")).status_code == 404
        run_doc = models.Run.from_dict((await api.get(f"/runs/{run.run_id}")).json())
        assert run_doc.channel_id == forum


async def test_context_override_per_run(fake: FakeServer) -> None:
    run = await fake.trigger()
    doc = fake.world.context(str(run.run_id))
    doc["thread_context"] = []
    doc["context_floor_seq"] = 7
    fake.set_context(run.run_id, doc)
    async with _api(fake) as api:
        event = await _invocation(api, run.run_id)
        r = await api.get(
            f"/runs/{run.run_id}/context", headers={"Invocation-Token": event["invocation_token"]}
        )
    ctx = models.RunContext.from_dict(r.json())
    assert (ctx.run_id, ctx.context_floor_seq, ctx.thread_context) == (run.run_id, 7, [])


async def test_session_ref_at_start_then_report(fake: FakeServer) -> None:
    run = await fake.trigger()
    ref = "0192d3a0-0000-4000-8000-0000000000aa"
    async with _api(fake) as api:
        event = await _invocation(api, run.run_id)
        token = event["invocation_token"]
        started = await api.post(
            f"/runs/{run.run_id}/start", json=_start(token, external_session_ref=ref, model="m")
        )
        assert started.status_code == 200, started.text
        bound = {"Invocation-Token": token}
        url = f"/threads/{run.thread_id}/agent-sessions/report"
        stale = await api.post(
            url,
            json={"external_session_ref": ref, "started": True},
            headers={**bound, "If-Match": '"9"'},
        )
        assert stale.status_code == 412
        current = models.AgentSession.from_dict(stale.json()["current"])
        assert (current.version, current.started) == (1, False)
        ok = await api.post(
            url,
            json={"external_session_ref": ref, "model": "m", "started": True},
            headers={**bound, "If-Match": '"1"'},
        )
        assert ok.status_code == 200, ok.text
        session = models.AgentSession.from_dict(ok.json())
        assert (session.version, session.started, session.external_session_ref) == (2, True, ref)
        assert ok.headers["ETag"] == '"2"'
    assert fake.sessions(run.thread_id)[0]["started"] is True


async def test_invocation_carries_agent_session(fake: FakeServer) -> None:
    ref = "0192d3a0-0000-4000-8000-0000000000bb"
    first = await fake.trigger()
    fake.world.session_for(first.thread_id, ref, started=True)
    second = await fake.trigger("Next", thread_id=first.thread_id)
    async with _api(fake) as api:
        event = await _invocation(api, second.run_id)
    session = models.AgentSession.from_dict(event["payload"]["agent_session"])
    assert (session.external_session_ref, session.started) == (ref, True)


async def test_thread_title_rules(fake: FakeServer) -> None:
    run = await fake.trigger()
    async with _api(fake) as api:
        event = await _invocation(api, run.run_id)
        bound = {"Invocation-Token": event["invocation_token"]}
        thread = models.Thread.from_dict(
            (await api.get(f"/threads/{run.thread_id}", headers=bound)).json()
        )
        assert str(thread.title_source) == "auto"
        url = f"/threads/{run.thread_id}/title"
        assert (await api.put(url, json={"title": "A"}, headers=bound)).status_code == 428
        wrong = await api.put(url, json={"title": "A"}, headers={**bound, "If-Match": '"99"'})
        assert wrong.status_code == 412
        ok = await api.put(
            url, json={"title": "A"}, headers={**bound, "If-Match": f'"{thread.version}"'}
        )
        assert ok.status_code == 200, ok.text
        assert models.Thread.from_dict(ok.json()).title == "A"
        fake.set_title_source(run.thread_id, "manual")
        manual = await api.put(
            url, json={"title": "B"}, headers={**bound, "If-Match": f'"{thread.version + 1}"'}
        )
        assert (manual.status_code, manual.json()["code"]) == (409, "title_manual")
    assert fake.thread(run.thread_id)["title"] == "A"


async def test_costs_behind_the_switch(fake: FakeServer) -> None:
    run = await fake.trigger()
    async with _api(fake) as api:
        token = (await _invocation(api, run.run_id))["invocation_token"]
        assert (await api.post(f"/runs/{run.run_id}/start", json=_start(token))).status_code == 200
        cost = {"provider": "anthropic", "model": "m", "currency": "USD", "amount_micros": 5}
        done = await api.post(
            f"/runs/{run.run_id}/finish",
            json={"state": "completed", "costs": [cost]},
            headers={"Invocation-Token": token},
        )
        assert done.status_code == 200, done.text
    assert fake.world.runs[run.run_id].costs == [cost]


async def test_trigger_kind_interaction_claims_the_interaction(fake: FakeServer) -> None:
    run = await fake.trigger("Approve", trigger_kind="interaction")
    async with _api(fake) as api:
        event = await _invocation(api, run.run_id)
    payload = event["payload"]
    assert payload["trigger_kind"] == "interaction"
    assert "message_id" not in payload
    assert _claims(event["invocation_token"])["interaction_id"] == payload["interaction_id"]


async def test_context_given_with_the_trigger(fake: FakeServer) -> None:
    voice = fake.add_file(b"ogg", name="memo.ogg", mime="audio/ogg", transcript="spoken words")
    earlier = fake.world.context(str((await fake.trigger("before")).run_id))["trigger_message"]
    earlier = {**earlier, "trust": "unverified"}
    run = await fake.trigger("now", attachments=[voice], context={"thread_context": [earlier]})
    async with _api(fake) as api:
        event = await _invocation(api, run.run_id)
        r = await api.get(
            f"/runs/{run.run_id}/context", headers={"Invocation-Token": event["invocation_token"]}
        )
    ctx = models.RunContext.from_dict(r.json())
    assert ctx.trigger_message.content_md == "now"
    assert ctx.trigger_message.attachments[0].transcript == "spoken words"
    assert [m.content_md for m in ctx.thread_context] == ["before"]
