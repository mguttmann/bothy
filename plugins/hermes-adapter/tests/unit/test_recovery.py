# SPDX-License-Identifier: Apache-2.0
"""Replies that Hermes' delivery ledger sends again are not posted twice (HERMES-E2E-REAL-SERVER).

Hermes records every final answer in its delivery ledger and sends it again, behind a notice
"Recovered reply ... may be a duplicate", when its send failed or the gateway restarted during
the send. The adapter's outbox already owns the delivery of every answer it took over (and a
refused answer ends its run with the error), so such a send must not become a second message.
"""

from __future__ import annotations

from pathlib import Path
from uuid import UUID

import pytest
from adapter_kit import rig

from bothy_hermes_adapter.reports import recovered_reply
from bothy_hermes_adapter.store import ANSWER_RETENTION_SECONDS, CallbackStore

pytestmark = pytest.mark.anyio

# The three notices of the pinned Hermes (gateway/delivery_ledger.py). The recycling sign and the
# dash are built from their code points: the repository holds no dash character.
_HEAD = chr(0x267B) + chr(0xFE0F) + " Recovered reply " + chr(0x2014) + " "
RESTARTED = _HEAD + "the gateway restarted during delivery, so this may be a duplicate:\n\n"
RECONNECTED = (
    _HEAD + "the messaging platform reconnected after the original delivery failed, so this may "
    "be a duplicate:\n\n"
)
FLOOD = (
    _HEAD + "the messaging platform's rate limit refused the original, so part of it may already "
    "have arrived above:\n\n"
)
THREAD = UUID("0192f4a0-7b1c-7000-8000-0000000000aa")


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.mark.parametrize("notice", [RESTARTED, RECONNECTED, FLOOD])
def test_recovered_reply_strips_each_notice(notice: str) -> None:
    assert recovered_reply(notice + "Done.\n\nSecond paragraph:\n\nx") == (
        "Done.\n\nSecond paragraph:\n\nx"
    )


@pytest.mark.parametrize(
    "content",
    [
        "Done.",
        "Recovered reply: not from Hermes",
        chr(0x267B) + chr(0xFE0F) + " Recovered reply without the end of a notice",
        "x" + RESTARTED + "Done.",
    ],
)
def test_other_texts_are_no_recovered_reply(content: str) -> None:
    assert recovered_reply(content) is None


async def test_store_keeps_answer_digests_for_the_retention_only(tmp_path: Path) -> None:
    store = CallbackStore(tmp_path / "hermes.sqlite3")
    try:
        await store.put_answer(THREAD, "Old answer.", now=1000.0)
        assert await store.answered(THREAD, "Old answer.")
        assert not await store.answered(THREAD, "Other answer.")
        assert not await store.answered(UUID(int=7), "Old answer.")
        await store.put_answer(THREAD, "New answer.", now=1000.0 + ANSWER_RETENTION_SECONDS + 1)
        assert not await store.answered(THREAD, "Old answer.")
        assert await store.answered(THREAD, "New answer.")
    finally:
        await store.close()
    raw = (tmp_path / "hermes.sqlite3").read_bytes()
    assert b"New answer." not in raw  # a digest, never the text


async def test_a_redelivered_answer_is_recognised_and_nothing_is_posted(tmp_path: Path) -> None:
    async with rig(tmp_path) as r:
        t = await r.fake.trigger("Hello @Lotse")
        await r.until(lambda: r.finished(t.run_id), "the end of the run")
        posted = len(r.requests("createMessage"))
        handler = r.handler
        assert await handler.recovered_duplicate(t.thread_id, RESTARTED + "Done so far.")
        assert await handler.recovered_duplicate(t.thread_id, FLOOD + "Done so far.")
        # The plain text is no redelivery; another text or thread was never answered.
        assert not await handler.recovered_duplicate(t.thread_id, "Done so far.")
        assert not await handler.recovered_duplicate(t.thread_id, RESTARTED + "Something else.")
        assert not await handler.recovered_duplicate(UUID(int=5), RESTARTED + "Done so far.")
        assert not await handler.recovered_duplicate(None, RESTARTED + "Done so far.")
        assert len(r.requests("createMessage")) == posted


async def test_a_refused_answer_is_recorded_too(tmp_path: Path) -> None:
    from bothy_sdk.testing.faults import Fault

    async with rig(tmp_path) as r:
        r.fake.inject("createMessage", Fault.problem("validation_failed"))
        t = await r.fake.trigger("Hello @Lotse")
        await r.until(lambda: r.finished(t.run_id), "the end of the run")
        # Hermes' ledger sends the refused answer again later: the adapter does not post it.
        assert await r.handler.recovered_duplicate(t.thread_id, RECONNECTED + "Done so far.")
