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
from collections.abc import AsyncIterator, Callable, Sequence
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
from bothy_sdk.commands import AutocompleteRequest, CommandSpec, ReconcileResult
from bothy_sdk.generated.models import (
    CommandChoice,
    Heartbeat,
    HeartbeatAck,
    ProjectReadiness,
    ProjectTrashResult,
    ProjectTrashResultResult,
)
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import EventSource, SourceReady
from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.doubles import RecordingHandler, RecordingSource
from bothy_sdk.testing.fakeserver import FakeServer
from bothy_sdk.testing.faults import Fault


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
    def __init__(self, reason: str | None, summary: str | None = None) -> None:
        super().__init__()
        self.reason = reason
        self.summary = summary
        self.not_started: list[UUID] = []

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        await super().prepare(inv, run)
        raise RunRefused(self.reason, summary=self.summary)

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


def finish_bodies(fake: FakeServer, run_id: UUID) -> list[Any]:
    return [
        r.json for r in fake.requests if r.operation_id == "finishRun" and str(run_id) in r.path
    ]


@pytest.mark.anyio
async def test_run_refused_with_a_summary_names_the_cause(fake: FakeServer) -> None:
    """RUNNER-NO-GIT: the cause of a refusal reaches the thread as the run's summary."""
    handler = Refusing("adapter_error", summary="This forum has no project folder.")
    adapter = make_adapter(fake, handler, "cause")
    async with running(adapter, fake):
        triggered = await fake.trigger()
        await until(fake, lambda: handler.not_started == [triggered.run_id])
    assert fake.run_state(triggered.run_id).state_reason == "adapter_error"
    (body,) = finish_bodies(fake, triggered.run_id)
    assert body["summary"] == "This forum has no project folder."


@pytest.mark.anyio
async def test_run_refused_without_a_summary_sends_none(fake: FakeServer) -> None:
    handler = Refusing("runtime_not_authenticated")
    adapter = make_adapter(fake, handler, "nocause")
    async with running(adapter, fake):
        triggered = await fake.trigger()
        await until(fake, lambda: handler.not_started == [triggered.run_id])
    (body,) = finish_bodies(fake, triggered.run_id)
    assert "summary" not in body


class Crashing(RecordingHandler):
    def __init__(self, where: str) -> None:
        super().__init__()
        self.where = where

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        options = await super().prepare(inv, run)
        if self.where == "prepare":
            raise KeyError("/private/path/never/shown")
        return options

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        if self.where == "execute":
            raise KeyError("/private/path/never/shown")
        await super().execute(inv, run)


@pytest.mark.anyio
@pytest.mark.parametrize("where", ["prepare", "execute"])
async def test_unexpected_handler_error_names_its_kind(fake: FakeServer, where: str) -> None:
    """RUNNER-NO-GIT: an unexpected error is never a bare adapter_error; never its message."""
    handler = Crashing(where)
    adapter = make_adapter(fake, handler, f"crash-{where}")
    async with running(adapter, fake):
        triggered = await fake.trigger()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "failed")
    assert fake.run_state(triggered.run_id).state_reason == "adapter_error"
    (body,) = finish_bodies(fake, triggered.run_id)
    assert "KeyError" in body["summary"]
    assert "private" not in body["summary"]


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
async def test_runtime_status_sends_the_project_folders(fake: FakeServer) -> None:
    """``RuntimeStatus.project_folders`` reaches the fake server (M4-44)."""
    handler = Reporting(RuntimeStatus(project_folders=("api", "website")))
    adapter = make_adapter(fake, handler, "folders")
    async with running(adapter, fake):
        await until(fake, lambda: handler.ready == 1)
        before = len(fake.heartbeats)
        await adapter.send_heartbeat()
    assert fake.heartbeats[before]["project_folders"] == ["api", "website"]


class Acked(Reporting):
    def __init__(self, status: RuntimeStatus) -> None:
        super().__init__(status)
        self.acks: list[tuple[Heartbeat, HeartbeatAck]] = []

    async def on_heartbeat_ack(self, sent: Heartbeat, ack: HeartbeatAck) -> None:
        self.acks.append((sent, ack))


@pytest.mark.anyio
async def test_send_heartbeat_hands_the_answer_with_project_trash_to_the_handler(
    fake: FakeServer,
) -> None:
    """A folder move of a deleted forum reaches the runner in the answer (09 18.15)."""
    channel = UUID("0192d3a0-0000-7000-8000-00000000c0de")
    handler = Acked(RuntimeStatus())
    adapter = make_adapter(fake, handler, "trash")
    fake.request_project_trash(channel, "website")
    async with running(adapter, fake):
        await until(fake, lambda: handler.ready == 1)
        await adapter.send_heartbeat()
        sent, ack = handler.acks[-1]
        assert sent.to_dict()["state"] == "online"
        requested = [(r.channel_id, r.workspace_ref) for r in ack.project_trash]
        assert requested == [(channel, "website")]
        # The outcome travels in a later heartbeat and ends the request.
        handler.status = RuntimeStatus(
            project_trash_results=(
                ProjectTrashResult(
                    channel_id=channel, result=ProjectTrashResultResult.REFUSED, reason="symlink"
                ),
            )
        )
        before = len(fake.heartbeats)
        await adapter.send_heartbeat()
    assert fake.heartbeats[before]["project_trash_results"] == [
        {"channel_id": str(channel), "result": "refused", "reason": "symlink"}
    ]
    assert fake.project_trash_results == [
        {"channel_id": str(channel), "result": "refused", "reason": "symlink"}
    ]
    _, ack = handler.acks[-1]
    assert not ack.project_trash


@pytest.mark.anyio
async def test_a_failing_heartbeat_ack_hook_changes_nothing(fake: FakeServer) -> None:
    class Failing(Reporting):
        async def on_heartbeat_ack(self, sent: Heartbeat, ack: HeartbeatAck) -> None:
            raise RuntimeError("hook failed")

    handler = Failing(RuntimeStatus())
    adapter = make_adapter(fake, handler, "failing")
    async with running(adapter, fake):
        await until(fake, lambda: handler.ready == 1)
        before = len(fake.heartbeats)
        await adapter.send_heartbeat()
        assert len(fake.heartbeats) == before + 1


@pytest.mark.anyio
async def test_polling_heartbeats_reach_the_heartbeat_ack_hook(fake: FakeServer) -> None:
    channel = UUID("0192d3a0-0000-7000-8000-00000000c0df")
    handler = Acked(RuntimeStatus())
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer="polled",
        settings=fake.sdk_settings,
        transport="polling",
    )
    handler.clock = fake.clock
    adapter = Adapter(config, handler, clock=fake.clock)
    fake.request_project_trash(channel, "docs")
    async with running(adapter, fake):
        await until(fake, lambda: any(a.project_trash for _, a in handler.acks))


@pytest.mark.anyio
async def test_without_runtime_status_the_heartbeat_is_unchanged(fake: FakeServer) -> None:
    adapter = make_adapter(fake, RecordingHandler(), "plain")
    beat = adapter.heartbeat().to_dict()
    assert beat["state"] == "online"
    assert "project_readiness" not in beat
    assert "project_folders" not in beat
    assert "billing_modes_supported" not in beat


@pytest.mark.anyio
async def test_runtime_status_reports_the_billing_mode_and_its_evidence(fake: FakeServer) -> None:
    status = RuntimeStatus(
        billing_modes_supported=("organization",),
        reported_billing_mode="organization",
        billing_evidence={"key_type": "none"},
    )
    adapter = make_adapter(fake, Reporting(status), "mode")
    beat = adapter.heartbeat().to_dict()
    assert beat["reported_billing_mode"] == "organization"
    assert beat["billing_evidence"] == {"key_type": "none"}


class Announcing(RecordingHandler):
    def __init__(self) -> None:
        super().__init__()
        self.reconciled: list[ReconcileResult] = []
        self.sent: list[tuple[UUID, list[str]]] = []
        self.ready = 0

    async def on_ready(self) -> None:
        self.ready += 1

    async def on_commands_reconciled(self, result: ReconcileResult) -> None:
        self.reconciled.append(result)

    async def on_autocomplete(self, request: AutocompleteRequest) -> list[CommandChoice]:
        return [CommandChoice(name="billing", value="billing")]

    async def on_autocomplete_sent(
        self, request: AutocompleteRequest, choices: Sequence[CommandChoice]
    ) -> None:
        self.sent.append((request.request_id, [c.value for c in choices]))


@pytest.mark.anyio
async def test_on_commands_reconciled_follows_each_reconciliation(fake: FakeServer) -> None:
    handler = Announcing()
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer="commands",
        settings=fake.sdk_settings,
        commands=[CommandSpec("version", "Show the version", control=True)],
    )
    handler.clock = fake.clock
    adapter = Adapter(config, handler, clock=fake.clock)
    async with running(adapter, fake):
        await until(fake, lambda: len(handler.reconciled) == 1)
    assert handler.reconciled[0].created == ("version",)


@pytest.mark.anyio
async def test_without_declared_commands_nothing_is_reconciled(fake: FakeServer) -> None:
    handler = Announcing()
    adapter = make_adapter(fake, handler, "nocommands")
    async with running(adapter, fake):
        await until(fake, lambda: handler.ready == 1)
    assert handler.reconciled == []


@pytest.mark.anyio
async def test_on_autocomplete_sent_follows_the_sent_answer(fake: FakeServer) -> None:
    handler = Announcing()
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer="suggest",
        settings=fake.sdk_settings,
    )
    handler.clock = fake.clock
    adapter = Adapter(config, handler, clock=fake.clock)
    async with running(adapter, fake):
        command_id = await fake.register_command("summarize")
        request_id = await fake.request_autocomplete(command_id, "bi")
        await until(fake, lambda: handler.sent != [])
    assert handler.sent == [(request_id, ["billing"])]
    answer = fake.autocomplete_answer(request_id)
    assert answer is not None
    assert [c.value for c in answer] == ["billing"]


class FailingHooks(Announcing):
    async def on_commands_reconciled(self, result: ReconcileResult) -> None:
        await super().on_commands_reconciled(result)
        raise RuntimeError("handler bug")

    async def on_autocomplete_sent(
        self, request: AutocompleteRequest, choices: Sequence[CommandChoice]
    ) -> None:
        await super().on_autocomplete_sent(request, choices)
        raise RuntimeError("handler bug")


@pytest.mark.anyio
async def test_failing_hooks_keep_the_connection(fake: FakeServer) -> None:
    handler = FailingHooks()
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer="failing",
        settings=fake.sdk_settings,
        commands=[CommandSpec("version", "Show the version", control=True)],
    )
    handler.clock = fake.clock
    adapter = Adapter(config, handler, clock=fake.clock)
    async with running(adapter, fake):
        await until(fake, lambda: len(handler.reconciled) == 1)
        command_id = await fake.register_command("summarize")
        first = await fake.request_autocomplete(command_id, "bi")
        await until(fake, lambda: len(handler.sent) == 1)
        second = await fake.request_autocomplete(command_id, "bil")
        await until(fake, lambda: len(handler.sent) == 2)
    assert [r for r, _ in handler.sent] == [first, second]


@pytest.mark.anyio
async def test_a_failed_reconciliation_calls_no_hook(fake: FakeServer) -> None:
    handler = Announcing()
    fake.inject("listServedChannels", Fault.problem("forbidden"))
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer="refused",
        settings=fake.sdk_settings,
        commands=[CommandSpec("version", "Show the version", control=True)],
    )
    handler.clock = fake.clock
    adapter = Adapter(config, handler, clock=fake.clock)
    async with running(adapter, fake):
        await until(fake, lambda: handler.ready == 1)
        await until(
            fake, lambda: any(r.operation_id == "listServedChannels" for r in fake.requests)
        )
    assert handler.reconciled == []
