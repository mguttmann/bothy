# SPDX-License-Identifier: Apache-2.0
"""FakeServer routes for uploads and attachments, `upload_part` and the outbox member (M3-12).

``getInstance``, ``createUpload``, ``uploadPart``, ``getUpload``, ``completeUpload``,
``abortUpload`` and ``createMessage`` with ``attachment_file_ids``. Every answer goes through the
generated core of docs/api/openapi.yaml (`Client.call_as` parses it into the generated model),
so a FakeServer answer outside the contract fails here.
"""

from __future__ import annotations

import asyncio
import hashlib
from collections.abc import AsyncIterator
from uuid import UUID

import pytest

from bothy_sdk.client import Client
from bothy_sdk.errors import ConflictError, GoneError, ProblemError, ValidationFailed
from bothy_sdk.generated.api.admin import get_instance
from bothy_sdk.generated.api.files import (
    abort_upload,
    complete_upload,
    create_upload,
    get_upload,
)
from bothy_sdk.generated.models import (
    File,
    InstanceInfo,
    UploadComplete,
    UploadCreate,
    UploadPartResult,
    UploadSession,
    UploadState,
)
from bothy_sdk.outbox import Outbox, OutboxState
from bothy_sdk.testing.fakeserver import FakeServer
from bothy_sdk.testing.faults import Fault, HoldGate
from bothy_sdk.uploads import content_digest, upload_part

pytestmark = pytest.mark.anyio

DATA = b"0123456789abcdefghij-0123456789"  # 31 bytes: three parts of 12, 12 and 7


@pytest.fixture
def anyio_backend() -> str:
    """The FakeServer runs on asyncio."""
    return "asyncio"


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer(dpop=False) as server:
        server.uploads.part_size = 12
        yield server


@pytest.fixture
async def client(fake: FakeServer) -> AsyncIterator[Client]:
    async with Client(
        fake.base_url, fake.credentials, settings=fake.sdk_settings, clock=fake.clock
    ) as c:
        yield c


async def _thread(fake: FakeServer) -> tuple[UUID, UUID]:
    """A thread of a served forum: its ID and its channel."""
    forum = fake.add_forum("Projekt-A", "website")
    triggered = await fake.trigger("Hello @Lotse", channel_id=forum)
    return triggered.thread_id, forum


async def _create(
    client: Client, channel: UUID, data: bytes = DATA, key: str = ""
) -> UploadSession:
    body = UploadCreate(
        name="report.md", size_bytes=len(data), sha256=hashlib.sha256(data).hexdigest()
    )
    if key:
        with client.idempotency_key(key):
            return await client.call_as(
                UploadSession, create_upload.asyncio_detailed, channel_id=channel, body=body
            )
    return await client.call_as(
        UploadSession, create_upload.asyncio_detailed, channel_id=channel, body=body
    )


async def _upload(client: Client, session: UploadSession, data: bytes = DATA) -> File:
    size = session.part_size_bytes
    etags = []
    for part in session.parts:
        start = (part.part_number - 1) * size
        headers = {h.name: h.value for h in part.headers}
        result = await upload_part(
            client, session.file_id, part.part_number, data[start : start + size], headers
        )
        etags.append(result)
    return await client.call_as(
        File,
        complete_upload.asyncio_detailed,
        file_id=session.file_id,
        body=UploadComplete(parts=etags),
    )


async def test_get_instance_limits(fake: FakeServer, client: Client) -> None:
    fake.uploads.limits["message_max_attachments"] = 3
    fake.uploads.limits["file_max_upload_bytes"] = 64
    info = await client.call_as(InstanceInfo, get_instance.asyncio_detailed)
    assert info.limits.message_max_attachments == 3
    assert info.limits.file_max_upload_bytes == 64


async def test_upload_session_parts_and_complete(fake: FakeServer, client: Client) -> None:
    _, channel = await _thread(fake)
    session = await _create(client, channel)
    assert session.mode.value == "direct"
    assert [p.part_number for p in session.parts] == [1, 2, 3]
    file = await _upload(client, session)
    assert file.upload_state.value == "complete"
    assert fake.uploads.sessions[session.file_id].content() == DATA
    puts = [r for r in fake.requests if r.operation_id == "uploadPart"]
    assert [r.content for r in puts] == [DATA[:12], DATA[12:24], DATA[24:]]
    assert all(r.json is None for r in puts)
    assert puts[0].headers["content-digest"] == content_digest(DATA[:12])
    assert puts[0].headers["content-type"] == "application/octet-stream"
    state = await client.call_as(UploadState, get_upload.asyncio_detailed, file_id=session.file_id)
    assert state.upload_state.value == "complete"
    # A completed session cannot be aborted, and completing again answers the file.
    with pytest.raises(ConflictError) as err:
        await client.call(abort_upload.asyncio_detailed, file_id=session.file_id)
    assert err.value.code == "invalid_state_transition"
    again = await client.call_as(
        File,
        complete_upload.asyncio_detailed,
        file_id=session.file_id,
        body=UploadComplete(parts=[]),
    )
    assert again.id == session.file_id


async def test_parts_idempotent_digest_checked(fake: FakeServer, client: Client) -> None:
    _, channel = await _thread(fake)
    session = await _create(client, channel)
    first = await upload_part(client, session.file_id, 1, DATA[:12])
    again = await upload_part(client, session.file_id, 1, DATA[:12])
    assert again.etag == first.etag
    with pytest.raises(ValidationFailed) as err:
        await upload_part(client, session.file_id, 2, DATA[12:20])  # wrong length
    assert err.value.errors[0].code == "part_digest_mismatch"
    with pytest.raises(ConflictError) as conflict:
        await upload_part(client, session.file_id, 1, b"X" * 12)
    assert conflict.value.reason == "part_conflict"
    state = await client.call_as(UploadState, get_upload.asyncio_detailed, file_id=session.file_id)
    assert state.upload_state.value == "uploading"
    assert [p.part_number for p in state.completed_parts] == [1]
    with pytest.raises(ConflictError) as missing:
        await client.call(
            complete_upload.asyncio_detailed,
            file_id=session.file_id,
            body=UploadComplete(parts=[UploadPartResult(part_number=1, etag=first.etag)]),
        )
    assert missing.value.reason == "upload_incomplete"
    await client.call(abort_upload.asyncio_detailed, file_id=session.file_id)
    with pytest.raises(GoneError):
        await client.call(get_upload.asyncio_detailed, file_id=session.file_id)


async def test_create_upload_replays_stored_key(fake: FakeServer, client: Client) -> None:
    _, channel = await _thread(fake)
    key = "0192d3a0-0000-7000-8000-0000000000c1"
    first = await _create(client, channel, key=key)
    second = await _create(client, channel, key=key)
    assert second.file_id == first.file_id
    assert len(fake.uploads.sessions) == 1
    with pytest.raises(ProblemError) as err:
        await _create(client, channel, data=b"other", key=key)
    assert err.value.code == "idempotency_key_reuse"


async def test_create_message_attaches_completed_uploads(fake: FakeServer, client: Client) -> None:
    thread, channel = await _thread(fake)
    a = await _upload(client, await _create(client, channel))
    b = await _upload(client, await _create(client, channel, data=b"second file"), b"second file")
    outbox = Outbox(client)
    entry = await outbox.send_message(thread, "", attachment_file_ids=[b.id, a.id])
    assert entry.state is OutboxState.SENT
    assert entry.created is not None
    assert [x.file.id for x in entry.created.attachments] == [b.id, a.id]
    assert [x.position for x in entry.created.attachments] == [0, 1]
    # An attached file is not attachable again; neither is an open or foreign one.
    again = await outbox.send_message(thread, "again", attachment_file_ids=[a.id])
    assert again.state is OutboxState.FAILED
    assert again.problem is not None
    assert (again.problem.code, again.problem.reason) == ("conflict", "file_not_attachable")
    open_one = await _create(client, channel)
    pending = await outbox.send_message(thread, "x", attachment_file_ids=[open_one.file_id])
    assert pending.problem is not None
    assert pending.problem.reason == "file_not_attachable"
    other_thread, _ = await _thread(fake)
    elsewhere = await _upload(
        client, await _create(client, channel, data=b"third file"), b"third file"
    )
    foreign = fake.add_forum("Projekt-B", "docs")
    triggered = await fake.trigger("Hi", channel_id=foreign)
    moved = await outbox.send_message(triggered.thread_id, "x", attachment_file_ids=[elsewhere.id])
    assert moved.problem is not None
    assert moved.problem.reason == "file_not_attachable"
    assert other_thread != thread
    fake.uploads.limits["message_max_attachments"] = 0
    too_many = await outbox.send_message(thread, "x", attachment_file_ids=[elsewhere.id])
    assert too_many.problem is not None
    assert too_many.problem.code == "validation_failed"


async def test_outbox_attachments_refused_with_403_fail_at_once(
    fake: FakeServer, client: Client
) -> None:
    thread, channel = await _thread(fake)
    file = await _upload(client, await _create(client, channel))
    outbox = Outbox(client)
    fake.inject("createMessage", Fault.problem("feature_disabled"))
    refused = await outbox.send_message(thread, "with", attachment_file_ids=[file.id])
    assert refused.state is OutboxState.FAILED
    assert refused.problem is not None
    assert refused.problem.code == "feature_disabled"
    assert await outbox.store.pending() == []
    # Without attachments a 403 stays pending as before (unchanged behaviour).
    fake.inject("createMessage", Fault.problem("forbidden", reason="capability_missing"))
    kept = await outbox.send_message(thread, "plain")
    assert kept.state is OutboxState.PENDING


async def test_passthrough_and_hold_faults(fake: FakeServer, client: Client) -> None:
    _, channel = await _thread(fake)
    fake.inject("createUpload", Fault.passthrough())
    fake.inject("createUpload", Fault.problem("conflict", reason="quota_exceeded"))
    await _create(client, channel)
    with pytest.raises(ConflictError) as err:
        await _create(client, channel)
    assert err.value.reason == "quota_exceeded"
    gate = HoldGate()
    fake.inject("getInstance", Fault.hold(gate))
    call = asyncio.ensure_future(client.call_as(InstanceInfo, get_instance.asyncio_detailed))
    await gate.reached.wait()
    assert not call.done()
    gate.open()
    assert isinstance(await call, InstanceInfo)
