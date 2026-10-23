# SPDX-License-Identifier: Apache-2.0
"""SDK support for the Hermes adapter (M2-33): allow-list failures, reply IDs, costs, FakeServer.

The allow list of 17.7 that cannot be loaded (docs/DECISIONS.md D-M2-33-3), the server's ID of a
created reply for a later ``updateMessage``, the end of a run on a server that refuses ``costs``,
and the FakeServer operations an adapter of a runtime with sessions needs (``getMe``,
``getThread``, ``setThreadTitle``, ``updateMessage``, ``reportAgentSession``, ``downloadFile``,
contexts with every trust mark, the single-use click and problem faults).
"""

from __future__ import annotations

import asyncio
import contextlib
import contextvars
from collections.abc import AsyncIterator, Callable
from typing import Any
from uuid import UUID

import httpx
import pytest

from bothy_sdk.adapter import Adapter, AdapterConfig, Invocation
from bothy_sdk.auth import DpopAuth
from bothy_sdk.client import Client
from bothy_sdk.errors import ValidationFailed
from bothy_sdk.generated.models import CostReport, RunContext
from bothy_sdk.ids import uuid7
from bothy_sdk.outbox import Outbox, OutboxState
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import SourceReady
from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.doubles import RecordingHandler, RecordingSource
from bothy_sdk.testing.fakeserver import ContextFile, ContextMessage, FakeServer
from bothy_sdk.testing.faults import Fault, FaultKind

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


async def _until(fake: FakeServer, predicate: Callable[[], Any]) -> None:
    await eventually(predicate, "the condition", step=lambda: fake.pass_time(0.05))


def _adapter(fake: FakeServer, handler: RecordingHandler, consumer: str) -> Adapter:
    config = AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer=consumer,
        settings=fake.sdk_settings,
        allowlist=True,
    )
    handler.clock = fake.clock
    return Adapter(config, handler, clock=fake.clock)


@contextlib.asynccontextmanager
async def _running(fake: FakeServer, adapter: Adapter) -> AsyncIterator[RecordingSource]:
    source = RecordingSource(await adapter.make_source())
    adapter.source = source
    task = asyncio.ensure_future(adapter.run())
    await _until(fake, lambda: any(isinstance(d, SourceReady) for d in source.deliveries))
    try:
        yield source
    finally:
        await adapter.stop()
        with contextlib.suppress(Exception):
            await within_patience(task)


def _api(fake: FakeServer) -> httpx.AsyncClient:
    return httpx.AsyncClient(
        base_url=fake.base_url, auth=DpopAuth(fake.credentials, clock=fake.clock)
    )


# ---- allow list (Design note 5 of M2-33) -------------------------------------------------


async def test_allowlist_404_fails_the_run_and_settles(fake: FakeServer) -> None:
    handler = RecordingHandler()
    adapter = _adapter(fake, handler, "allow-404")
    fake.inject("getAgentTriggerers", Fault.problem("not_found"), times=1)
    async with _running(fake, adapter):
        triggered = await fake.trigger()
        await _until(fake, lambda: fake.run_state(triggered.run_id).state == "failed")
        await _until(fake, lambda: fake.consumer_state("allow-404")[0] >= 1)
    view = fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", "adapter_error")
    assert handler.count("prepare") == 0
    entries = await adapter.inbox.for_run(triggered.run_id)
    assert all(e.token is None for e in entries)


@pytest.mark.parametrize(
    "fault",
    [Fault.problem("internal_error"), Fault.rate_limited(0), Fault.disconnect()],
    ids=["5xx", "429", "network"],
)
async def test_allowlist_transient_failure_leaves_the_event_unsettled(
    fake: FakeServer, fault: Fault
) -> None:
    handler = RecordingHandler()
    adapter = _adapter(fake, handler, "allow-transient")
    # Every attempt of the retry transport hits the fault, so the load fails as a whole.
    attempts = fake.sdk_settings.retry_max_attempts
    fake.inject("getAgentTriggerers", fault, times=attempts)

    def loads() -> int:
        return sum(1 for r in fake.requests if r.operation_id == "getAgentTriggerers")

    async with _running(fake, adapter):
        triggered = await fake.trigger()
        await _until(fake, lambda: loads() >= attempts)
        await _until(fake, lambda: adapter.verifier is not None)
        assert fake.run_state(triggered.run_id).state == "queued"
        assert await adapter.inbox.for_run(triggered.run_id) == []
        assert fake.consumer_state("allow-transient")[0] == 0
        assert handler.count("prepare") == 0


# ---- outbox: the created reply ------------------------------------------------------------


async def test_outbox_returns_the_created_message(fake: FakeServer) -> None:
    triggered = await fake.trigger()
    async with Client(fake.base_url, fake.credentials, clock=fake.clock) as client:
        outbox = Outbox(client)
        entry = await outbox.send_message(triggered.thread_id, "Hello")
    assert entry.state is OutboxState.SENT
    assert entry.message_id is not None
    stored = fake.message(entry.message_id)
    assert stored["content_md"] == "Hello"
    assert entry.message_version == stored["version"]


# ---- finishRun with costs -----------------------------------------------------------------


class _Costly(RecordingHandler):
    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        await super().execute(inv, run)
        await run.finish(
            "completed",
            costs=[CostReport(provider="anthropic", model="model-small", currency="USD")],
        )


@pytest.mark.parametrize("enabled", [True, False], ids=["costs-enabled", "costs-not-enabled"])
async def test_finish_with_costs(fake: FakeServer, enabled: bool) -> None:
    if enabled:
        fake.enable_costs()
    handler = _Costly()
    adapter = _adapter(fake, handler, "costs")
    async with _running(fake, adapter):
        triggered = await fake.trigger()
        await _until(fake, lambda: fake.run_state(triggered.run_id).state == "completed")
    finishes = [r for r in fake.requests if r.operation_id == "finishRun"]
    assert "costs" in finishes[0].json
    # Without cost booking the server refuses the member and the run ends without it.
    assert len(finishes) == (1 if enabled else 2)
    assert "costs" not in finishes[-1].json or enabled


async def test_finish_reraises_other_validation_errors(fake: FakeServer) -> None:
    seen: list[str] = []

    class Bad(RecordingHandler):
        async def execute(self, inv: Invocation, run: RunHandle) -> None:
            await super().execute(inv, run)
            try:
                await run.finish(
                    "cancelled",
                    reason="not_a_reason",
                    costs=[CostReport(provider="anthropic", model="m", currency="USD")],
                )
            except ValidationFailed as exc:
                seen.append(exc.code)

    fake.enable_costs()
    adapter = _adapter(fake, Bad(), "bad-finish")
    async with _running(fake, adapter):
        await fake.trigger()
        await _until(fake, lambda: seen)
    # Only a refusal of `costs` as not enabled is retried; a bad reason is the caller's error.
    assert seen == ["validation_failed"]
    assert len([r for r in fake.requests if r.operation_id == "finishRun"]) == 1


# ---- FakeServer operations -----------------------------------------------------------------


async def test_fake_me_thread_title_and_message_update(fake: FakeServer) -> None:
    triggered = await fake.trigger()
    async with _api(fake) as api:
        me = (await api.get("/me")).json()
        assert me["principal"]["id"] == str(fake.agent_id)
        thread = (await api.get(f"/threads/{triggered.thread_id}")).json()
        assert thread["title_source"] == "auto"
        path = f"/threads/{triggered.thread_id}/title"
        assert (await api.put(path, json={"title": "Trip"})).status_code == 428
        stale = await api.put(path, json={"title": "Trip"}, headers={"If-Match": '"9"'})
        assert stale.status_code == 412
        ok = await api.put(
            path, json={"title": "Trip"}, headers={"If-Match": f'"{thread["version"]}"'}
        )
        assert ok.status_code == 200
        assert fake.titles == [{"thread_id": str(triggered.thread_id), "title": "Trip"}]
        fake.set_manual_title(triggered.thread_id, "Mine")
        current = (await api.get(f"/threads/{triggered.thread_id}")).json()
        manual = await api.put(
            path, json={"title": "Trip"}, headers={"If-Match": f'"{current["version"]}"'}
        )
        assert (manual.status_code, manual.json()["code"]) == (409, "title_manual")

        created = await api.post(
            f"/threads/{triggered.thread_id}/messages",
            json={"client_msg_id": str(uuid7()), "kind": "text", "content_md": "draft"},
        )
        message = created.json()
        update = await api.patch(
            f"/messages/{message['id']}",
            json={"content_md": "final"},
            headers={"If-Match": f'"{message["version"]}"'},
        )
        assert update.json()["content_md"] == "final"
        foreign = await api.patch(
            f"/messages/{triggered.message_id}",
            json={"content_md": "x"},
            headers={"If-Match": '"1"'},
        )
        assert foreign.status_code == 403
    assert fake.message_updates[0]["body"] == {"content_md": "final"}


async def test_fake_session_reports(fake: FakeServer) -> None:
    triggered = await fake.trigger(session_mode="new")
    path = f"/threads/{triggered.thread_id}/agent-sessions/report"
    async with _api(fake) as api:
        failed = await api.post(
            path, json={"external_session_ref": "s1", "bind_failed": "session_missing"}
        )
        assert (failed.status_code, failed.json().get("reason")) == (409, "session_not_bound")
        first = await api.post(path, json={"external_session_ref": "s1", "started": True})
        assert first.status_code == 200
        body = first.json()
        assert (body["external_session_ref"], body["started"]) == ("s1", True)
        again = await api.post(path, json={"external_session_ref": "s2"})
        assert again.status_code == 412
        bound = await api.post(
            path,
            json={"external_session_ref": "s2"},
            headers={"If-Match": f'"{body["version"]}"'},
        )
        assert bound.json()["external_session_ref"] == "s2"
    assert [r["external_session_ref"] for r in fake.session_reports(triggered.thread_id)] == [
        "s1",
        "s2",
    ]
    # A later operator run of the thread carries the row as agent_session.
    later = await fake.trigger("again", thread_id=triggered.thread_id, session_mode="resume")
    event = next(
        e for e in fake.hub.log if e.envelope["payload"].get("run_id") == str(later.run_id)
    )
    assert event.envelope["payload"]["agent_session"]["external_session_ref"] == "s2"
    assert event.envelope["payload"]["session_mode"] == "resume"


async def test_fake_context_trust_files_and_download(fake: FakeServer) -> None:
    note = ContextFile(uuid7(), "notes.txt", "text/plain", b"0123456789")
    voice = ContextFile(uuid7(), "voice.m4a", "audio/mp4", b"\x00", transcript="spoken")
    pending = ContextFile(uuid7(), "late.txt", "text/plain", b"x", scan_status="pending")
    blocked = ContextFile(uuid7(), "bad.txt", "text/plain", b"x", scan_status="blocked")
    triggered = await fake.trigger(
        "Summarise",
        history=(
            ContextMessage("writer", "posted by writer", trust="unverified"),
            ContextMessage("texter", "by texter", trust="unverified", role="other"),
            ContextMessage("sam", "seen", trust="authorized", already_delivered=True),
        ),
        files=(note, voice, pending, blocked),
    )
    fake.set_channel("Answer briefly.", ("search",))
    async with _api(fake) as api:
        doc = (await api.get(f"/runs/{triggered.run_id}/context")).json()
        ctx = RunContext.from_dict(doc)
        assert [m.trust for m in ctx.thread_context] == ["unverified", "unverified", "authorized"]
        assert ctx.thread_context[1].role == "other"
        assert ctx.thread_context[2].already_delivered is True
        assert ctx.trigger_message.trust == "trigger"
        assert [a.original_name for a in ctx.trigger_message.attachments][:2] == [
            "notes.txt",
            "voice.m4a",
        ]
        assert ctx.trigger_message.attachments[1].transcript == "spoken"
        whole = await api.get(f"/files/{note.file_id}/content")
        assert (whole.status_code, whole.content) == (200, b"0123456789")
        part = await api.get(f"/files/{note.file_id}/content", headers={"Range": "bytes=0-3"})
        assert (part.status_code, part.content) == (206, b"0123")
        assert part.headers["Content-Range"] == "bytes 0-3/10"
        for item, code in ((pending, "file_not_ready"), (blocked, "file_blocked")):
            refused = await api.get(f"/files/{item.file_id}/content")
            assert (refused.status_code, refused.json()["code"]) == (409, code)
        config = (await api.get(f"/agents/me/channels/{fake.world.channel_id}/config")).json()
        assert config["behavior"]["prompt"] == "Answer briefly."
        assert config["behavior"]["skills"] == ["search"]
        triggerers = (await api.get("/agents/me/triggerers")).json()
        assert str(fake.sam_id) in triggerers["principal_ids"]


async def test_fake_click_is_single_use(fake: FakeServer) -> None:
    triggered = await fake.trigger()
    async with _api(fake) as api:
        body = {
            "client_msg_id": str(uuid7()),
            "kind": "card",
            "content_md": "Pick",
            "run_id": str(triggered.run_id),
            "components": [
                {"custom_id": "cl:1:0", "type": "button", "single_use": True, "definition": {}},
                {"custom_id": "cl:1:1", "type": "button", "single_use": True, "definition": {}},
            ],
        }
        assert (await api.post(f"/threads/{triggered.thread_id}/messages", json=body)).is_success
    first = await fake.click(triggered.run_id, "cl:1:0")
    assert isinstance(first, UUID)
    assert await fake.click(triggered.run_id, "cl:1:1", author="sam") is None
    with pytest.raises(KeyError):
        await fake.click(triggered.run_id, "cl:9:9")


async def test_problem_fault(fake: FakeServer) -> None:
    fault = Fault.problem("agent_invocation_denied")
    assert fault.kind is FaultKind.PROBLEM
    with pytest.raises(ValueError, match="code"):
        Fault.problem("")
    fake.inject("getMe", Fault.problem("conflict", reason="session_changed"))
    async with _api(fake) as api:
        refused = await api.get("/me")
        assert (refused.status_code, refused.json()["reason"]) == (409, "session_changed")
        assert (await api.get("/me")).status_code == 200
        missing = await api.get(f"/threads/{uuid7()}")
        assert missing.status_code == 404
        unknown = await api.get(f"/files/{uuid7()}/content")
        assert unknown.status_code == 404


# ---- binding a runtime callback to its run (07 SR9) ---------------------------------------


async def test_rebind_binds_a_callback_task_to_the_run(fake: FakeServer) -> None:
    bound: list[bool] = []

    class Callback(RecordingHandler):
        async def execute(self, inv: Invocation, run: RunHandle) -> None:
            await super().execute(inv, run)

            async def elsewhere() -> None:
                # A runtime callback: its own task, created outside the run's binding.
                with run.bound() as ok:
                    bound.append(ok)
                    await run.load_context()
                with run.client.rebind(uuid7()) as other:
                    bound.append(other)

            loop = asyncio.get_running_loop()
            await loop.create_task(elsewhere(), context=contextvars.Context())

    adapter = _adapter(fake, Callback(), "rebind")
    async with _running(fake, adapter):
        triggered = await fake.trigger()
        await _until(fake, lambda: fake.run_state(triggered.run_id).state == "completed")
    assert bound == [True, False]
    reads = [r for r in fake.requests if r.operation_id == "getRunContext"]
    assert reads
    assert "invocation-token" in reads[-1].headers


@pytest.mark.parametrize(
    ("kind", "reason"),
    [
        ("missing", "invalid_invocation_token"),
        ("bad_signature", "invalid_invocation_token"),
        ("alg_hs256", "invalid_invocation_token"),
        ("typ_access", "invalid_invocation_token"),
        ("aud_other_agent", "invalid_invocation_token"),
    ],
)
async def test_trigger_with_a_forged_token(fake: FakeServer, kind: str, reason: str) -> None:
    handler = RecordingHandler()
    adapter = _adapter(fake, handler, f"forged-{kind}")
    async with _running(fake, adapter):
        triggered = await fake.trigger(forged=kind)
        await _until(fake, lambda: fake.run_state(triggered.run_id).state == "failed")
    assert fake.run_state(triggered.run_id).state_reason == reason
    assert handler.count("prepare") == 0
    with pytest.raises(ValueError, match="unknown token kind"):
        await fake.trigger(forged="nonsense")


async def test_trigger_with_an_expired_token(fake: FakeServer) -> None:
    handler = RecordingHandler()
    adapter = _adapter(fake, handler, "expired")
    fake.inject("renewInvocationToken", Fault.problem("agent_invocation_denied"))
    async with _running(fake, adapter):
        triggered = await fake.trigger(forged="expired")
        await _until(fake, lambda: fake.requests[-1].operation_id == "renewInvocationToken")
        await fake.pass_time(0.5)
    assert handler.count("execute") == 0
    assert [r for r in fake.requests if r.operation_id == "startRun"] == []
    assert fake.run_state(triggered.run_id).starts == 0
