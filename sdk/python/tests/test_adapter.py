# SPDX-License-Identifier: Apache-2.0
"""Adapter loop against the FakeServer: acceptance order, start, resume, cancel, restart, SR9."""

from __future__ import annotations

import asyncio
import contextlib
from collections.abc import AsyncIterator, Callable
from pathlib import Path
from typing import Any
from uuid import UUID

import httpx
import pytest

from bothy_sdk.adapter import Adapter, AdapterConfig, Invocation, RuntimeFailure, StartOptions
from bothy_sdk.generated.api.runs import get_run
from bothy_sdk.inbox import EntryState, InboxEntry, MemoryInboxStore
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import EventSource, SourceReady
from bothy_sdk.testing.doubles import RecordingHandler, RecordingSource, fingerprint
from bothy_sdk.testing.fakeserver import FakeServer

Log = list[str]


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


async def until(fake: FakeServer, predicate: Callable[[], Any], limit: int = 400) -> None:
    for _ in range(limit):
        result = predicate()
        if asyncio.iscoroutine(result):
            result = await result
        if result:
            return
        await fake.pass_time(0.05)
        await asyncio.sleep(0.005)
    raise AssertionError("condition not reached")


class LoggingInbox(MemoryInboxStore):
    def __init__(self, log: Log) -> None:
        super().__init__()
        self.log = log

    async def add(self, entry: InboxEntry) -> None:
        await super().add(entry)
        self.log.append(f"store:{entry.type}")


class LoggingSource(RecordingSource):
    def __init__(self, inner: EventSource, log: Log) -> None:
        super().__init__(inner)
        self.log = log

    async def ack(self, seq: int) -> None:
        self.log.append("ack")
        await self.inner.ack(seq)

    async def settle(self, seq: int) -> None:
        # Route through this wrapper so the acknowledgement is visible in the log.
        point = self.inner.window.settle(seq)  # type: ignore[attr-defined]
        if point is not None:
            await self.ack(point)


class LoggingTransport(httpx.AsyncBaseTransport):
    def __init__(self, log: Log) -> None:
        self.inner = httpx.AsyncHTTPTransport()
        self.log = log
        self.requests: list[httpx.Request] = []

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.requests.append(request)
        response = await self.inner.handle_async_request(request)
        name = request.url.path.rsplit("/", 1)[-1]
        if request.method == "POST" and name in ("start", "resume", "finish", "invocation-token"):
            self.log.append(f"{name}:{response.status_code}")
        return response

    async def aclose(self) -> None:
        await self.inner.aclose()


class LoggingHandler(RecordingHandler):
    def __init__(self, log: Log, **kw: Any) -> None:
        super().__init__(**kw)
        self.log = log

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        self.log.append("prepare")
        return await super().prepare(inv, run)

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        self.log.append("execute")
        await super().execute(inv, run)


def make_adapter(fake: FakeServer, handler: RecordingHandler, consumer: str, **kw: Any) -> Adapter:
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer=consumer,
        settings=fake.sdk_settings,
        state_dir=kw.pop("state_dir", None),
        transport=kw.pop("transport", "gateway"),
    )
    handler.clock = fake.clock
    return Adapter(config, handler, clock=fake.clock, **kw)


@contextlib.asynccontextmanager
async def running(
    adapter: Adapter,
    wrap: Callable[[EventSource], EventSource] | None = None,
    fake: FakeServer | None = None,
) -> AsyncIterator[asyncio.Task[Any]]:
    source = await adapter.make_source()
    adapter.source = wrap(source) if wrap is not None else RecordingSource(source)
    task = asyncio.ensure_future(adapter.run())
    rec = adapter.source
    if fake is not None:
        await until(fake, lambda: any(isinstance(d, SourceReady) for d in rec.deliveries))  # type: ignore[attr-defined]
    try:
        yield task
    finally:
        await adapter.stop()
        with contextlib.suppress(Exception):
            await asyncio.wait_for(task, 10)


@pytest.mark.anyio
async def test_acceptance_order(fake: FakeServer) -> None:
    log: Log = []
    handler = LoggingHandler(log)
    adapter = make_adapter(
        fake, handler, "order", inbox=LoggingInbox(log), http_transport=LoggingTransport(log)
    )
    async with running(adapter, lambda s: LoggingSource(s, log), fake):
        triggered = await fake.trigger()
        await until(fake, lambda: "execute" in log)
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed")
    steps = ("store:agent.invocation", "ack", "prepare", "start:200", "execute")
    first = [log.index(step) for step in steps]
    assert first == sorted(first), log


@pytest.mark.anyio
async def test_execute_only_after_start_200(fake: FakeServer) -> None:
    from bothy_sdk.testing.faults import Fault

    log: Log = []
    handler = LoggingHandler(log)
    adapter = make_adapter(fake, handler, "after200", http_transport=LoggingTransport(log))
    fake.inject("startRun", Fault.rate_limited(1))
    async with running(adapter, fake=fake):
        await fake.trigger()
        await until(fake, lambda: "execute" in log)
    starts = [e for e in log if e.startswith("start:")]
    assert starts in (["start:429", "start:200"], ["start:200"])
    assert log.index("execute") > log.index("start:200")


class WaitingHandler(RecordingHandler):
    """Asks a question in execute and in on_followup; ends after the interaction."""

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        await super().execute(inv, run)
        await run.await_input("Which file?")

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        await super().on_followup(inv, run)
        await run.await_input("Approve?")


@pytest.mark.anyio
async def test_resume_on_followup_and_interaction(fake: FakeServer) -> None:
    handler = WaitingHandler()
    adapter = make_adapter(fake, handler, "resume")
    async with running(adapter, fake=fake):
        triggered = await fake.trigger()
        run_id = triggered.run_id
        await until(fake, lambda: fake.run_state(run_id).state == "waiting_input")
        await fake.deliver_followup(run_id)
        await until(fake, lambda: handler.count("on_followup") == 1)
        await until(fake, lambda: fake.run_state(run_id).state == "waiting_input")
        await fake.deliver_interaction(run_id)
        await until(fake, lambda: handler.count("on_interaction") == 1)
        await until(fake, lambda: fake.run_state(run_id).state == "completed")
    assert fake.run_state(run_id).starts == 3  # start and two resumes


@pytest.mark.anyio
async def test_cancel_requested_finishes_cancelled(fake: FakeServer) -> None:
    block = asyncio.Event()
    handler = RecordingHandler(block_execute=block)
    adapter = make_adapter(fake, handler, "cancel")
    async with running(adapter, fake=fake):
        triggered = await fake.trigger()
        await until(fake, lambda: handler.count("execute") == 1)
        await fake.cancel(triggered.run_id)
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "cancelled")
    view = fake.run_state(triggered.run_id)
    assert view.state_reason == "user_cancelled"


class Failing(RecordingHandler):
    def __init__(self, error: Exception) -> None:
        super().__init__()
        self.error = error

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        await super().execute(inv, run)
        raise self.error


@pytest.mark.parametrize(
    ("error", "reason"),
    [(RuntimeFailure("model gone"), "runtime_error"), (ValueError("bug"), "adapter_error")],
)
@pytest.mark.anyio
async def test_handler_exception_reasons(fake: FakeServer, error: Exception, reason: str) -> None:
    handler = Failing(error)
    adapter = make_adapter(fake, handler, f"fail-{reason}")
    async with running(adapter, fake=fake):
        triggered = await fake.trigger()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "failed")
    assert fake.run_state(triggered.run_id).state_reason == reason
    assert handler.count("execute") == 1  # never retried


@pytest.mark.anyio
async def test_restart_resumes_accepted_and_closes_started(
    fake: FakeServer, tmp_path: Path
) -> None:
    first = RecordingHandler(block_execute=asyncio.Event())
    adapter = make_adapter(fake, first, "restart", state_dir=tmp_path)
    async with running(adapter, fake=fake):
        started = await fake.trigger("started @Lotse")
        await until(fake, lambda: first.count("execute") == 1)
        first.block_prepare = asyncio.Event()  # the second run stays in prepare
        accepted = await fake.trigger("accepted @Lotse")
        await until(fake, lambda: first.count("prepare", accepted.run_id) == 1)
    assert (tmp_path / "inbox.sqlite3").stat().st_mode & 0o777 == 0o600
    second = RecordingHandler()
    log: Log = []
    restarted = make_adapter(
        fake, second, "restart", state_dir=tmp_path, http_transport=LoggingTransport(log)
    )
    async with running(restarted, fake=fake):
        await until(fake, lambda: second.count("execute", accepted.run_id) == 1)
        entries = await restarted.inbox.for_run(started.run_id)
        assert all(e.state is EntryState.CLOSED for e in entries)
    assert second.count("prepare", started.run_id) == 0
    assert fake.run_state(accepted.run_id).starts == 1
    finishes = [r for r in log if r.startswith("finish")]
    assert len(finishes) == 1  # only the run that started in the new process


@pytest.mark.anyio
async def test_superseded_waits_and_closes_when_started_elsewhere(fake: FakeServer) -> None:
    gates = {"alpha": asyncio.Event(), "beta": asyncio.Event()}
    alpha = RecordingHandler(block_prepare=gates["alpha"])
    beta = RecordingHandler(block_prepare=gates["beta"])
    a = make_adapter(fake, alpha, "sup-alpha")
    b = make_adapter(fake, beta, "sup-beta")
    async with running(a, fake=fake), running(b, fake=fake):
        triggered = await fake.trigger()
        run_id = triggered.run_id
        await until(fake, lambda: alpha.count("prepare") == 1 and beta.count("prepare") == 1)
        current = fake.world.runs[run_id].invocation_jti
        names = {"alpha": alpha, "beta": beta}
        holder = next(n for n, h in names.items() if h.invocations[0].claims.jti == current)
        other = "beta" if holder == "alpha" else "alpha"
        other_adapter = a if other == "alpha" else b
        gates[other].set()  # the stale token starts first: superseded, the run is still queued

        async def waiting() -> bool:
            state = other_adapter._runs.get(run_id)
            return state is not None and state.waiting_for_token

        await until(fake, waiting)
        gates[holder].set()
        await until(fake, lambda: names[holder].count("execute") == 1)

        async def closed() -> bool:
            entries = await other_adapter.inbox.for_run(run_id)
            return bool(entries) and all(e.state is EntryState.CLOSED for e in entries)

        await until(fake, closed)
    assert names[other].count("execute") == 0
    assert fake.run_state(run_id).starts == 1


class ContextHandler(RecordingHandler):
    def __init__(self) -> None:
        super().__init__()
        self.seen: list[str | None] = []

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        await run.load_context()
        await run.client.call(get_run.asyncio_detailed, run_id=run.run_id)
        return await super().prepare(inv, run)


@pytest.mark.anyio
async def test_invocation_header_inside_prepare(fake: FakeServer) -> None:
    handler = ContextHandler()
    recording: dict[str, RecordingSource] = {}

    def wrap(source: EventSource) -> EventSource:
        recording["source"] = RecordingSource(source)
        return recording["source"]

    adapter = make_adapter(fake, handler, "sr9")
    async with running(adapter, wrap, fake):
        triggered = await fake.trigger()
        await until(fake, lambda: handler.count("execute") == 1)
    env = recording["source"].envelopes("agent.invocation")[0]
    assert env.token is not None
    expected = fingerprint(env.token)
    bound = [r for r in fake.requests if r.method == "GET" and str(triggered.run_id) in r.path]
    assert len(bound) >= 2
    for request in bound:
        assert request.headers.get("invocation-token") == expected
    discovery = [r for r in fake.requests if r.path.endswith("/discovery")]
    assert all("invocation-token" not in r.headers for r in discovery)
    _ = UUID


@pytest.mark.anyio
async def test_heartbeat_reports_active_runs(fake: FakeServer) -> None:
    block = asyncio.Event()
    handler = RecordingHandler(block_execute=block)
    adapter = make_adapter(fake, handler, "beat")
    async with running(adapter, fake=fake):
        triggered = await fake.trigger()
        await until(fake, lambda: handler.count("execute") == 1)
        await fake.pass_time(fake.timings["sync.heartbeat_interval"] * 2)
        await until(
            fake,
            lambda: any(
                str(triggered.run_id) in h.get("details", {}).get("active_runs", [])
                for h in fake.heartbeats
            ),
        )
        block.set()
    beat = fake.heartbeats[-1]
    assert beat["details"]["adapter"] == {"name": "bothy-adapter", "version": "0.0.0"}
    assert beat["state"] == "online"


@pytest.mark.anyio
async def test_polling_adapter_runs(fake: FakeServer) -> None:
    handler = RecordingHandler()
    adapter = make_adapter(fake, handler, "polling", transport="polling")
    async with running(adapter, fake=fake):
        triggered = await fake.trigger()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed")
    assert handler.count("execute") == 1
    assert any(r.operation_id == "pollAgentEvents" for r in fake.requests)
    assert any(r.operation_id == "ackAgentEvents" for r in fake.requests)


@pytest.mark.anyio
async def test_renewal_refused_closes_the_entry(fake: FakeServer) -> None:
    gate = asyncio.Event()
    late = RecordingHandler(block_prepare=gate)
    quick = RecordingHandler()
    a = make_adapter(fake, late, "renew-late")
    b = make_adapter(fake, quick, "renew-quick")
    async with running(a, fake=fake), running(b, fake=fake):
        triggered = await fake.trigger()
        run_id = triggered.run_id
        await until(fake, lambda: late.count("prepare") == 1)
        await until(fake, lambda: quick.count("execute") == 1)
        ttl = fake.timings["agent.invocation_token_ttl"] + fake.timings["auth.clock_leeway"]
        await fake.pass_time(ttl + 1)
        gate.set()  # the late side's token expired; the renewal answers 409 already_started

        async def closed() -> bool:
            entries = await a.inbox.for_run(run_id)
            return bool(entries) and all(e.state is EntryState.CLOSED for e in entries)

        await until(fake, closed)
        assert run_id not in a._runs
    assert late.count("execute") == 0
    assert fake.run_state(run_id).starts == 1


class TwoAnswers(RecordingHandler):
    """Asks once; works on an answer until the second answer arrived too."""

    def __init__(self) -> None:
        super().__init__()
        self.both = asyncio.Event()

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        await super().execute(inv, run)
        await run.await_input("Which file?")

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        await super().on_followup(inv, run)
        if self.count("on_followup") == 2:
            self.both.set()
        await asyncio.wait_for(self.both.wait(), 5)


@pytest.mark.anyio
async def test_two_answers_to_one_question_reach_the_handler(fake: FakeServer) -> None:
    handler = TwoAnswers()
    adapter = make_adapter(fake, handler, "two-answers")
    async with running(adapter, fake=fake):
        triggered = await fake.trigger()
        run_id = triggered.run_id
        await until(fake, lambda: fake.run_state(run_id).state == "waiting_input")
        await fake.deliver_followup(run_id, "the first file")
        await fake.deliver_followup(run_id, "no, the second")
        await until(fake, lambda: handler.count("on_followup") == 2)
        await until(fake, lambda: fake.run_state(run_id).state == "completed")
    assert fake.run_state(run_id).starts == 2  # the start and one resume
