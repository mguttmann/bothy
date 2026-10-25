# SPDX-License-Identifier: Apache-2.0
"""Handler hooks of the run loop added for the Claude Code runner (M2-34).

`RunRefused` ends a run from ``prepare`` with a given reason (or without ``finishRun``),
`AgentHandler.on_not_started` frees what ``prepare`` took when a run does not reach ``execute``,
`AgentHandler.runtime_status` supplies the heartbeat members that change at run time,
`Adapter.send_heartbeat` sends one at once and `AgentHandler.on_ready` follows each ready source.
"""

from __future__ import annotations

import asyncio
import contextlib
from collections.abc import AsyncIterator, Callable
from datetime import UTC, datetime
from typing import Any
from uuid import UUID

import pytest

from bothy_sdk.adapter import (
    ActualBilling,
    Adapter,
    AdapterConfig,
    Invocation,
    RunRefused,
    RuntimeStatus,
    StartOptions,
)
from bothy_sdk.generated.models import ProjectReadiness
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import EventSource, SourceReady
from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.doubles import RecordingHandler, RecordingSource
from bothy_sdk.testing.fakeserver import FakeServer


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


async def until(fake: FakeServer, predicate: Callable[[], Any]) -> None:
    await eventually(predicate, "the condition", step=lambda: fake.pass_time(0.05))


def make_adapter(fake: FakeServer, handler: RecordingHandler, consumer: str) -> Adapter:
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer=consumer,
        settings=fake.sdk_settings,
    )
    handler.clock = fake.clock
    return Adapter(config, handler, clock=fake.clock)


@contextlib.asynccontextmanager
async def running(adapter: Adapter, fake: FakeServer) -> AsyncIterator[None]:
    source: EventSource = RecordingSource(await adapter.make_source())
    adapter.source = source
    task = asyncio.ensure_future(adapter.run())
    await until(fake, lambda: any(isinstance(d, SourceReady) for d in source.deliveries))  # type: ignore[attr-defined]
    try:
        yield
    finally:
        await adapter.stop()
        with contextlib.suppress(Exception):
            await within_patience(task)


class Refusing(RecordingHandler):
    def __init__(self, reason: str | None) -> None:
        super().__init__()
        self.reason = reason
        self.not_started: list[UUID] = []

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        await super().prepare(inv, run)
        raise RunRefused(self.reason)

    async def on_not_started(self, inv: Invocation) -> None:
        self.not_started.append(inv.claims.run_id or UUID(int=0))


def ops(fake: FakeServer, name: str) -> list[str]:
    return [r.operation_id for r in fake.requests if r.operation_id == name]


@pytest.mark.anyio
async def test_run_refused_in_prepare_finishes_with_its_reason(fake: FakeServer) -> None:
    handler = Refusing("runtime_not_authenticated")
    adapter = make_adapter(fake, handler, "refused")
    async with running(adapter, fake):
        triggered = await fake.trigger()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "failed")
        await until(fake, lambda: handler.not_started == [triggered.run_id])
    assert fake.run_state(triggered.run_id).state_reason == "runtime_not_authenticated"
    assert ops(fake, "startRun") == []
    assert handler.count("execute") == 0


@pytest.mark.anyio
async def test_run_refused_without_reason_closes_without_finish(fake: FakeServer) -> None:
    handler = Refusing(None)
    adapter = make_adapter(fake, handler, "gone")
    async with running(adapter, fake):
        triggered = await fake.trigger()
        await until(fake, lambda: handler.not_started == [triggered.run_id])
        entry = await adapter.inbox.get(handler.invocations[0].event_id)
    assert entry is not None
    assert entry.state.value == "closed"
    assert ops(fake, "finishRun") == []
    assert ops(fake, "startRun") == []
    assert fake.run_state(triggered.run_id).state == "queued"


class MismatchAtStart(RecordingHandler):
    """Prepare succeeds; the second comparison at the start finds an unknown mode."""

    def __init__(self) -> None:
        super().__init__()
        self.asked = 0
        self.mismatch_at_start = True
        self.not_started: list[UUID] = []

    def actual_billing(self, inv: Invocation) -> ActualBilling:
        self.asked += 1
        if self.mismatch_at_start and self.asked > 1:
            return ActualBilling("unknown")
        return ActualBilling(inv.claims.billing_mode)

    async def on_not_started(self, inv: Invocation) -> None:
        self.not_started.append(inv.claims.run_id or UUID(int=0))


@pytest.mark.anyio
async def test_on_not_started_after_a_refused_start(fake: FakeServer) -> None:
    handler = MismatchAtStart()
    adapter = make_adapter(fake, handler, "mismatch")
    async with running(adapter, fake):
        triggered = await fake.trigger()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "failed")
        await until(fake, lambda: handler.not_started == [triggered.run_id])
    assert fake.run_state(triggered.run_id).state_reason == "billing_mode_mismatch"
    assert handler.count("prepare") == 1
    assert handler.count("execute") == 0


@pytest.mark.anyio
async def test_on_not_started_is_not_called_after_execute(fake: FakeServer) -> None:
    handler = MismatchAtStart()
    handler.mismatch_at_start = False
    adapter = make_adapter(fake, handler, "executed")
    async with running(adapter, fake):
        triggered = await fake.trigger()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed")
    assert handler.not_started == []


class Reporting(RecordingHandler):
    def __init__(self, status: RuntimeStatus) -> None:
        super().__init__()
        self.status = status
        self.ready = 0

    def runtime_status(self) -> RuntimeStatus:
        return self.status

    async def on_ready(self) -> None:
        self.ready += 1


@pytest.mark.anyio
async def test_runtime_status_fills_the_heartbeat_and_send_heartbeat_sends_now(
    fake: FakeServer,
) -> None:
    channel = UUID("0192d3a0-0000-7000-8000-00000000c0de")
    handler = Reporting(
        RuntimeStatus(
            state="degraded",
            degraded_reasons=("runtime_version_untested",),
            runtime_version="2.1.300",
            billing_modes_supported=("organization",),
            project_readiness=(ProjectReadiness(channel_id=channel, ready=True),),
        )
    )
    adapter = make_adapter(fake, handler, "status")
    async with running(adapter, fake):
        await until(fake, lambda: handler.ready == 1)
        before = len(fake.heartbeats)
        await adapter.send_heartbeat()
        assert len(fake.heartbeats) == before + 1
    beat = fake.heartbeats[before]
    assert beat["state"] == "degraded"
    assert beat["runtime_version"] == "2.1.300"
    assert beat["billing_modes_supported"] == ["organization"]
    assert beat["project_readiness"] == [{"channel_id": str(channel), "ready": True}]
    assert beat["details"]["degraded_reasons"] == ["runtime_version_untested"]
    sent = [r for r in fake.requests if r.operation_id == "sendHeartbeat"]
    assert sent
    assert all("invocation-token" not in r.headers for r in sent)


@pytest.mark.anyio
async def test_runtime_status_sends_the_rate_limit(fake: FakeServer) -> None:
    """``RuntimeStatus.rate_limit`` reaches the fake server, and nothing else new (M6-14)."""
    resets = datetime(2026, 10, 10, 9, 30, tzinfo=UTC)
    handler = Reporting(RuntimeStatus(rate_limit={"state": "near_limit", "resets_at": resets}))
    adapter = make_adapter(fake, handler, "limits")
    plain = make_adapter(fake, RecordingHandler(), "plain").heartbeat().to_dict()
    async with running(adapter, fake):
        await until(fake, lambda: handler.ready == 1)
        before = len(fake.heartbeats)
        await adapter.send_heartbeat()
    beat = fake.heartbeats[before]
    assert beat["rate_limit"] == {"state": "near_limit", "resets_at": "2026-10-10T09:30:00+00:00"}
    assert set(beat) - set(plain) == {"rate_limit"}


@pytest.mark.anyio
async def test_without_runtime_status_the_heartbeat_is_unchanged(fake: FakeServer) -> None:
    adapter = make_adapter(fake, RecordingHandler(), "plain")
    beat = adapter.heartbeat().to_dict()
    assert beat["state"] == "online"
    assert "project_readiness" not in beat
    assert "billing_modes_supported" not in beat
