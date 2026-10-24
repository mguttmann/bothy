# SPDX-License-Identifier: Apache-2.0
"""The Fake-Agent against the FakeServer (brief M2-31, Tests "Unit, Python").

Every op runs through `FakeAgent.handle` in the test's event loop; the FakeServer is the only
listener (127.0.0.1, port 0). The real-server use of the Fake-Agent is the conformance suite in
``services/server/test/agents``.
"""

# ruff: noqa: PT018, S106 - compound checks of one reply; token names are no secrets

from __future__ import annotations

import asyncio
import logging
import os
import subprocess
import sys
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest
from _fake_agent_driver import Driver, _b64, private_jwk

from bothy_sdk.testing.fake_agent import ControlledClock, MemoryDpopKey
from bothy_sdk.testing.fakeserver import FakeServer
from bothy_sdk.testing.faults import Fault

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


@pytest.fixture
async def driver(fake: FakeServer) -> AsyncIterator[Driver]:
    d = Driver(fake)
    yield d
    await d.close()


async def test_receive_accepts_and_acknowledges_only_afterwards(
    fake: FakeServer, driver: Driver
) -> None:
    await driver.hello()
    trig = await fake.trigger()
    held = await driver.op("receive", type="agent.invocation", accept=False)
    assert held["verdict"] == "valid" and held["run_id"] == str(trig.run_id)
    assert fake.consumer_state("default")[0] < held["seq"]
    await fake.redeliver()
    got = await driver.op("receive", type="agent.invocation")
    assert got["verdict"] == "valid" and got["redelivered"] is True
    assert fake.consumer_state("default")[0] >= got["seq"]
    assert (await driver.op("receive", type="agent.invocation")) == {"none": True}


async def test_redelivery_replaces_the_token_and_is_not_processed_twice(
    fake: FakeServer, driver: Driver
) -> None:
    await driver.hello()
    trig = await fake.trigger()
    first = await driver.op("receive", type="agent.invocation", accept=False, keep_token="t1")
    await fake.redeliver()
    second = await driver.op("receive", type="agent.invocation", keep_token="t2")
    assert first["id"] == second["id"] and first["jti_hash"] != second["jti_hash"]
    newest = await driver.op("export_token", run=str(trig.run_id))
    assert newest["jti_hash"] == second["jti_hash"]
    assert (await driver.op("receive", type="agent.invocation")) == {"none": True}
    started = await driver.op("start", run=str(trig.run_id))
    assert started["status"] == 200 and started["run_state"] == "running"


async def test_unknown_kid_is_neither_accepted_nor_acknowledged(
    fake: FakeServer, driver: Driver, monkeypatch: pytest.MonkeyPatch
) -> None:
    from bothy_sdk.testing._fake_tokens import Issuer

    await driver.hello()
    other = Issuer(fake.instance_id)
    monkeypatch.setattr(fake.world.issuer, "issue", other.issue)
    await fake.trigger()
    got = await driver.op("receive", type="agent.invocation")
    assert got["verdict"] == "unknown_key" and "finished" not in got
    assert fake.consumer_state("default")[0] < got["seq"]


@pytest.mark.parametrize(
    ("runtime", "forged", "verdict"),
    [
        ({}, "bad_signature", "invalid"),
        ({"billing_mode": "personal"}, None, "billing_mode_mismatch"),
        ({"tool_class": "host"}, None, "tool_class_mismatch"),
    ],
)
async def test_well_behaved_default_finishes_the_run(
    fake: FakeServer, driver: Driver, runtime: dict[str, Any], forged: str | None, verdict: str
) -> None:
    await driver.hello(runtime=runtime)
    trig = await fake.trigger(forged=forged)
    got = await driver.op("receive", type="agent.invocation")
    assert got["verdict"] == verdict
    assert got["finished"]["status"] == 200
    view = fake.run_state(trig.run_id)
    reason = {"invalid": "invalid_invocation_token"}.get(verdict, verdict)
    assert (view.state, view.state_reason, view.starts) == ("failed", reason, 0)


async def test_skip_own_checks_sends_the_start_anyway(fake: FakeServer, driver: Driver) -> None:
    await driver.hello(runtime={"billing_mode": "personal"})
    trig = await fake.trigger()
    got = await driver.op("receive", type="agent.invocation", skip_own_checks=True)
    assert got["verdict"] == "billing_mode_mismatch" and "finished" not in got
    refused = await driver.op("start", run=str(trig.run_id))
    assert refused == {"refused": "billing_mode_mismatch"}
    sent = await driver.op(
        "start", run=str(trig.run_id), skip_own_checks=True, billing_mode_used="personal"
    )
    assert sent["status"] == 409 and sent["code"] == "billing_mode_mismatch"


async def test_patch_with_a_stale_base_reports_the_conflict_and_rebases(
    fake: FakeServer, driver: Driver
) -> None:
    await driver.hello()
    trig = await fake.trigger()
    await driver.op("receive", type="agent.invocation")
    started = await driver.op("start", run=str(trig.run_id))
    ops = [{"op": "replace", "path": "/status_line", "value": "Reading"}]
    got = await driver.op("patch", run=str(trig.run_id), base_version="stale", ops=ops)
    assert got["status"] == 409 and got["code"] == "base_version_conflict"
    assert got["current_version"] == started["run_version"]
    assert got["rebased"] == {"status": 200, "current_version": started["run_version"] + 1}
    forbidden = await driver.op(
        "patch",
        run=str(trig.run_id),
        ops=[{"op": "replace", "path": "/state", "value": "completed"}],
    )
    assert forbidden["status"] == 400 and forbidden["code"] == "validation_failed"
    no_rebase = await driver.op(
        "patch", run=str(trig.run_id), base_version="stale", rebase=False, ops=ops
    )
    assert no_rebase["code"] == "base_version_conflict" and "rebased" not in no_rebase


async def test_heartbeat_lists_the_runs_it_holds(fake: FakeServer, driver: Driver) -> None:
    await driver.hello()
    trig = await fake.trigger()
    await driver.op("receive", type="agent.invocation")
    await driver.op("start", run=str(trig.run_id))
    got = await driver.op("heartbeat", capabilities={"followups": "steer"})
    assert got["status"] == 200
    details = fake.heartbeats[-1]["details"]
    assert details["active_runs"] == [str(trig.run_id)] and details["capabilities"] == {
        "followups": "steer"
    }
    done = await driver.op("finish", run=str(trig.run_id), state="completed")
    assert done["status"] == 200
    await driver.op("heartbeat")
    assert fake.heartbeats[-1]["details"]["active_runs"] == []


async def test_renew_without_retry_reports_a_dropped_answer_after_one_request(
    fake: FakeServer, driver: Driver
) -> None:
    await driver.hello()
    trig = await fake.trigger()
    await driver.op("receive", type="agent.invocation")
    fake.inject("renewInvocationToken", Fault.disconnect())
    before = sum(1 for r in fake.requests if r.operation_id == "renewInvocationToken")
    got = await driver.op("renew", run=str(trig.run_id), retry=False)
    assert got["error"] == "transport"
    after = sum(1 for r in fake.requests if r.operation_id == "renewInvocationToken")
    assert after - before == 1


async def test_crash_exits_non_zero_without_a_request(tmp_path: Path) -> None:
    src = Path(__file__).resolve().parents[2] / "src"
    env = {**os.environ, "PYTHONPATH": str(src)}
    done = subprocess.run(
        [sys.executable, "-m", "bothy_sdk.testing.fake_agent"],
        input=b'{"id": 1, "op": "crash", "args": {}}\n',
        capture_output=True,
        env=env,
        cwd=tmp_path,
        check=False,
    )
    assert done.returncode == 3 and done.stdout == b""


async def test_the_clock_follows_now_for_expiry_and_jwks_max_age(
    fake: FakeServer, driver: Driver
) -> None:
    await driver.hello()
    trig = await fake.trigger()
    await driver.op("receive", type="agent.invocation", accept=False, keep_token="t1")
    fetches = driver.agent._verifier.fetches  # type: ignore[union-attr]
    ttl = fake.timings["agent.invocation_token_ttl"] + 10
    max_age = fake.timings["agent.jwks_max_age"] + 1
    await fake.pass_time(max(ttl, max_age))
    # A token check reads the JWKS again once its max-age passed on the harness clock.
    got = await driver.op("verify", token="t1", run=str(trig.run_id))
    assert got["verdict"] == "expired"
    assert driver.agent._verifier.fetches == fetches + 1  # type: ignore[union-attr]


def test_controlled_clock_never_goes_back() -> None:
    from datetime import UTC, datetime, timedelta

    clock = ControlledClock()
    start = datetime(2030, 1, 1, tzinfo=UTC)
    clock.set(start)
    mono = clock.monotonic()
    clock.set(start - timedelta(hours=1))
    assert clock.now() == start and clock.monotonic() >= mono
    clock.set(start + timedelta(hours=1))
    assert clock.monotonic() >= mono + 3600


def test_memory_dpop_key_refuses_other_keys() -> None:
    with pytest.raises(ValueError, match="P-256"):
        MemoryDpopKey({"kty": "OKP", "crv": "Ed25519", "x": "AA"})
    assert "redacted" in repr(MemoryDpopKey(private_jwk_for_tests()))


def private_jwk_for_tests() -> dict[str, str]:
    from cryptography.hazmat.primitives.asymmetric import ec

    key = ec.generate_private_key(ec.SECP256R1())
    n = key.private_numbers()
    return {
        "kty": "EC",
        "crv": "P-256",
        "x": _b64(n.public_numbers.x),
        "y": _b64(n.public_numbers.y),
        "d": _b64(n.private_value),
    }


async def test_no_secrets_in_logs(fake: FakeServer, caplog: pytest.LogCaptureFixture) -> None:
    caplog.set_level(logging.DEBUG)
    canary = "bothy_agt_canary0000000000000000000000000"
    driver = Driver(fake)
    try:
        await driver.hello()
        trig = await fake.trigger()
        replies = [await driver.op("receive", type="agent.invocation")]
        replies.append(await driver.op("start", run=str(trig.run_id), token="forged:bad_signature"))
        replies.append(await driver.op("renew", run=str(trig.run_id)))
        token = (await driver.op("export_token", run=str(trig.run_id)))["token"]
        other = Driver(fake)
        await other.op(
            "hello",
            base_url=fake.base_url,
            gateway_url="/api/v1/gateway",
            instance_id=str(fake.instance_id),
            agent_id=str(fake.agent_id),
            credentials={"token": canary, "dpop_private_jwk": private_jwk(fake)},
            runtime={},
        )
        replies.append(await other.op("connect"))
        replies.append(await other.op("heartbeat"))
        await other.close()
    finally:
        await driver.close()
    text = repr(replies) + caplog.text
    for secret in (canary, token, fake.credentials.api_token.reveal()):
        assert secret not in text
    assert any(r.get("status") == 401 for r in replies)


async def test_ops_answer_usage_errors_for_bad_arguments(fake: FakeServer, driver: Driver) -> None:
    await driver.hello()
    assert (await driver.op("start", run="x", token="nothing"))["error"] == "usage"
    assert (await driver.op("start", run="x", token="forged:swap"))["error"] == "usage"
    assert (await driver.op("autocomplete_mode", mode="maybe"))["error"] == "usage"
    assert (await driver.op("verify", event="unknown"))["error"] == "usage"
    assert (await driver.op("hello", base_url="x"))["error"] == "usage"
    await asyncio.sleep(0)
