# SPDX-License-Identifier: Apache-2.0
"""Outbox: the ID is stored before the send, never twice, FIFO per thread, failures kept.

docs/spec/04-sync-protocol.md 13.2 and Design note 13 of the brief: a reply gets its
``client_msg_id`` and a write its ``Idempotency-Key`` stored before the first send, every
repeat sends the same one; ``400``, ``404``, ``409``, ``410``, ``413`` and ``422`` mark the entry
failed with its problem code; a transient failure leaves it pending for `Outbox.flush`.
"""

from __future__ import annotations

import asyncio
import dataclasses
import json
import os
import random
import stat
from collections.abc import AsyncIterator, Awaitable, Callable
from pathlib import Path
from typing import Any
from uuid import UUID

import httpx
import pytest

from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import IDEMPOTENCY_KEY_HEADER, Client
from bothy_sdk.generated.api.commands import register_command
from bothy_sdk.generated.models import CommandRegister
from bothy_sdk.outbox import (
    PERMANENT_STATUSES,
    MemoryOutboxStore,
    Outbox,
    OutboxEntry,
    OutboxState,
    SqliteOutboxStore,
)
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing import FakeClock
from bothy_sdk.testing._patience import eventually
from bothy_sdk.testing.fakeserver import FakeServer
from bothy_sdk.testing.faults import Fault

pytestmark = pytest.mark.anyio

BASE = "https://example.com/api/v1"
THREAD_A = UUID("01900000-0000-7000-8000-00000000a001")
THREAD_B = UUID("01900000-0000-7000-8000-00000000b001")
RUN = UUID("01900000-0000-7000-8000-000000000101")
CHANNEL = "01900000-0000-7000-8000-000000000103"
AGENT = "01900000-0000-7000-8000-000000000301"
# One attempt per call: transient failures stay pending at once instead of being retried.
ONE_ATTEMPT = SdkSettings(retry_max_attempts=1)

Handler = Callable[[httpx.Request], Awaitable[httpx.Response]]


def seeded(seed: int) -> random.Random:
    return random.Random(seed)  # noqa: S311 - deterministic jitter for the test


def _problem(status: int, code: str) -> httpx.Response:
    return httpx.Response(
        status,
        json={
            "type": "about:blank",
            "title": code,
            "status": status,
            "code": code,
            "correlation_id": "01900000-0000-7000-8000-00000000e001",
        },
        headers={"Content-Type": "application/problem+json"},
    )


def _message(thread_id: str, body: dict[str, Any]) -> dict[str, Any]:
    """A ``Message`` as the server answers ``createMessage``."""
    return {
        "id": "01900000-0000-7000-8000-00000000c001",
        "thread_id": thread_id,
        "channel_id": CHANNEL,
        "thread_seq": 1,
        "author": {"id": AGENT, "kind": "agent", "handle": "lotse", "display_name": "Lotse"},
        "client_msg_id": body["client_msg_id"],
        "origin": "agent",
        "kind": body.get("kind", "text"),
        "content_md": body.get("content_md", ""),
        "content_format": 1,
        "silent": False,
        "attachments": [],
        "mention_ids": [],
        "reactions": [],
        "pinned": False,
        "components": [],
        "version": 1,
        "created_at": "2026-01-01T00:00:00Z",
    }


def _command(body: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": "01900000-0000-7000-8000-00000000d001",
        "owner_principal_id": AGENT,
        "name": body["name"],
        "description": body["description"],
        "scope_kind": "instance",
        "options": [],
        "required_level": "trigger",
        "control": False,
        "version": 1,
    }


def _thread_of(request: httpx.Request) -> str:
    # /api/v1/threads/<thread_id>/messages
    return request.url.path.split("/")[-2]


class _Server:
    """A MockTransport handler of ``createMessage`` and ``registerCommand``.

    ``answers`` per thread (or ``"commands"``) is a queue of statuses; an empty queue answers
    success. Every request is recorded as (thread, client_msg_id or Idempotency-Key).
    """

    def __init__(self) -> None:
        self.answers: dict[str, list[int | str]] = {}
        self.seen: list[tuple[str, str]] = []
        self.gates: dict[str, asyncio.Event] = {}

    async def __call__(self, request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content) if request.content else {}
        if request.url.path.endswith("/messages"):
            target, key = _thread_of(request), str(body["client_msg_id"])
        else:
            target, key = "commands", request.headers.get(IDEMPOTENCY_KEY_HEADER, "")
        self.seen.append((target, key))
        gate = self.gates.get(target)
        if gate is not None:
            await gate.wait()
        queue = self.answers.get(target, [])
        answer = queue.pop(0) if queue else 201
        if answer == "network":
            raise httpx.ConnectError("refused", request=request)
        if answer != 201:
            return _problem(int(answer), _CODES.get(int(answer), "internal"))
        if target == "commands":
            return httpx.Response(201, json=_command(body))
        return httpx.Response(201, json=_message(target, body))

    def keys(self, target: str) -> list[str]:
        return [k for t, k in self.seen if t == str(target)]


_CODES = {
    400: "validation_failed",
    404: "not_found",
    409: "conflict",
    410: "gone",
    413: "payload_too_large",
    422: "unprocessable",
    500: "internal",
    502: "bad_gateway",
    503: "unavailable",
}


def _client(handler: Handler, settings: SdkSettings = ONE_ATTEMPT) -> Client:
    return Client(
        BASE,
        Credentials(Secret("example-api-token")),
        settings=settings,
        clock=FakeClock(),
        rng=seeded(1),
        transport=httpx.MockTransport(handler),
    )


# Store before send -----------------------------------------------------------------------------


async def test_id_stored_before_send() -> None:
    store = MemoryOutboxStore()
    observed: list[tuple[str, list[str]]] = []

    async def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        pending = await store.pending()
        observed.append((str(body["client_msg_id"]), [e.key for e in pending]))
        return httpx.Response(201, json=_message(_thread_of(request), body))

    async with _client(handler) as client:
        outbox = Outbox(client, store)
        entry = await outbox.send_message(THREAD_A, "Hello Alex", run_id=RUN)
    assert entry.state is OutboxState.SENT
    assert observed == [(entry.key, [entry.key])]
    assert UUID(entry.key).version == 7
    assert entry.run_id == RUN
    assert entry.payload["run_id"] == str(RUN)
    assert await store.pending() == []


async def test_reply_members_stored() -> None:
    server = _Server()
    store = MemoryOutboxStore()
    reply_to = UUID("01900000-0000-7000-8000-000000000105")
    async with _client(server) as client:
        entry = await Outbox(client, store).send_message(
            THREAD_A, "Done", reply_to_message_id=reply_to
        )
    assert entry.payload["reply_to_message_id"] == str(reply_to)
    assert entry.payload["content_md"] == "Done"
    assert entry.payload["kind"] == "text"
    assert "run_id" not in entry.payload
    assert entry.run_id is None


# FakeServer faults -----------------------------------------------------------------------------


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    """A started FakeServer with DPoP."""
    async with FakeServer() as server:
        yield server


def _fake_client(fake: FakeServer, **settings: Any) -> Client:
    return Client(
        fake.base_url,
        fake.credentials,
        settings=dataclasses.replace(fake.sdk_settings, **settings),
        clock=fake.clock,
        rng=seeded(2),
    )


async def _drive(fake: FakeServer, task: asyncio.Future[Any], step: float = 0.25) -> None:
    """Move the shared clock until ``task`` is done (retries sleep on the FakeClock)."""
    await eventually(task.done, "the outbox to finish", step=lambda: fake.pass_time(step))


def _message_keys(fake: FakeServer) -> list[str]:
    return [
        str(r.json["client_msg_id"]) for r in fake.requests if r.operation_id == "createMessage"
    ]


async def test_stored_then_502_no_duplicate(fake: FakeServer) -> None:
    triggered = await fake.trigger()
    fake.inject("createMessage", Fault.stored_then_502())
    before = len(fake.messages)
    async with _fake_client(fake, retry_max_attempts=3) as client:
        await client.discover()  # takes the DPoP nonce, so no request meets the challenge
        task = asyncio.ensure_future(
            Outbox(client).send_message(triggered.thread_id, "Reply", run_id=triggered.run_id)
        )
        await _drive(fake, task)
        entry = task.result()
    assert entry.state is OutboxState.SENT
    keys = _message_keys(fake)
    assert keys == [entry.key, entry.key]
    stored = fake.messages[before:]
    assert [m["client_msg_id"] for m in stored] == [entry.key]


async def test_stored_late_no_duplicate(fake: FakeServer) -> None:
    triggered = await fake.trigger()
    delay = 5.0
    fake.inject("createMessage", Fault.stored_late(delay))
    before = len(fake.messages)
    # A short real timeout makes the client give up on the late answer and repeat the send.
    async with _fake_client(fake, retry_max_attempts=3, http_timeout=0.3) as client:
        await client.discover()  # takes the DPoP nonce, so no request meets the challenge
        task = asyncio.ensure_future(Outbox(client).send_message(triggered.thread_id, "Late"))
        # The server waits on its clock; the client times out and sleeps before its retry.
        await eventually(lambda: fake.clock.sleepers >= 2, "the server wait and the retry wait")
        await _drive(fake, task)
        entry = task.result()
        # Let the late request finish its store as well.
        await fake.pass_time(delay)
        await asyncio.sleep(0.05)
        await fake.pass_time(0)
    assert entry.state is OutboxState.SENT
    # At least one repeat after the timeout; a slow machine may time out once more.
    keys = _message_keys(fake)
    assert len(keys) >= 2
    assert set(keys) == {entry.key}
    stored = fake.messages[before:]
    assert [m["client_msg_id"] for m in stored] == [entry.key]


# Order -----------------------------------------------------------------------------------------


async def test_fifo_per_thread() -> None:
    server = _Server()
    server.answers[str(THREAD_A)] = [503, 503]
    async with _client(server) as client:
        outbox = Outbox(client)
        a1 = await outbox.send_message(THREAD_A, "a1")
        assert a1.state is OutboxState.PENDING
        # a1 is tried again first and fails again, so a2 waits behind it and is not sent.
        a2 = await outbox.send_message(THREAD_A, "a2")
        assert a2.state is OutboxState.PENDING
        assert server.keys(str(THREAD_A)) == [a1.key, a1.key]
        # Another thread is not blocked.
        b1 = await outbox.send_message(THREAD_B, "b1")
        assert b1.state is OutboxState.SENT
        await outbox.flush()
        assert server.keys(str(THREAD_A)) == [a1.key, a1.key, a1.key, a2.key]
        assert await outbox.store.pending() == []
        a3 = await outbox.send_message(THREAD_A, "a3")
        assert a3.state is OutboxState.SENT
    assert server.keys(str(THREAD_A))[-1] == a3.key


async def test_threads_flush_in_parallel() -> None:
    server = _Server()
    server.answers[str(THREAD_A)] = ["network"]
    server.answers[str(THREAD_B)] = ["network"]
    async with _client(server) as client:
        outbox = Outbox(client)
        a1 = await outbox.send_message(THREAD_A, "a1")
        b1 = await outbox.send_message(THREAD_B, "b1")
        assert (a1.state, b1.state) == (OutboxState.PENDING, OutboxState.PENDING)
        # Thread A hangs on the server; thread B still goes out during the same flush.
        server.gates[str(THREAD_A)] = asyncio.Event()
        flush = asyncio.ensure_future(outbox.flush())
        await eventually(
            lambda: server.keys(str(THREAD_B)).count(b1.key) == 2, "the second send of thread B"
        )
        assert server.keys(str(THREAD_B)) == [b1.key, b1.key]
        assert not flush.done()
        server.gates[str(THREAD_A)].set()
        await flush
        assert await outbox.store.pending() == []


# Failures --------------------------------------------------------------------------------------


@pytest.mark.parametrize("status", sorted(PERMANENT_STATUSES))
async def test_permanent_failure_kept_failed(status: int) -> None:
    assert {400, 404, 409, 410, 413, 422} == PERMANENT_STATUSES
    server = _Server()
    server.answers[str(THREAD_A)] = [status]
    reported: list[OutboxEntry] = []

    async def on_failed(entry: OutboxEntry) -> None:
        reported.append(entry)

    async with _client(server, SdkSettings(retry_max_attempts=3)) as client:
        outbox = Outbox(client, on_failed=on_failed)
        entry = await outbox.send_message(THREAD_A, "rejected")
        assert entry.state is OutboxState.FAILED
        assert entry.problem_code == _CODES[status]
        assert [e.key for e in reported] == [entry.key]
        assert reported[0].problem_code == _CODES[status]
        failed = await outbox.failed()
        assert [(e.key, e.problem_code) for e in failed] == [(entry.key, _CODES[status])]
        assert await outbox.store.pending() == []
        # Not retried: one request, and a flush sends nothing again.
        await outbox.flush()
        assert server.keys(str(THREAD_A)) == [entry.key]
        # A failed entry does not block the thread.
        later = await outbox.send_message(THREAD_A, "later")
        assert later.state is OutboxState.SENT


async def test_sync_on_failed_called() -> None:
    server = _Server()
    server.answers[str(THREAD_A)] = [422]
    reported: list[str] = []
    async with _client(server) as client:
        outbox = Outbox(client, on_failed=lambda e: reported.append(e.key))
        entry = await outbox.send_message(THREAD_A, "rejected")
    assert reported == [entry.key]


@pytest.mark.parametrize("answer", [500, 502, 503, 429, "network"])
async def test_transient_failure_stays_pending(answer: int | str) -> None:
    server = _Server()
    attempts = 3
    server.answers[str(THREAD_A)] = [answer] * attempts
    clock = FakeClock()
    settings = SdkSettings(retry_max_attempts=attempts)
    async with Client(
        BASE,
        Credentials(Secret("example-api-token")),
        settings=settings,
        clock=clock,
        rng=seeded(3),
        transport=httpx.MockTransport(server),
    ) as client:
        outbox = Outbox(client)
        task = asyncio.ensure_future(outbox.send_message(THREAD_A, "later"))
        await eventually(task.done, "the call", step=lambda: clock.advance(1))
        entry = task.result()
        assert entry.state is OutboxState.PENDING
        assert server.keys(str(THREAD_A)) == [entry.key] * attempts
        assert await outbox.failed() == []
        await outbox.flush()
    assert server.keys(str(THREAD_A)) == [entry.key] * (attempts + 1)


# Restart ---------------------------------------------------------------------------------------


async def test_restart_resends_same_id(tmp_path: Path) -> None:
    path = tmp_path / "outbox.sqlite3"
    server = _Server()
    server.answers[str(THREAD_A)] = [503]
    store = SqliteOutboxStore(path)
    async with _client(server) as client:
        entry = await Outbox(client, store).send_message(THREAD_A, "survives", run_id=RUN)
    assert entry.state is OutboxState.PENDING
    await store.close()
    if os.name == "posix":
        assert stat.S_IMODE(path.stat().st_mode) == 0o600

    reopened = SqliteOutboxStore(path)
    [kept] = await reopened.pending()
    assert (kept.key, kept.thread_id, kept.run_id) == (entry.key, THREAD_A, RUN)
    async with _client(server) as client:
        await Outbox(client, reopened).flush()
    assert server.keys(str(THREAD_A)) == [entry.key, entry.key]
    assert await reopened.pending() == []
    await reopened.close()

    third = SqliteOutboxStore(path)
    assert await third.pending() == []
    assert await third.failed() == []
    await third.close()


async def test_sqlite_failed_entry_survives_reopen(tmp_path: Path) -> None:
    path = tmp_path / "outbox.sqlite3"
    server = _Server()
    server.answers[str(THREAD_A)] = [409]
    store = SqliteOutboxStore(path)
    async with _client(server) as client:
        entry = await Outbox(client, store).send_message(THREAD_A, "conflict")
    await store.close()
    reopened = SqliteOutboxStore(path)
    failed = await reopened.failed()
    await reopened.close()
    assert [(e.key, e.state, e.problem_code) for e in failed] == [
        (entry.key, OutboxState.FAILED, "conflict")
    ]


# Outbox.call -----------------------------------------------------------------------------------


@pytest.mark.parametrize("durable", [False, True])
async def test_call_idempotency_key_stored_and_repeated(tmp_path: Path, durable: bool) -> None:
    server = _Server()
    server.answers["commands"] = [503]
    store = SqliteOutboxStore(tmp_path / "outbox.sqlite3") if durable else MemoryOutboxStore()
    body = CommandRegister(name="status", description="Show the run status")
    async with _client(server) as client:
        outbox = Outbox(client, store)
        entry = await outbox.call(register_command.asyncio_detailed, thread_id=THREAD_A, body=body)
        assert entry.state is OutboxState.PENDING
        assert UUID(entry.key).version == 7
        [stored] = await store.pending()
        assert stored.key == entry.key
        assert stored.kind == "call"
        await outbox.flush()
    assert server.keys("commands") == [entry.key, entry.key]
    assert await store.pending() == []
    await store.close()


async def test_call_key_repeated_across_retries_of_one_attempt() -> None:
    server = _Server()
    server.answers["commands"] = [502, 503]
    clock = FakeClock()
    async with Client(
        BASE,
        Credentials(Secret("example-api-token")),
        settings=SdkSettings(retry_max_attempts=3),
        clock=clock,
        rng=seeded(4),
        transport=httpx.MockTransport(server),
    ) as client:
        outbox = Outbox(client)
        task = asyncio.ensure_future(
            outbox.call(
                register_command.asyncio_detailed,
                thread_id=THREAD_A,
                body=CommandRegister(name="status", description="Show the run status"),
            )
        )
        await eventually(task.done, "the call", step=lambda: clock.advance(1))
        entry = task.result()
    assert entry.state is OutboxState.SENT
    assert server.keys("commands") == [entry.key] * 3


async def test_call_refuses_unstorable_argument() -> None:
    server = _Server()
    async with _client(server) as client:
        with pytest.raises(TypeError, match="cannot store"):
            await Outbox(client).call(
                register_command.asyncio_detailed, thread_id=THREAD_A, body=object()
            )
    assert server.seen == []


async def test_run_entries_go_out_bound_to_their_run() -> None:
    run_a = UUID("01900000-0000-7000-8000-0000000000aa")
    run_b = UUID("01900000-0000-7000-8000-0000000000bb")
    seen: list[str | None] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request.headers.get("Invocation-Token"))
        body = json.loads(request.content)
        return httpx.Response(201, json=_message(_thread_of(request), body))

    tokens = {run_a: Secret("token-of-a")}

    async def token_for(run_id: UUID) -> Secret | None:
        return tokens.get(run_id)

    failed: list[str | None] = []
    async with Client(
        BASE, Credentials(Secret("api")), transport=httpx.MockTransport(handler)
    ) as c:
        outbox = Outbox(c, token_for=token_for, on_failed=lambda e: failed.append(e.problem_code))
        tokens[run_b] = Secret("token-of-b")
        with c.bind_run(run_b, Secret("token-of-b")):
            # Inside run B: an entry of run A carries A's token; a write into another thread
            # without run_id still carries B's token (09 4.7: every request inside a run).
            await outbox.send_message(THREAD_A, "for A", run_id=run_a)
            other = await outbox.send_message(THREAD_B, "into another thread")
        # Outside any run: no header.
        await outbox.send_message(THREAD_B, "outside")
        del tokens[run_b]
        entry = await outbox.send_message(THREAD_B, "for B", run_id=run_b)
    assert seen == ["token-of-a", "token-of-b", None]
    assert other.run_id == run_b
    assert entry.state.value == "failed"
    assert failed == ["run_token_unavailable"]  # never sent unbound
