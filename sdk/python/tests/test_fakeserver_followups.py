# SPDX-License-Identifier: Apache-2.0
"""FakeServer parts the runner's follow-up, stop and session scenarios need (M2-35, 09 16.1).

Follow-up rows and their patches, follow-ups of another author or with replaced claims,
``getMessage`` (deleted, empty, unknown), a run state set by the test, ``finishRun`` of a run
the server already cancelled, and ``reportAgentSession`` with a reference change and with
``bind_failed``. Every answer is parsed with the generated core of docs/api/openapi.yaml.
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
    async with FakeServer(dpop=False) as server:
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


async def _events(api: httpx.AsyncClient) -> list[dict[str, Any]]:
    r = await api.get("/agents/me/consumers/rest/events")
    assert r.status_code == 200, r.text
    events: list[dict[str, Any]] = r.json()["events"]
    return events


async def _started(fake: FakeServer, api: httpx.AsyncClient) -> tuple[UUID, UUID, str]:
    run = await fake.trigger()
    [event] = [e for e in await _events(api) if e["type"] == "agent.invocation"]
    token = event["invocation_token"]
    body = {
        "invocation_token": token,
        "billing_mode_used": "organization",
        "tool_class_used": "chat",
    }
    started = await api.post(f"/runs/{run.run_id}/start", json=body)
    assert started.status_code == 200, started.text
    return run.run_id, run.thread_id, token


async def test_followup_rows_and_patches(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, _, token = await _started(fake, api)
        message_id = await fake.deliver_followup(run_id, "Also the footer", author="sam")
        [event] = [e for e in await _events(api) if e["type"] == "agent.followup"]
        models.AgentFollowupEvent.from_dict(
            {k: v for k, v in event.items() if k != "invocation_token"}
        )
        claims = _claims(event["invocation_token"])
        assert (claims["sub"], claims["trigger_kind"]) == (str(fake.world.sam.id), "followup")
        assert fake.followup_state(run_id, message_id) == "queued"
        bound = {"Invocation-Token": token}
        run = models.Run.from_dict((await api.get(f"/runs/{run_id}", headers=bound)).json())
        assert run.followups[str(message_id)].state == "queued"

        async def patch(target: str, state: str) -> httpx.Response:
            version = (await api.get(f"/runs/{run_id}", headers=bound)).json()["version"]
            op = {"op": "replace", "path": f"/followups/{target}/state", "value": state}
            return await api.patch(
                f"/runs/{run_id}", json={"base_version": version, "ops": [op]}, headers=bound
            )

        assert (await patch(str(message_id), "delivered")).status_code == 200
        assert (await patch(str(message_id), "absorbed")).status_code == 200
        assert fake.followup_state(run_id, message_id) == "absorbed"
        backward = await patch(str(message_id), "delivered")
        assert (backward.status_code, backward.json()["code"]) == (400, "validation_failed")
        unknown = await patch(str(fake.world.alex.id), "delivered")
        assert (unknown.status_code, unknown.json()["code"]) == (400, "validation_failed")


async def test_followup_claims_replaced_but_signed(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, _, _ = await _started(fake, api)
        await fake.deliver_followup(run_id, claims={"billing_mode": "personal"})
        [event] = [e for e in await _events(api) if e["type"] == "agent.followup"]
    claims = _claims(event["invocation_token"])
    assert (claims["billing_mode"], claims["sub"]) == ("personal", str(fake.world.alex.id))
    checked = fake.world.issuer.check(
        event["invocation_token"],
        aud=fake.world.lotse.id,
        now=fake.world.now(),
        leeway=0,
        ignore_expiry=True,
    )
    assert checked.verdict == "valid"


async def test_get_message_deleted_empty_and_unknown(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, _, token = await _started(fake, api)
        bound = {"Invocation-Token": token}
        message_id = await fake.deliver_followup(run_id, "Check the header")
        ok = await api.get(f"/messages/{message_id}", headers=bound)
        assert ok.status_code == 200, ok.text
        message = models.Message.from_dict(ok.json())
        assert (message.content_md, message.author.id) == ("Check the header", fake.world.alex.id)
        fake.edit_message(message_id, content_md="", deleted=True)
        changed = models.Message.from_dict(
            (await api.get(f"/messages/{message_id}", headers=bound)).json()
        )
        assert changed.content_md == ""
        assert changed.deleted_at is not None
        fake.hide_message(message_id)
        gone = await api.get(f"/messages/{message_id}", headers=bound)
        assert (gone.status_code, gone.json()["code"]) == (404, "not_found")


async def test_set_run_state_and_finish_of_a_cancelled_run(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, _, token = await _started(fake, api)
        fake.set_run_state(run_id, "failed", "stale")
        assert (fake.run_state(run_id).state, fake.run_state(run_id).state_reason) == (
            "failed",
            "stale",
        )
        queued = await fake.trigger("Second")
        [event] = [
            e
            for e in await _events(api)
            if e["type"] == "agent.invocation" and e["payload"]["run_id"] == str(queued.run_id)
        ]
        await fake.cancel(queued.run_id, "access_revoked")
        again = await api.post(
            f"/runs/{queued.run_id}/finish",
            json={"state": "cancelled", "reason": "access_revoked"},
            headers={"Invocation-Token": event["invocation_token"]},
        )
    assert again.status_code == 200, again.text
    assert models.Run.from_dict(again.json()).state_reason == "access_revoked"
    del token


async def test_report_reference_change_and_bind_failed(fake: FakeServer) -> None:
    old = "0192d3a0-0000-4000-8000-0000000000c1"
    new = "0192d3a0-0000-4000-8000-0000000000c2"
    third = "0192d3a0-0000-4000-8000-0000000000c3"
    first = await fake.trigger()
    fake.world.session_for(first.thread_id, old, started=True)
    async with _api(fake) as api:
        [event] = [e for e in await _events(api) if e["type"] == "agent.invocation"]
        body = {
            "invocation_token": event["invocation_token"],
            "billing_mode_used": "organization",
            "tool_class_used": "chat",
        }
        assert (await api.post(f"/runs/{first.run_id}/start", json=body)).status_code == 200
        bound = {"Invocation-Token": event["invocation_token"]}
        url = f"/threads/{first.thread_id}/agent-sessions/report"
        changed = await api.post(
            url,
            json={"external_session_ref": new, "started": True},
            headers={**bound, "If-Match": '"1"'},
        )
        assert changed.status_code == 200, changed.text
        row = models.AgentSession.from_dict(changed.json())
        assert (row.external_session_ref, row.version) == (new, 2)
        rebind = await api.post(
            url,
            json={"external_session_ref": third, "started": True, "bind_failed": "session_missing"},
            headers={**bound, "If-Match": '"2"'},
        )
        assert rebind.status_code == 200, rebind.text
        assert models.AgentSession.from_dict(rebind.json()).external_session_ref == third
    rows = fake.sessions(first.thread_id)
    assert [(r["external_session_ref"], r["is_current"]) for r in rows] == [
        (new, False),
        (third, True),
    ]
    assert rows[0]["end_reason"] == "rebind"
    reports = [r for r in fake.requests if r.operation_id == "reportAgentSession"]
    assert [r.headers.get("if-match") for r in reports] == ['"1"', '"2"']
    assert reports[-1].json["bind_failed"] == "session_missing"
