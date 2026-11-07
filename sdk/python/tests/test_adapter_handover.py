# SPDX-License-Identifier: Apache-2.0
"""The handover hooks of the run loop (docs/spec/09-agents.md 18.19.3, 18.19.7; ticket AR-03).

`Adapter.stop` with ``hand_over=True`` lets a runner hand its live turns over at SIGTERM: from
then on no heartbeat goes out, the run tasks are cancelled with `Adapter.handing_over` set, so
the handler leaves its runtime running, and the SDK neither finishes such a run nor closes its
inbox entry. `AgentHandler.adopted_run` lets a handler take the follow-ups of a run it adopted
from an earlier process; the default (``None``) closes them as before. Nothing changes for an
adapter that uses neither.
"""

from __future__ import annotations

import asyncio
import contextlib
from collections.abc import AsyncIterator, Callable
from typing import Any
from uuid import UUID

import pytest

from bothy_sdk.adapter import Adapter, AdapterConfig, Invocation
from bothy_sdk.inbox import EntryState, MemoryInboxStore
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


def make_adapter(
    fake: FakeServer,
    handler: RecordingHandler,
    consumer: str,
    inbox: MemoryInboxStore | None = None,
) -> Adapter:
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer=consumer,
        settings=fake.sdk_settings,
    )
    handler.clock = fake.clock
    return Adapter(config, handler, clock=fake.clock, inbox=inbox)


async def started(adapter: Adapter, fake: FakeServer) -> asyncio.Task[Any]:
    source: EventSource = RecordingSource(await adapter.make_source())
    adapter.source = source
    task = asyncio.ensure_future(adapter.run())
    await until(fake, lambda: any(isinstance(d, SourceReady) for d in source.deliveries))  # type: ignore[attr-defined]
    return task


class Keeping(RecordingHandler):
    """Its runtime runs on; it notes whether its run task was cancelled for a handover."""

    def __init__(self) -> None:
        super().__init__(block_execute=asyncio.Event())
        self.adapter: Adapter | None = None
        self.cancelled_for: list[bool] = []

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        try:
            await super().execute(inv, run)
        except asyncio.CancelledError:
            assert self.adapter is not None
            self.cancelled_for.append(self.adapter.handing_over)
            raise


def finishes(fake: FakeServer, run_id: UUID) -> list[Any]:
    return [r for r in fake.requests if r.operation_id == "finishRun" and str(run_id) in r.path]


@pytest.mark.anyio
async def test_stop_with_hand_over_leaves_the_run_and_sends_no_heartbeat(
    fake: FakeServer,
) -> None:
    handler = Keeping()
    inbox = MemoryInboxStore()
    adapter = make_adapter(fake, handler, "handover", inbox)
    handler.adapter = adapter
    task = await started(adapter, fake)
    triggered = await fake.trigger()
    await until(fake, lambda: handler.count("execute") == 1)
    entry = (await adapter.inbox.for_run(triggered.run_id))[0]
    assert not adapter.handing_over

    await adapter.stop(hand_over=True)
    beats = len(fake.heartbeats)
    await adapter.send_heartbeat()  # no heartbeat after the handover began (18.19.3 item 1)
    with contextlib.suppress(Exception):
        await within_patience(task)

    assert len(fake.heartbeats) == beats
    assert handler.cancelled_for == [True]
    assert finishes(fake, triggered.run_id) == []
    assert fake.run_state(triggered.run_id).state == "running"
    stored = await inbox.get(entry.event_id)  # the memory store outlives the adapter
    assert stored is not None
    assert stored.state is EntryState.STARTED


@pytest.mark.anyio
async def test_a_cancel_request_in_flight_is_not_finished_during_a_handover(
    fake: FakeServer,
) -> None:
    # The turn runs on under the next runner: the SDK must not finish its run cancelled.
    handler = Keeping()
    adapter = make_adapter(fake, handler, "handover-cancel")
    handler.adapter = adapter
    task = await started(adapter, fake)
    triggered = await fake.trigger()
    await until(fake, lambda: handler.count("execute") == 1)
    state = adapter._runs[triggered.run_id]
    state.cancel_reason = "user_cancelled"  # a request that the handler had not acted on yet
    await adapter.stop(hand_over=True)
    with contextlib.suppress(Exception):
        await within_patience(task)
    assert finishes(fake, triggered.run_id) == []


@pytest.mark.anyio
async def test_plain_stop_is_unchanged(fake: FakeServer) -> None:
    handler = Keeping()
    adapter = make_adapter(fake, handler, "plain")
    handler.adapter = adapter
    task = await started(adapter, fake)
    await fake.trigger()
    await until(fake, lambda: handler.count("execute") == 1)
    await adapter.stop()
    with contextlib.suppress(Exception):
        await within_patience(task)
    assert handler.cancelled_for == [False]
    assert not adapter.handing_over


class Adopting(RecordingHandler):
    """Holds the handle of a run an earlier process started (an adopted turn)."""

    def __init__(self) -> None:
        super().__init__()
        self.adopted: dict[UUID, RunHandle] = {}

    def adopted_run(self, run_id: UUID) -> RunHandle | None:
        return self.adopted.get(run_id)


@pytest.mark.anyio
async def test_a_followup_of_an_adopted_run_reaches_the_handler(fake: FakeServer) -> None:
    # 18.19.7: the adopter takes follow-ups; the run did not start in this process.
    first = RecordingHandler(block_execute=asyncio.Event())
    old = make_adapter(fake, first, "adopt")
    task = await started(old, fake)
    triggered = await fake.trigger()
    await until(fake, lambda: first.count("execute") == 1)
    await old.stop(hand_over=True)
    with contextlib.suppress(Exception):
        await within_patience(task)

    handler = Adopting()
    new = make_adapter(fake, handler, "adopt")
    handler.adopted[triggered.run_id] = RunHandle(
        new.client, triggered.run_id, triggered.thread_id, outbox=new.outbox
    )
    task = await started(new, fake)
    try:
        await fake.deliver_followup(triggered.run_id, "Also the footer", author="sam")
        await until(fake, lambda: handler.count("on_followup", triggered.run_id) == 1)
    finally:
        await new.stop()
        with contextlib.suppress(Exception):
            await within_patience(task)
    assert handler.count("on_followup", triggered.run_id) == 1


@pytest.mark.anyio
async def test_without_the_hook_a_followup_of_a_foreign_run_is_closed(fake: FakeServer) -> None:
    first = RecordingHandler(block_execute=asyncio.Event())
    old = make_adapter(fake, first, "foreign")
    task = await started(old, fake)
    triggered = await fake.trigger()
    await until(fake, lambda: first.count("execute") == 1)
    await old.stop(hand_over=True)
    with contextlib.suppress(Exception):
        await within_patience(task)
    handler = RecordingHandler()
    new = make_adapter(fake, handler, "foreign")
    task = await started(new, fake)
    try:
        message_id = await fake.deliver_followup(triggered.run_id, "Hello", author="sam")
        await until(fake, lambda: fake.consumer_state("foreign")[0] >= 1)
        await asyncio.sleep(0)
    finally:
        await new.stop()
        with contextlib.suppress(Exception):
            await within_patience(task)
    assert handler.count("on_followup") == 0
    assert fake.followup_state(triggered.run_id, message_id) == "queued"
