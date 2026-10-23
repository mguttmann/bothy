# SPDX-License-Identifier: Apache-2.0
"""The FakeServer plays the server rules the SDK meets (docs/spec/09-agents.md 16.1).

Written with plain ``httpx`` and ``websockets`` (not with the SDK's client), so the server is
proven on its own. Tokens are compared by ``jti`` or `fingerprint`, never printed.
"""

from __future__ import annotations

import asyncio
import base64
import json
from collections.abc import AsyncIterator, Callable
from typing import Any
from uuid import UUID

import httpx
import jwt
import pytest
import websockets

from bothy_sdk.auth import DpopAuth, Secret, dpop_proof
from bothy_sdk.generated import models
from bothy_sdk.testing import fingerprint
from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.fakeserver import FakeServer, RunView
from bothy_sdk.testing.faults import Fault
from bothy_sdk.tolerance import decode_event

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    """The FakeServer runs on asyncio."""
    return "asyncio"


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    """A started FakeServer with DPoP."""
    async with FakeServer() as server:
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


def _start_body(token: str, **extra: Any) -> dict[str, Any]:
    body = {
        "invocation_token": token,
        "billing_mode_used": "organization",
        "tool_class_used": "scoped",
    }
    body.update(extra)
    return body


async def _poll(api: httpx.AsyncClient, consumer: str = "rest") -> dict[str, Any]:
    r = await api.get(f"/agents/me/consumers/{consumer}/events")
    assert r.status_code == 200, r.text
    page: dict[str, Any] = r.json()
    models.EventPage.from_dict(page)
    return page


async def _queued_with_token(fake: FakeServer, api: httpx.AsyncClient) -> tuple[UUID, str]:
    """Trigger a run and fetch its agent.invocation over REST; returns run ID and token."""
    await _poll(api)  # the first poll creates the cursor at the head
    triggered = await fake.trigger()
    page = await _poll(api)
    [event] = [e for e in page["events"] if e["type"] == "agent.invocation"]
    await api.post("/agents/me/consumers/rest/ack", json={"seq": page["next_cursor"]})
    return triggered.run_id, event["invocation_token"]


class _Gateway:
    """A minimal gateway client for the tests."""

    def __init__(self, ws: Any, fake: FakeServer) -> None:
        self.ws = ws
        self.fake = fake
        self.pending: list[dict[str, Any]] = []
        self.session_id = ""

    async def recv(self) -> dict[str, Any]:
        frame: dict[str, Any] = json.loads(await within_patience(self.ws.recv()))
        return frame

    async def send(self, op: str, d: dict[str, Any]) -> None:
        await self.ws.send(json.dumps({"op": op, "d": d}))

    async def until(self, op: str) -> dict[str, Any]:
        for frame in self.pending:
            if frame["op"] == op:
                self.pending.remove(frame)
                return frame
        while True:
            frame = await self.recv()
            if frame["op"] == op:
                return frame


async def _open(fake: FakeServer) -> tuple[_Gateway, str]:
    ws = await websockets.connect(fake.gateway_url)
    gw = _Gateway(ws, fake)
    hello = await gw.recv()
    assert hello["op"] == "hello"
    models.GatewayHello.from_dict(hello["d"])
    nonce: str = hello["d"]["dpop_nonce"]
    return gw, nonce


def _credentials(fake: FakeServer, nonce: str) -> dict[str, Any]:
    token = fake.credentials.api_token
    key = fake.credentials.dpop
    assert key is not None
    proof = dpop_proof(
        key, htm="GET", htu=fake.gateway_htu, token=token, nonce=nonce, clock=fake.clock
    )
    return {"api_token": token.reveal(), "dpop": proof}


async def _connect(fake: FakeServer, consumer: str = "gw") -> _Gateway:
    gw, nonce = await _open(fake)
    await gw.send(
        "identify", {"protocol_version": 1, "consumer": consumer, **_credentials(fake, nonce)}
    )
    ready = await gw.recv()
    assert ready["op"] == "ready", ready
    models.GatewayReady.from_dict(ready["d"])
    assert ready["d"]["principal"]["id"] == str(fake.agent_id)
    gw.session_id = ready["d"]["gateway_session_id"]
    while (frame := await gw.recv())["op"] != "catchup.complete":
        gw.pending.append(frame)
    return gw


async def _resume(fake: FakeServer, session_id: str) -> _Gateway:
    gw, nonce = await _open(fake)
    await gw.send(
        "resume", {"gateway_session_id": session_id, "cursor": 0, **_credentials(fake, nonce)}
    )
    return gw


async def _close_code(gw: _Gateway) -> int:
    with pytest.raises(websockets.ConnectionClosed) as closed:
        await gw.until("never")
    assert closed.value.rcvd is not None
    return closed.value.rcvd.code


async def _eventually(check: Callable[[], bool]) -> None:
    await eventually(check, "the condition")


# Discovery, JWKS and tokens -----------------------------------------------------------------


async def test_discovery_and_jwks(fake: FakeServer) -> None:
    async with httpx.AsyncClient(base_url=fake.base_url) as anon:
        disco = (await anon.get("/instance/discovery")).json()
        jwks = await anon.get("/instance/jwks")
    parsed = models.Discovery.from_dict(disco)
    assert parsed.instance_id == fake.instance_id
    assert disco["gateway"]["url"] == "/api/v1/gateway"
    assert jwks.headers["cache-control"] == "public, max-age=300"
    [key] = jwks.json()["keys"]
    assert (key["kty"], key["crv"], key["alg"], key["use"]) == ("OKP", "Ed25519", "EdDSA", "sig")


async def test_token_claims_verify_with_jwks(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        jwks = (await api.get("/instance/jwks")).json()
    header = jwt.get_unverified_header(token)
    assert (header["alg"], header["typ"]) == ("EdDSA", "agent-invocation+jwt")
    [jwk] = [k for k in jwks["keys"] if k["kid"] == header["kid"]]
    claims = jwt.decode(
        token,
        key=jwt.PyJWK(jwk).key,
        algorithms=["EdDSA"],
        audience=str(fake.agent_id),
        issuer=str(fake.instance_id),
        options={"verify_exp": False, "verify_iat": False},
    )
    assert claims["run_id"] == str(run_id)
    assert claims["trigger_kind"] == "message"
    assert claims["level"] == "trigger"
    assert (claims["billing_mode"], claims["tool_class"]) == ("organization", "scoped")
    assert claims["exp"] - claims["iat"] == fake.timings["agent.invocation_token_ttl"]
    assert claims["iat"] == int(fake.clock.now().timestamp())
    assert UUID(claims["jti"]).version == 7
    for name in ("sub", "thread_id", "channel_id", "space_id", "message_id"):
        assert UUID(claims[name])
    assert isinstance(claims["permission_version"], int)


# DPoP ---------------------------------------------------------------------------------------


async def test_dpop_nonce_enforced(fake: FakeServer) -> None:
    key = fake.credentials.dpop
    assert key is not None
    token = fake.credentials.api_token
    url = f"{fake.base_url}/runs/{UUID(int=0)}"

    def headers(nonce: str | None, proof: str | None = None) -> dict[str, str]:
        p = proof or dpop_proof(key, htm="GET", htu=url, token=token, nonce=nonce, clock=fake.clock)
        return {"Authorization": f"DPoP {token.reveal()}", "DPoP": p}

    async with httpx.AsyncClient() as raw:
        first = await raw.get(url, headers=headers(None))
        assert first.status_code == 401
        assert first.json()["code"] == "use_dpop_nonce"
        nonce = first.headers["DPoP-Nonce"]
        proof = dpop_proof(key, htm="GET", htu=url, token=token, nonce=nonce, clock=fake.clock)
        ok = await raw.get(url, headers=headers(nonce, proof))
        assert ok.status_code == 404  # authenticated; the run does not exist
        assert ok.headers["DPoP-Nonce"]
        replay = await raw.get(url, headers=headers(nonce, proof))
        assert replay.status_code == 401
        assert replay.json()["code"] == "invalid_dpop_proof"
        bearer = await raw.get(url, headers={"Authorization": f"Bearer {token.reveal()}"})
        assert bearer.json()["code"] == "invalid_dpop_proof"
        # The nonce rotates: after two of its windows the old one is refused.
        await fake.pass_time(2 * fake.timings["auth.dpop_nonce_ttl"])
        stale = await raw.get(url, headers=headers(nonce))
        assert stale.json()["code"] == "use_dpop_nonce"
        assert stale.headers["DPoP-Nonce"] != nonce
        wrong_htu = dpop_proof(
            key,
            htm="GET",
            htu=fake.base_url + "/other",
            token=token,
            nonce=stale.headers["DPoP-Nonce"],
            clock=fake.clock,
        )
        bad = await raw.get(url, headers=headers(None, wrong_htu))
        assert bad.json()["code"] == "invalid_dpop_proof"
    async with _api(fake) as api:
        assert (await api.get(f"/runs/{UUID(int=0)}")).status_code == 404


async def test_bearer_mode_without_dpop() -> None:
    async with FakeServer(dpop=False) as fake:
        assert fake.credentials.dpop is None
        async with _api(fake) as api:
            r = await api.get("/agents/me/channels")
            assert r.status_code == 200
            models.AgentChannelConfigList.from_dict(r.json())
        async with httpx.AsyncClient(base_url=fake.base_url) as anon:
            assert (await anon.get("/agents/me/channels")).status_code == 401


# Delivery -----------------------------------------------------------------------------------


async def test_trigger_delivers_over_gateway_and_polling(fake: FakeServer) -> None:
    gw = await _connect(fake)
    async with _api(fake) as api:
        await _poll(api)
        triggered = await fake.trigger()
        frame = await gw.until("dispatch")
        page = await _poll(api)
    envelope = frame["d"]
    event = decode_event({k: v for k, v in envelope.items() if k != "invocation_token"})
    assert isinstance(event, models.AgentInvocationEvent)
    assert event.payload.run_id == triggered.run_id
    assert event.payload.message_id == triggered.message_id
    assert "redelivered" not in envelope
    [polled] = page["events"]
    assert polled["id"] == envelope["id"]
    # A fresh token per delivery: two consumers, two tokens; the newest is current.
    assert (
        _claims(polled["invocation_token"])["jti"] != _claims(envelope["invocation_token"])["jti"]
    )
    await gw.send("ack", {"seq": envelope["seq"]})
    await _eventually(lambda: fake.consumer_state("gw") == (envelope["seq"], envelope["seq"]))
    await gw.ws.close()


async def test_redelivery_after_ack_timeout(fake: FakeServer) -> None:
    gw = await _connect(fake)
    async with _api(fake) as api:
        await _poll(api)
        run = await fake.trigger()
        first = (await gw.until("dispatch"))["d"]
        polled = (await _poll(api))["events"][0]
        assert (await _poll(api))["events"] == []  # nothing repeats within the ack timeout
        await fake.pass_time(fake.timings["agent.ack_timeout"])
        again = (await gw.until("dispatch"))["d"]
        repolled = (await _poll(api))["events"][0]
        assert again["id"] == first["id"]
        assert again["redelivered"] is True
        assert repolled["redelivered"] is True
        jtis = {_claims(t["invocation_token"])["jti"] for t in (first, polled, again, repolled)}
        assert len(jtis) == 4
        # The newest delivery's token is the run's current one; an older one is superseded.
        old = await api.post(
            f"/runs/{run.run_id}/start", json=_start_body(first["invocation_token"])
        )
        assert old.status_code == 403
        assert old.json()["reason"] == "superseded"
        new = await api.post(
            f"/runs/{run.run_id}/start", json=_start_body(repolled["invocation_token"])
        )
        assert new.status_code == 200, new.text
        models.Run.from_dict(new.json())
    await gw.ws.close()


async def test_same_consumer_newest_wins_and_restart(fake: FakeServer) -> None:
    old = await _connect(fake, "alpha")
    new = await _connect(fake, "alpha")
    with pytest.raises(websockets.ConnectionClosed) as closed:
        await old.until("never")
    assert closed.value.rcvd is not None
    assert closed.value.rcvd.code == 4015
    run = await fake.trigger()
    frame = (await new.until("dispatch"))["d"]
    await new.send("ack", {"seq": frame["seq"]})
    await _eventually(lambda: fake.consumer_state("alpha")[0] == frame["seq"])
    await fake.restart("alpha", acked_seq=frame["seq"] - 1)
    with pytest.raises(websockets.ConnectionClosed) as restarted:
        await new.until("never")
    assert restarted.value.rcvd is not None
    assert restarted.value.rcvd.code == 4013
    again = await _connect(fake, "alpha")
    redelivered = (await again.until("dispatch"))["d"]
    assert redelivered["id"] == frame["id"]
    assert _claims(redelivered["invocation_token"])["run_id"] == str(run.run_id)
    assert fake.run_state(run.run_id).state == "queued"
    await again.ws.close()


# Run rules ----------------------------------------------------------------------------------


async def test_start_rules(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        for variant in ("bad_signature", "alg_hs256", "typ_access", "aud_other_agent"):
            forged = fake.forge_token(run_id, variant)
            for path in ("start", "invocation-token"):
                body = (
                    _start_body(forged.reveal())
                    if path == "start"
                    else {"invocation_token": forged.reveal()}
                )
                r = await api.post(f"/runs/{run_id}/{path}", json=body)
                assert r.status_code == 403, (variant, path)
                assert r.json()["code"] == "invocation_token_invalid"
                assert r.json()["reason"] == "invalid"
        empty = await api.post(f"/runs/{run_id}/start", json=_start_body(""))
        assert empty.json()["reason"] == "invalid"
        assert fake.run_state(run_id).state == "queued"
        # Expired after agent.invocation_token_ttl plus auth.clock_leeway (strictly after).
        await fake.pass_time(
            fake.timings["agent.invocation_token_ttl"] + fake.timings["auth.clock_leeway"] + 1
        )
        expired = await api.post(f"/runs/{run_id}/start", json=_start_body(token))
        assert expired.json()["reason"] == "expired"
        grant = await api.post(f"/runs/{run_id}/invocation-token", json={"invocation_token": token})
        assert grant.status_code == 200, grant.text
        models.InvocationTokenGrant.from_dict(grant.json())
        renewed = grant.json()["invocation_token"]
        # The renewed-from token may renew again (a lost answer), the older grant is replaced.
        retry = await api.post(f"/runs/{run_id}/invocation-token", json={"invocation_token": token})
        renewed2 = retry.json()["invocation_token"]
        stale = await api.post(f"/runs/{run_id}/start", json=_start_body(renewed))
        assert stale.json()["reason"] == "superseded"
        started = await api.post(f"/runs/{run_id}/start", json=_start_body(renewed2))
        assert started.status_code == 200
        assert started.json()["state"] == "running"
        replay = await api.post(f"/runs/{run_id}/start", json=_start_body(renewed2))
        assert replay.status_code == 200
        assert replay.headers["Idempotent-Replay"] == "true"
        again = await api.post(f"/runs/{run_id}/start", json=_start_body(token))
        assert again.status_code == 409
        assert again.json()["code"] == "already_started"
        renew_running = await api.post(
            f"/runs/{run_id}/invocation-token", json={"invocation_token": token}
        )
        assert renew_running.json()["code"] == "already_started"
    view = fake.run_state(run_id)
    assert (view.state, view.starts) == ("running", 1)


@pytest.mark.parametrize(
    ("extra", "code"),
    [
        ({"billing_mode_used": "personal"}, "billing_mode_mismatch"),
        ({"billing_evidence": {"key_type": "unknown"}}, "billing_mode_mismatch"),
        ({"tool_class_used": "host"}, "tool_class_mismatch"),
    ],
)
async def test_mismatch_fails_the_run(fake: FakeServer, extra: dict[str, Any], code: str) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        r = await api.post(f"/runs/{run_id}/start", json=_start_body(token, **extra))
    assert r.status_code == 409
    assert (r.json()["code"], r.json()["reason"]) == (code, code)
    view = fake.run_state(run_id)
    assert (view.state, view.state_reason, view.starts) == ("failed", code, 0)


async def test_lower_tool_class_passes(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        r = await api.post(f"/runs/{run_id}/start", json=_start_body(token, tool_class_used="chat"))
    assert r.status_code == 200


async def test_finish_await_resume_and_followup(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        await api.post(f"/runs/{run_id}/start", json=_start_body(token))
        waiting = await api.post(f"/runs/{run_id}/await-input", json={"status_line": "Which one?"})
        assert waiting.json()["state"] == "waiting_input"
        consumed = await api.post(f"/runs/{run_id}/resume", json=_start_body(token))
        assert consumed.json()["reason"] == "superseded"
        message_id = await fake.deliver_followup(run_id, text="The second one")
        page = await _poll(api)
        [followup] = [e for e in page["events"] if e["type"] == "agent.followup"]
        assert followup["payload"]["message_id"] == str(message_id)
        assert _claims(followup["invocation_token"])["trigger_kind"] == "followup"
        resumed = await api.post(
            f"/runs/{run_id}/resume", json=_start_body(followup["invocation_token"])
        )
        assert resumed.status_code == 200, resumed.text
        assert resumed.json()["state"] == "running"
        done = await api.post(f"/runs/{run_id}/finish", json={"state": "completed"})
        assert done.json()["state"] == "completed"
        repeat = await api.post(f"/runs/{run_id}/finish", json={"state": "completed"})
        assert repeat.status_code == 200
        costs = await api.post(f"/runs/{run_id}/finish", json={"state": "completed", "costs": []})
        error = costs.json()["errors"][0]
        assert (costs.status_code, error["pointer"], error["code"]) == (
            400,
            "/costs",
            "not_enabled",
        )
    assert fake.run_state(run_id).starts == 2


async def test_refused_run_finished_failed(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, _ = await _queued_with_token(fake, api)
        r = await api.post(
            f"/runs/{run_id}/finish", json={"state": "failed", "reason": "invalid_invocation_token"}
        )
    assert r.status_code == 200
    view = fake.run_state(run_id)
    assert (view.state, view.state_reason, view.starts) == ("failed", "invalid_invocation_token", 0)


async def test_cancel_running_and_queued(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        await api.post(f"/runs/{run_id}/start", json=_start_body(token))
        await fake.cancel(run_id)
        page = (await _poll(api))["events"]
        [event] = [e for e in page if e["type"] == "run.cancel_requested"]
        assert event["type"] == "run.cancel_requested"
        assert "invocation_token" not in event
        assert fake.run_state(run_id).state == "running"
        await api.post(
            f"/runs/{run_id}/finish", json={"state": "cancelled", "reason": "user_cancelled"}
        )
        queued = await fake.trigger()
        await fake.cancel(queued.run_id)
    assert fake.run_state(queued.run_id).state == "cancelled"


# Patches ------------------------------------------------------------------------------------


async def test_patch_rules(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        started = (await api.post(f"/runs/{run_id}/start", json=_start_body(token))).json()
        base = started["version"]
        forbidden = await api.patch(
            f"/runs/{run_id}",
            json={"base_version": base, "ops": [{"op": "replace", "path": "/state", "value": "x"}]},
        )
        assert forbidden.status_code == 400
        assert forbidden.json()["code"] == "validation_failed"
        error = forbidden.json()["errors"][0]
        assert (error["code"], error["pointer"]) == ("patch_path_forbidden", "/ops/0")
        ok = await api.patch(
            f"/runs/{run_id}",
            json={
                "base_version": base,
                "ops": [{"op": "replace", "path": "/status_line", "value": "Reading"}],
            },
        )
        assert ok.json() == {"version": base + 1}
        stale = await api.patch(
            f"/runs/{run_id}",
            json={"base_version": base, "ops": [{"op": "replace", "path": "/model", "value": "m"}]},
        )
        assert stale.status_code == 409
        assert stale.json()["code"] == "base_version_conflict"
        current = models.Run.from_dict(stale.json()["current"])
        assert current.version == base + 1
        assert current.status_line == "Reading"
        step = "01900000-0000-7000-8000-00000000abcd"
        steps = await api.patch(
            f"/runs/{run_id}",
            json={
                "base_version": base + 1,
                "ops": [
                    {"op": "replace", "path": "/model", "value": "example-model"},
                    {
                        "op": "add",
                        "path": f"/steps/{step}",
                        "value": {"kind": "tool_call", "name": "read"},
                    },
                    {"op": "replace", "path": f"/steps/{step}/status", "value": "completed"},
                ],
            },
        )
        assert steps.status_code == 200, steps.text
        run = (await api.get(f"/runs/{run_id}")).json()
        assert run["model"] == "example-model"
        assert run["steps"][step]["ordinal"] == 1
        assert run["steps"][step]["status"] == "completed"
        ordinal = await api.patch(
            f"/runs/{run_id}",
            json={
                "base_version": run["version"],
                "ops": [
                    {
                        "op": "add",
                        "path": "/steps/01900000-0000-7000-8000-00000000abce",
                        "value": {"kind": "status", "name": "n", "ordinal": 3},
                    }
                ],
            },
        )
        error = ordinal.json()["errors"][0]
        assert (error["pointer"], error["code"]) == ("/ops/0/value/ordinal", "patch_path_forbidden")
        bad_id = await api.patch(
            f"/runs/{run_id}",
            json={
                "base_version": run["version"],
                "ops": [{"op": "add", "path": f"/steps/{UUID(int=7)}", "value": {}}],
            },
        )
        error = bad_id.json()["errors"][0]
        assert (error["pointer"], error["code"]) == ("/ops/0/path", "invalid_format")
        await api.post(f"/runs/{run_id}/finish", json={"state": "completed"})
        ended = await api.patch(
            f"/runs/{run_id}",
            json={
                "base_version": run["version"] + 1,
                "ops": [{"op": "remove", "path": "/status_line"}],
            },
        )
        assert ended.status_code == 409
        assert (ended.json()["code"], ended.json()["reason"]) == ("conflict", "run_not_active")


async def test_run_update_over_gateway(fake: FakeServer) -> None:
    gw = await _connect(fake)
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        base = (await api.post(f"/runs/{run_id}/start", json=_start_body(token))).json()["version"]
        await gw.send(
            "run.update",
            {
                "run_id": str(run_id),
                "base_version": base,
                "ref": "r1",
                "ops": [{"op": "replace", "path": "/status_line", "value": "Thinking"}],
            },
        )
        await gw.send(
            "run.update",
            {
                "run_id": str(run_id),
                "base_version": base + 1,
                "ref": "r2",
                "ops": [{"op": "replace", "path": "/state", "value": "done"}],
            },
        )
        forbidden = await gw.until("error")
        assert forbidden["d"] == {
            "op": "run.update",
            "code": "validation_failed",
            "detail": "/ops/0: patch_path_forbidden",
            "ref": "r2",
        }
        await gw.send(
            "run.update",
            {
                "run_id": str(run_id),
                "base_version": base,
                "ref": "r3",
                "ops": [{"op": "replace", "path": "/status_line", "value": "Late"}],
            },
        )
        conflict = await gw.until("error")
        assert conflict["d"] == {"op": "run.update", "code": "base_version_conflict", "ref": "r3"}
        run = (await api.get(f"/runs/{run_id}")).json()
    assert (run["status_line"], run["version"]) == ("Thinking", base + 1)
    await gw.ws.close()


# Invocation-Token header, commands, interactions, autocomplete ------------------------------


async def test_invocation_token_header(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        bound = await api.get(f"/runs/{run_id}/context", headers={"Invocation-Token": token})
        assert bound.status_code == 200
        models.RunContext.from_dict(bound.json())
        assert bound.json()["trigger_message"]["content_md"] == "Hello @Lotse"
        bad = await api.get(f"/runs/{run_id}", headers={"Invocation-Token": "a.b.c"})
        assert (bad.status_code, bad.json()["reason"]) == (403, "invalid")
        await api.post(
            f"/runs/{run_id}/finish", json={"state": "failed", "reason": "adapter_error"}
        )
        inactive = await api.get(f"/runs/{run_id}", headers={"Invocation-Token": token})
        assert (inactive.status_code, inactive.json()["reason"]) == (403, "run_inactive")
        recorded = next(r for r in fake.requests if r.operation_id == "getRunContext")
        assert recorded.headers["invocation-token"] == fingerprint(token)
        assert token not in json.dumps([r.json for r in fake.requests])


async def test_messages_commands_and_interactions(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        await api.post(f"/runs/{run_id}/start", json=_start_body(token))
        thread_id = fake.run(run_id)["thread_id"]
        body = {
            "client_msg_id": "01900000-0000-7000-8000-00000000c001",
            "content_md": "Done",
            "run_id": str(run_id),
            "components": [
                {
                    "custom_id": "approval.once",
                    "type": "button",
                    "definition": {"label": "Once", "style": "primary"},
                }
            ],
        }
        created = await api.post(f"/threads/{thread_id}/messages", json=body)
        assert created.status_code == 201
        message = models.Message.from_dict(created.json())
        replay = await api.post(f"/threads/{thread_id}/messages", json=body)
        assert replay.status_code == 200
        assert replay.headers["Idempotent-Replay"] == "true"
        assert len(fake.messages) == 1
        component = message.components[0]
        upd = await api.patch(
            f"/components/{component.id}",
            json={"state": "disabled"},
            headers={"If-Match": f'"{component.version}"'},
        )
        assert upd.status_code == 200
        reg = await api.post(
            "/commands",
            json={
                "name": "status",
                "description": "Show the status",
                "scope_kind": "channel",
                "scope_id": fake.run(run_id)["channel_id"],
                "options": [],
            },
        )
        assert reg.status_code == 201
        command = models.Command.from_dict(reg.json())
        listed = await api.get(f"/channels/{fake.run(run_id)['channel_id']}/commands")
        assert [c["id"] for c in listed.json()["items"]] == [str(command.id)]
        no_match = await api.put(
            f"/commands/{command.id}", json={"name": "status", "description": "x"}
        )
        assert no_match.status_code == 428
        put = await api.put(
            f"/commands/{command.id}",
            json={"name": "status", "description": "New"},
            headers={"If-Match": f'"{command.version}"'},
        )
        assert put.status_code == 200
        assert (await api.delete(f"/commands/{command.id}")).status_code == 204
        interaction_id = await fake.deliver_interaction(run_id)
        page = await _poll(api)
        [event] = [e for e in page["events"] if e["type"] == "interaction.created"]
        assert event["payload"]["interaction"]["id"] == str(interaction_id)
        assert _claims(event["invocation_token"])["interaction_id"] == str(interaction_id)
        responded = await api.post(
            f"/interactions/{interaction_id}/respond", json={"state": "acknowledged"}
        )
        assert responded.status_code == 200
        models.Interaction.from_dict(responded.json())


async def test_autocomplete_over_gateway(fake: FakeServer) -> None:
    gw = await _connect(fake)
    command_id = await fake.register_command("deploy")
    request_id = await fake.request_autocomplete(command_id, "pr")
    frame = (await gw.until("dispatch"))["d"]
    assert frame["type"] == "command.autocomplete_requested"
    assert "seq" not in frame
    models.CommandAutocompleteRequestedEvent.from_dict(
        {k: v for k, v in frame.items() if k != "invocation_token"}
    )
    claims = _claims(frame["invocation_token"])
    assert (claims["trigger_kind"], "run_id" in claims) == ("autocomplete", False)
    await gw.send(
        "command.autocomplete_result",
        {"request_id": str(request_id), "choices": [{"name": "Production", "value": "prod"}]},
    )
    await _eventually(lambda: fake.autocomplete_answer(request_id) is not None)
    answer = fake.autocomplete_answer(request_id)
    assert answer is not None
    assert [c.value for c in answer] == ["prod"]
    await gw.ws.close()


# Faults -------------------------------------------------------------------------------------


def _message(thread_id: str, n: int) -> tuple[str, dict[str, Any]]:
    return f"/threads/{thread_id}/messages", {
        "client_msg_id": f"01900000-0000-7000-8000-00000000d{n:03d}",
        "content_md": f"m{n}",
    }


async def test_faults(fake: FakeServer) -> None:
    triggered = await fake.trigger()
    thread = str(triggered.thread_id)
    async with _api(fake) as api:
        fake.inject("createMessage", Fault.stored_then_502())
        path, body = _message(thread, 1)
        r = await api.post(path, json=body)
        assert r.status_code == 502
        assert len(fake.messages) == 1
        assert (await api.post(path, json=body)).status_code == 200  # the stored original

        fake.inject("createMessage", Fault.rate_limited(7))
        path, body = _message(thread, 2)
        limited = await api.post(path, json=body)
        assert (limited.status_code, limited.headers["Retry-After"]) == (429, "7")
        assert limited.json()["code"] == "rate_limited"
        assert len(fake.messages) == 1

        fake.inject("createMessage", Fault.disconnect())
        path, body = _message(thread, 3)
        with pytest.raises(httpx.TransportError):
            await api.post(path, json=body)
        assert len(fake.messages) == 1

        fake.inject("createMessage", Fault.stored_late(5))
        path, body = _message(thread, 4)
        late = asyncio.create_task(api.post(path, json=body))
        await _eventually(lambda: fake.clock.sleepers > 0)
        assert not late.done()
        assert len(fake.messages) == 1
        await fake.pass_time(5)
        assert (await within_patience(late)).status_code == 201
        assert len(fake.messages) == 2


async def test_gateway_disconnect_fault(fake: FakeServer) -> None:
    gw = await _connect(fake)
    await fake.trigger()
    frame = (await gw.until("dispatch"))["d"]
    fake.inject("ack", Fault.disconnect())
    await gw.send("ack", {"seq": frame["seq"]})
    with pytest.raises(websockets.ConnectionClosed):
        await gw.until("never")
    assert fake.consumer_state("gw")[0] < frame["seq"]


async def test_deprecation_header(fake: FakeServer) -> None:
    fake.deprecate("listServedChannels")
    async with _api(fake) as api:
        r = await api.get("/agents/me/channels")
        other = await api.get("/agents/me/triggerers")
    assert r.headers["Deprecation"] == "true"
    assert r.headers["Sunset"]
    assert "Deprecation" not in other.headers
    models.AgentTriggerers.from_dict(other.json())


async def test_recorded_requests_never_hold_tokens(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        await api.post(f"/runs/{run_id}/start", json=_start_body(token))
    start = next(r for r in fake.requests if r.operation_id == "startRun")
    assert start.json is not None
    assert start.json["invocation_token"] == fingerprint(token)
    assert start.headers["authorization"] == fingerprint(
        f"DPoP {fake.credentials.api_token.reveal()}"
    )
    text = repr(fake.requests)
    assert token not in text
    assert fake.credentials.api_token.reveal() not in text


async def test_timings_are_validated() -> None:
    with pytest.raises(ValueError, match="unknown"):
        FakeServer(timings={"agent.no_such_key": 1})
    assert Secret("x") != Secret("y")


async def test_forge_for_a_run_never_delivered(fake: FakeServer) -> None:
    triggered = await fake.trigger()  # no consumer: the run stays queued without a delivery
    async with _api(fake) as api:
        for variant in ("bad_signature", "alg_hs256", "typ_access", "aud_other_agent"):
            forged = fake.forge_token(triggered.run_id, variant).reveal()
            start = await api.post(f"/runs/{triggered.run_id}/start", json=_start_body(forged))
            renew = await api.post(
                f"/runs/{triggered.run_id}/invocation-token", json={"invocation_token": forged}
            )
            for answer in (start, renew):
                assert (answer.status_code, answer.json()["reason"]) == (403, "invalid"), variant
    assert fake.run_state(triggered.run_id) == RunView("queued", None, 1, 0)


@pytest.mark.parametrize("code", [4015, 4012])
async def test_session_not_resumable_after_a_final_close(fake: FakeServer, code: int) -> None:
    first = await _connect(fake, "c7")
    if code == 4015:
        second = await _connect(fake, "c7")
    else:
        await fake.close_connections(code)
    assert await _close_code(first) == code
    resumed = await _resume(fake, first.session_id)
    assert await _close_code(resumed) == 4006
    if code == 4015:
        run = await fake.trigger()
        frame = (await second.until("dispatch"))["d"]
        assert frame["payload"]["run_id"] == str(run.run_id)
        await second.ws.close()


async def test_resume_loses_against_a_newer_holder(fake: FakeServer) -> None:
    first = await _connect(fake, "c2")
    fake.inject("heartbeat", Fault.disconnect())
    await first.send("heartbeat", {"last_seq": 0})
    with pytest.raises(websockets.ConnectionClosed):
        await first.until("never")
    second = await _connect(fake, "c2")
    late = await _resume(fake, first.session_id)
    assert (await late.recv())["op"] == "resumed"
    assert await _close_code(late) == 4015
    run = await fake.trigger()
    frame = (await second.until("dispatch"))["d"]
    assert frame["payload"]["run_id"] == str(run.run_id)
    await second.ws.close()


async def test_resume_continues_a_dropped_session(fake: FakeServer) -> None:
    first = await _connect(fake, "c3")
    fake.inject("heartbeat", Fault.disconnect())
    await first.send("heartbeat", {"last_seq": 0})
    with pytest.raises(websockets.ConnectionClosed):
        await first.until("never")
    again = await _resume(fake, first.session_id)
    resumed = await again.recv()
    assert resumed["op"] == "resumed"
    models.GatewayResumed.from_dict(resumed["d"])
    run = await fake.trigger()
    frame = (await again.until("dispatch"))["d"]
    assert frame["payload"]["run_id"] == str(run.run_id)
    await again.ws.close()


async def test_patch_needs_an_integer_base_version(fake: FakeServer) -> None:
    async with _api(fake) as api:
        run_id, token = await _queued_with_token(fake, api)
        await api.post(f"/runs/{run_id}/start", json=_start_body(token))
        ops = [{"op": "replace", "path": "/status_line", "value": "x"}]
        for body in ({"ops": ops}, {"base_version": True, "ops": ops}):
            r = await api.patch(f"/runs/{run_id}", json=body)
            assert (r.status_code, r.json()["errors"][0]["pointer"]) == (400, "/base_version")


async def test_backlog_beyond_the_budget_completes_after_acks() -> None:
    async with FakeServer(timings={"agent.max_inflight_events": 1}) as fake:
        first = await _connect(fake, "c4")
        await first.ws.close()
        await fake.trigger()
        await fake.trigger()
        gw, nonce = await _open(fake)
        await gw.send(
            "identify", {"protocol_version": 1, "consumer": "c4", **_credentials(fake, nonce)}
        )
        assert (await gw.recv())["op"] == "ready"
        one = await gw.recv()
        assert (one["op"], one["d"]["seq"]) == ("dispatch", 1)
        await gw.send("ack", {"seq": 1})
        two = await gw.recv()
        assert (two["op"], two["d"]["seq"]) == ("dispatch", 2)
        assert (await gw.recv()) == {"op": "catchup.complete", "d": {"head_seq": 2}}
        await gw.ws.close()


async def test_identify_refused_while_restarting(fake: FakeServer) -> None:
    gw = await _connect(fake, "c5")
    await fake.restart("c5", acked_seq=0)
    assert await _close_code(gw) == 4013
    again = await _connect(fake, "c5")
    await again.ws.close()
