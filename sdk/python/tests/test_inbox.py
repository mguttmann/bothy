# SPDX-License-Identifier: Apache-2.0
"""The inbox stores of Design note 9: durability, token replacement by iat and parity.

Every behaviour test runs against `SqliteInboxStore` and `MemoryInboxStore`. Tokens are compared
as `Secret` values or by `fingerprint`, never printed.
"""

from __future__ import annotations

import os
import stat
from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest

from bothy_sdk._store import Db, open_db
from bothy_sdk.auth import Secret
from bothy_sdk.ids import uuid7
from bothy_sdk.inbox import EntryState, InboxEntry, InboxStore, MemoryInboxStore, SqliteInboxStore
from bothy_sdk.testing import fingerprint

NOW = datetime(2026, 10, 20, 9, 0, tzinfo=UTC)


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture(params=["sqlite", "memory"])
async def store(request: pytest.FixtureRequest, tmp_path: Path) -> AsyncIterator[InboxStore]:
    s: InboxStore = (
        SqliteInboxStore(tmp_path / "inbox.sqlite3")
        if request.param == "sqlite"
        else MemoryInboxStore()
    )
    try:
        yield s
    finally:
        await s.close()


def entry(
    *,
    seq: int | None = 7,
    run_id: UUID | None = None,
    tok: str | None = "token-a",
    iat: int | None = 100,
    state: EntryState = EntryState.ACCEPTED,
    envelope: dict[str, Any] | None = None,
) -> InboxEntry:
    event_id = uuid7()
    return InboxEntry(
        event_id=event_id,
        seq=seq,
        type="agent.invocation",
        run_id=run_id if run_id is not None else uuid7(),
        envelope=envelope if envelope is not None else {"id": str(event_id), "seq": seq},
        token=Secret(tok) if tok else None,
        token_iat=iat,
        token_jti="jti-" + tok.removeprefix("token-") if tok else None,
        state=state,
        deliveries=1,
    )


def _mode(path: Path) -> int:
    return stat.S_IMODE(path.stat().st_mode)


@pytest.mark.anyio
async def test_store_survives_reopen_with_mode_0600(tmp_path: Path) -> None:
    path = tmp_path / "inbox.sqlite3"
    old_umask = os.umask(0)
    try:
        s = SqliteInboxStore(path)
        e = entry(envelope={"id": "x", "payload": {"run_id": "r", "nested": [1, "two"]}})
        await s.add(e)
        await s.remember_jti("jti-seen", NOW + timedelta(seconds=90))
        await s.set_state(e.event_id, EntryState.STARTED)
        await s.close()
    finally:
        os.umask(old_umask)
    # The database and every file SQLite keeps next to it are readable by the owner only.
    files = [p for p in tmp_path.iterdir() if p.name.startswith("inbox.sqlite3")]
    assert files
    for p in files:
        assert _mode(p) == 0o600, p.name

    s = SqliteInboxStore(path)
    try:
        got = await s.get(e.event_id)
        assert got is not None
        assert got.state is EntryState.STARTED
        assert got.token is not None
        assert fingerprint(got.token) == fingerprint(e.token)
        assert (got.token_iat, got.token_jti, got.deliveries, got.seq) == (100, "jti-a", 1, 7)
        assert got.envelope == e.envelope
        assert await s.jti_seen("jti-seen", NOW)
        assert not await s.jti_seen("jti-other", NOW)
    finally:
        await s.close()


def test_open_db_refuses_wider_mode_and_symlink(tmp_path: Path) -> None:
    wide = tmp_path / "wide.sqlite3"
    wide.touch(mode=0o644)
    wide.chmod(0o644)
    with pytest.raises(PermissionError):
        open_db(wide)
    target = tmp_path / "target.sqlite3"
    open_db(target).close()
    link = tmp_path / "link.sqlite3"
    link.symlink_to(target)
    with pytest.raises(OSError, match=r"symbolic link|regular file|Too many levels"):
        open_db(link)


@pytest.mark.anyio
async def test_db_pragmas_and_rollback(tmp_path: Path) -> None:
    db = Db(tmp_path / "db.sqlite3")
    try:
        sync = await db.run(lambda c: c.execute("PRAGMA synchronous").fetchone()[0])
        assert sync == 2  # FULL
        await db.run(lambda c: c.execute("CREATE TABLE t (v INTEGER)"))

        def fail(c: Any) -> None:
            c.execute("INSERT INTO t VALUES (1)")
            raise RuntimeError("boom")

        with pytest.raises(RuntimeError):
            await db.run(fail)
        assert await db.run(lambda c: c.execute("SELECT count(*) FROM t").fetchone()[0]) == 0
    finally:
        await db.close()


@pytest.mark.anyio
async def test_redelivery_replaces_token_by_iat(store: InboxStore) -> None:
    e = entry(tok="token-a", iat=100)
    await store.add(e)
    got = await store.redelivered(e.event_id, Secret("token-b"), 110, "jti-b")
    assert (got.token_jti, got.token_iat, got.deliveries) == ("jti-b", 110, 2)
    assert got.token == Secret("token-b")
    # An equal iat takes the later arrival.
    got = await store.redelivered(e.event_id, Secret("token-c"), 110, "jti-c")
    assert (got.token_jti, got.token_iat, got.deliveries) == ("jti-c", 110, 3)
    stored = await store.get(e.event_id)
    assert stored == got
    assert stored is not None
    assert stored.state is EntryState.ACCEPTED


@pytest.mark.anyio
async def test_older_token_kept(store: InboxStore) -> None:
    e = entry(tok="token-a", iat=100)
    await store.add(e)
    got = await store.redelivered(e.event_id, Secret("token-old"), 90, "jti-old")
    assert (got.token_jti, got.token_iat, got.deliveries) == ("jti-a", 100, 2)
    assert got.token == Secret("token-a")
    # A redelivery without token only counts.
    got = await store.redelivered(e.event_id, None, None, None)
    assert (got.token_jti, got.token_iat, got.deliveries) == ("jti-a", 100, 3)


@pytest.mark.anyio
async def test_duplicate_of_started_entry_only_acked(store: InboxStore) -> None:
    for state in (EntryState.STARTED, EntryState.WAITING, EntryState.CLOSED):
        e = entry(tok="token-a", iat=100)
        await store.add(e)
        await store.set_state(e.event_id, state)
        got = await store.redelivered(e.event_id, Secret("token-new"), 200, "jti-new")
        assert (got.state, got.token_jti, got.token_iat, got.deliveries) == (
            state,
            "jti-a",
            100,
            2,
        ), state
        assert got.token == Secret("token-a")


@pytest.mark.anyio
async def test_replace_token_unconditional(store: InboxStore) -> None:
    e = entry(tok="token-a", iat=100)
    await store.add(e)
    await store.set_state(e.event_id, EntryState.STARTED)
    await store.replace_token(e.event_id, Secret("token-grant"), 50, "jti-grant")
    got = await store.get(e.event_id)
    assert got is not None
    assert (got.token_jti, got.token_iat, got.deliveries, got.state) == (
        "jti-grant",
        50,
        1,
        EntryState.STARTED,
    )
    assert got.token == Secret("token-grant")


@pytest.mark.anyio
async def test_unknown_and_duplicate_ids(store: InboxStore) -> None:
    e = entry()
    assert await store.get(e.event_id) is None
    with pytest.raises(KeyError):
        await store.redelivered(e.event_id, None, None, None)
    with pytest.raises(KeyError):
        await store.set_state(e.event_id, EntryState.CLOSED)
    with pytest.raises(KeyError):
        await store.replace_token(e.event_id, Secret("t"), 1, "j")
    await store.add(e)
    with pytest.raises(ValueError, match="already"):
        await store.add(e)
    with pytest.raises(ValueError, match="invocation_token"):
        await store.add(entry(envelope={"invocation_token": "x"}))


@pytest.mark.anyio
async def test_for_run_and_in_states(store: InboxStore) -> None:
    run = uuid7()
    later = entry(seq=9, run_id=run)
    first = entry(seq=3, run_id=run)
    other = entry(seq=5)
    ephemeral = entry(seq=None, run_id=run, tok=None, iat=None)
    for e in (later, first, other, ephemeral):
        await store.add(e)
    assert [e.event_id for e in await store.for_run(run)] == [
        first.event_id,
        later.event_id,
        ephemeral.event_id,
    ]
    await store.set_state(first.event_id, EntryState.STARTED)
    await store.set_state(other.event_id, EntryState.WAITING)
    got = await store.in_states(EntryState.STARTED, EntryState.WAITING)
    assert [e.event_id for e in got] == [first.event_id, other.event_id]
    assert [e.event_id for e in await store.in_states(EntryState.CLOSED)] == []
    assert len(await store.in_states(EntryState.ACCEPTED)) == 2


@pytest.mark.anyio
async def test_jti_store_expiry_and_prune(store: InboxStore) -> None:
    e = entry()
    await store.add(e)
    await store.remember_jti("j1", NOW + timedelta(seconds=10))
    await store.remember_jti("j2", NOW + timedelta(seconds=100))
    assert await store.jti_seen("j1", NOW)
    assert await store.jti_seen("j1", NOW + timedelta(seconds=9))
    # Seen until (not at) its end.
    assert not await store.jti_seen("j1", NOW + timedelta(seconds=10))
    await store.prune(NOW + timedelta(seconds=50))
    assert not await store.jti_seen("j1", NOW)
    assert await store.jti_seen("j2", NOW + timedelta(seconds=50))
    # Prune drops jti rows only, never inbox entries.
    assert await store.get(e.event_id) is not None
    # Remembering again keeps the later end.
    await store.remember_jti("j2", NOW + timedelta(seconds=20))
    assert await store.jti_seen("j2", NOW + timedelta(seconds=60))


def test_entry_repr_hides_token_and_content() -> None:
    e = entry(tok="token-canary", envelope={"payload": {"text": "content-canary"}})
    text = repr(e) + str(e)
    assert "token-canary" not in text
    assert "content-canary" not in text
    assert str(e.event_id) in text
