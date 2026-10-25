# SPDX-License-Identifier: Apache-2.0
"""Contract test of the port `Store` (brief M2-50 Design note 9, Tests bullet ``test_store.py``).

The same contract runs against `SqliteStore` and `MemoryStore`: the thread map in both
directions, drafts, created objects, seen identifiers (``body_digest`` and ``webhook_id``),
the FIFO order of jobs, pruning after ``jira_app.dedupe_window`` with a fake clock (seen
identifiers, created objects and drafts go, the thread map stays) and deleted finished jobs.
For `SqliteStore` alone: commits survive a reopen, the state directory has mode ``0700`` and
the store's files ``0600`` (set by the store itself, also under a permissive umask), and a
second process holding the ``flock`` of the lock file makes the start fail with
`StoreLockedError` naming the directory.
"""

from __future__ import annotations

import os
import signal
import stat
import subprocess
import sys
from collections.abc import AsyncIterator, Callable
from pathlib import Path

import pytest

from jira_app.config import check_state_dir
from jira_app.store.base import (
    IssueDraft,
    MappedIssue,
    NewJob,
    SeenKey,
    StoreError,
    StoreLockedError,
    ThreadRef,
)
from jira_app.store.memory_store import MemoryStore
from jira_app.store.sqlite_store import DATABASE_FILE, LOCK_FILE, SqliteStore, SqlStore

pytestmark = pytest.mark.anyio

WINDOW = 600.0
START = 1_700_000_000.0
MANY = 100


class FakeClock:
    """A wall clock the test moves by hand."""

    def __init__(self) -> None:
        self.t = START

    def __call__(self) -> float:
        return self.t


StoreFactory = Callable[[Path, FakeClock], SqlStore]


def _sqlite(directory: Path, clock: FakeClock) -> SqlStore:
    return SqliteStore(check_state_dir(str(directory / "state")), now=clock, window=WINDOW)


def _memory(directory: Path, clock: FakeClock) -> SqlStore:
    del directory
    return MemoryStore(now=clock, window=WINDOW)


@pytest.fixture
def clock() -> FakeClock:
    return FakeClock()


@pytest.fixture(params=[_sqlite, _memory], ids=["sqlite", "memory"])
async def store(
    request: pytest.FixtureRequest, tmp_path: Path, clock: FakeClock
) -> AsyncIterator[SqlStore]:
    factory: StoreFactory = request.param
    s = factory(tmp_path, clock)
    await s.open()
    try:
        yield s
    finally:
        await s.close()


def _draft(draft_id: str, message_id: str, created_at: float) -> IssueDraft:
    return IssueDraft(
        draft_id=draft_id,
        thread_id="thread-" + draft_id,
        channel_id="forum-1",
        card_message_id=message_id,
        title="Login fails for Alex",
        projects=("SUP", "WEB"),
        created_at=created_at,
    )


# Contract: both forms.


async def test_thread_map_both_directions(store: SqlStore) -> None:
    assert await store.thread_of_issue("10001") is None
    assert await store.issue_of_thread("t-1") is None
    await store.map_thread("10001", "SUP-12", ThreadRef("t-1", "forum-1"))
    assert await store.thread_of_issue("10001") == ThreadRef("t-1", "forum-1")
    assert await store.issue_of_thread("t-1") == MappedIssue("10001", "SUP-12")

    await store.set_issue_key("10001", "WEB-3")
    assert await store.issue_of_thread("t-1") == MappedIssue("10001", "WEB-3")

    # A new map of the same issue replaces the older entry in both directions.
    await store.map_thread("10001", "WEB-3", ThreadRef("t-2", "forum-2"))
    assert await store.thread_of_issue("10001") == ThreadRef("t-2", "forum-2")
    assert await store.issue_of_thread("t-1") is None
    assert await store.issue_of_thread("t-2") == MappedIssue("10001", "WEB-3")


async def test_unmap_only_when_the_entry_still_points_to_the_thread(store: SqlStore) -> None:
    await store.map_thread("10001", "SUP-12", ThreadRef("t-1", "forum-1"))
    await store.unmap_thread("10001", "t-other")
    assert await store.thread_of_issue("10001") == ThreadRef("t-1", "forum-1")
    await store.unmap_thread("10001", "t-1")
    assert await store.thread_of_issue("10001") is None
    assert await store.issue_of_thread("t-1") is None


async def test_drafts(store: SqlStore, clock: FakeClock) -> None:
    draft = _draft("d-1", "m-1", clock())
    await store.put_draft(draft)
    assert await store.get_draft("d-1") == draft
    assert await store.draft_by_message("m-1") == draft
    assert await store.get_draft("d-2") is None
    assert await store.draft_by_message("m-2") is None

    moved = _draft("d-1", "m-2", clock())
    await store.put_draft(moved)
    assert await store.get_draft("d-1") == moved
    assert await store.draft_by_message("m-1") is None
    assert await store.draft_by_message("m-2") == moved

    await store.delete_draft("d-1")
    assert await store.get_draft("d-1") is None
    assert await store.draft_by_message("m-2") is None


async def test_created_objects(store: SqlStore) -> None:
    assert not await store.is_created("issue", "10001")
    await store.record_created("issue", "10001")
    await store.record_created("comment", "20001")
    assert await store.is_created("issue", "10001")
    assert await store.is_created("comment", "20001")
    assert not await store.is_created("comment", "10001")
    assert not await store.is_created("issue", "20001")


@pytest.mark.parametrize("kind", ["body_digest", "webhook_id"])
async def test_seen_identifiers(store: SqlStore, kind: str) -> None:
    key = SeenKey(kind, "id-1")  # type: ignore[arg-type]
    assert await store.seen_status(key) is None
    first = await store.admit(key, 202, NewJob("jira_event", {"n": 1}), MANY)
    assert first.kind == "accepted"
    assert first.status == 202
    assert first.job_id > 0
    assert await store.seen_status(key) == 202

    again = await store.admit(key, 204, NewJob("jira_event", {"n": 2}), MANY)
    assert again.kind == "duplicate"
    assert again.status == 202
    assert again.job_id == 0
    assert await store.count_jobs() == 1

    other_kind = "webhook_id" if kind == "body_digest" else "body_digest"
    assert await store.seen_status(SeenKey(other_kind, "id-1")) is None  # type: ignore[arg-type]


async def test_admit_without_job_stores_only_the_identifier(store: SqlStore) -> None:
    result = await store.admit(SeenKey("body_digest", "d-1"), 204, None, MANY)
    assert result.kind == "accepted"
    assert result.status == 204
    assert result.job_id == 0
    assert await store.next_job() is None
    assert await store.seen_status(SeenKey("body_digest", "d-1")) == 204


async def test_full_queue_stores_nothing(store: SqlStore) -> None:
    await store.admit(SeenKey("webhook_id", "w-1"), 202, NewJob("mirror", {}), 1)
    full = await store.admit(SeenKey("webhook_id", "w-2"), 202, NewJob("mirror", {}), 1)
    assert full.kind == "full"
    assert full.status == 503
    assert await store.seen_status(SeenKey("webhook_id", "w-2")) is None
    assert await store.count_jobs() == 1


async def test_jobs_fifo_and_finished_jobs_deleted(store: SqlStore, clock: FakeClock) -> None:
    ids = []
    for n in range(5):
        result = await store.admit(
            SeenKey("body_digest", f"b-{n}"), 202, NewJob("jira_event", {"n": n}), MANY
        )
        ids.append(result.job_id)
    assert ids == sorted(ids)
    assert len(set(ids)) == 5

    job = await store.next_job()
    assert job is not None
    job.progress = {"step": "attempted", "client_msg_id": "c-1"}
    await store.save_progress(job)
    again = await store.next_job()
    assert again is not None
    assert again.id == ids[0]
    assert again.progress == {"step": "attempted", "client_msg_id": "c-1"}
    assert again.created_at == clock()

    order = []
    while (current := await store.next_job()) is not None:
        order.append((current.id, current.kind, current.data["n"]))
        await store.remove_job(current.id)
    assert order == [(ids[n], "jira_event", n) for n in range(5)]
    assert await store.count_jobs() == 0


async def test_pruning_after_the_dedupe_window(store: SqlStore, clock: FakeClock) -> None:
    key = SeenKey("webhook_id", "w-old")
    digest = SeenKey("body_digest", "b-old")
    await store.admit(key, 202, NewJob("issue_form", {"draft": "d-old"}), MANY)
    await store.admit(digest, 204, None, MANY)
    await store.record_created("issue", "10001")
    await store.put_draft(_draft("d-old", "m-old", clock()))
    await store.map_thread("10001", "SUP-12", ThreadRef("t-1", "forum-1"))

    # Still inside the window: everything is there.
    clock.t = START + WINDOW
    assert await store.seen_status(key) == 202
    assert await store.is_created("issue", "10001")
    assert await store.get_draft("d-old") is not None

    # Past the window, a write prunes the old rows.
    clock.t = START + WINDOW + 1
    await store.record_created("comment", "20001")

    # Back to the start: rows that were only hidden by the window would show again.
    clock.t = START
    assert await store.seen_status(key) is None
    assert await store.seen_status(digest) is None
    assert not await store.is_created("issue", "10001")
    assert await store.get_draft("d-old") is None
    assert await store.draft_by_message("m-old") is None
    # The thread map and the queued job survive pruning.
    assert await store.thread_of_issue("10001") == ThreadRef("t-1", "forum-1")
    assert await store.issue_of_thread("t-1") == MappedIssue("10001", "SUP-12")
    job = await store.next_job()
    assert job is not None
    assert job.data == {"draft": "d-old"}

    # A pruned identifier is new again.
    clock.t = START + WINDOW + 1
    assert (await store.admit(key, 202, None, MANY)).kind == "accepted"


async def test_reads_hide_rows_past_the_window_before_pruning(
    store: SqlStore, clock: FakeClock
) -> None:
    key = SeenKey("body_digest", "b-1")
    await store.admit(key, 202, None, MANY)
    await store.record_created("issue", "10001")
    await store.put_draft(_draft("d-1", "m-1", clock()))
    clock.t = START + WINDOW + 1
    assert await store.seen_status(key) is None
    assert not await store.is_created("issue", "10001")
    assert await store.get_draft("d-1") is None
    assert await store.draft_by_message("m-1") is None


async def test_closed_store_refuses_use(tmp_path: Path, clock: FakeClock) -> None:
    for factory in (_sqlite, _memory):
        s = factory(tmp_path, clock)
        await s.open()
        await s.close()
        with pytest.raises(StoreError):
            await s.next_job()


# SqliteStore only.


async def test_commits_survive_a_reopen(tmp_path: Path, clock: FakeClock) -> None:
    first = _sqlite(tmp_path, clock)
    await first.open()
    try:
        await first.map_thread("10001", "SUP-12", ThreadRef("t-1", "forum-1"))
        await first.record_created("issue", "10001")
        await first.put_draft(_draft("d-1", "m-1", clock()))
        a = await first.admit(SeenKey("webhook_id", "w-1"), 202, NewJob("mirror", {"n": 1}), MANY)
        b = await first.admit(SeenKey("body_digest", "b-1"), 202, NewJob("mirror", {"n": 2}), MANY)
        job = await first.next_job()
        assert job is not None
        job.progress = {"step": "posted"}
        await first.save_progress(job)
    finally:
        await first.close()

    second = _sqlite(tmp_path, clock)
    await second.open()
    try:
        assert await second.thread_of_issue("10001") == ThreadRef("t-1", "forum-1")
        assert await second.issue_of_thread("t-1") == MappedIssue("10001", "SUP-12")
        assert await second.is_created("issue", "10001")
        assert await second.get_draft("d-1") == _draft("d-1", "m-1", clock())
        assert await second.seen_status(SeenKey("webhook_id", "w-1")) == 202
        assert await second.seen_status(SeenKey("body_digest", "b-1")) == 202
        order = []
        while (current := await second.next_job()) is not None:
            order.append((current.id, current.data, current.progress))
            await second.remove_job(current.id)
        assert order == [(a.job_id, {"n": 1}, {"step": "posted"}), (b.job_id, {"n": 2}, {})]
        c = await second.admit(SeenKey("webhook_id", "w-3"), 202, NewJob("mirror", {}), MANY)
        assert c.job_id > b.job_id
    finally:
        await second.close()


def _mode(path: Path) -> int:
    return stat.S_IMODE(path.stat().st_mode)


async def test_modes_0700_and_0600(tmp_path: Path, clock: FakeClock) -> None:
    # A permissive umask: the store sets the modes of its files on its own.
    old = os.umask(0o022)
    try:
        s = _sqlite(tmp_path, clock)
        await s.open()
        try:
            await s.admit(SeenKey("webhook_id", "w-1"), 202, NewJob("mirror", {}), MANY)
            directory = tmp_path / "state"
            assert _mode(directory) == 0o700
            assert _mode(directory / DATABASE_FILE) == 0o600
            assert _mode(directory / LOCK_FILE) == 0o600
            names = {p.name for p in directory.iterdir()}
            assert {DATABASE_FILE, LOCK_FILE} <= names
            for name in (DATABASE_FILE + "-wal", DATABASE_FILE + "-shm"):
                if name in names:
                    assert _mode(directory / name) == 0o600, name
            for path in directory.iterdir():
                assert _mode(path) & (stat.S_IRWXG | stat.S_IRWXO) == 0, path.name
        finally:
            await s.close()
    finally:
        os.umask(old)


_HOLDER = (
    "import fcntl, os, sys\n"
    "fd = os.open(sys.argv[1], os.O_RDWR | os.O_CREAT, 0o600)\n"
    "fcntl.flock(fd, fcntl.LOCK_EX)\n"
    "print('ready', flush=True)\n"
    "sys.stdin.read()\n"
)


async def test_second_process_holding_the_lock_fails_the_start(
    tmp_path: Path, clock: FakeClock
) -> None:
    directory = check_state_dir(str(tmp_path / "state"))
    child = subprocess.Popen(  # noqa: S603 - fixed script, the test's own interpreter
        [sys.executable, "-c", _HOLDER, str(directory / LOCK_FILE)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        text=True,
    )
    try:
        assert child.stdout is not None
        assert child.stdout.readline().strip() == "ready"
        s = SqliteStore(directory, now=clock, window=WINDOW)
        with pytest.raises(StoreLockedError) as info:
            await s.open()
        assert str(directory) in str(info.value)
        assert isinstance(info.value, StoreError)
        await s.close()
    finally:
        os.kill(child.pid, signal.SIGKILL)
        child.wait()
        if child.stdin is not None:
            child.stdin.close()
        if child.stdout is not None:
            child.stdout.close()

    # The kernel released the lock with the process: the start works now.
    s = SqliteStore(directory, now=clock, window=WINDOW)
    await s.open()
    try:
        assert await s.next_job() is None
    finally:
        await s.close()
