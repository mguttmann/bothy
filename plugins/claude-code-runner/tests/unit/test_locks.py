# SPDX-License-Identifier: Apache-2.0
"""File locks per thread and per session (09 18.2 "Locks", design note 11)."""

from __future__ import annotations

import asyncio
import subprocess
import sys
import time
from pathlib import Path

import pytest
from bothy_sdk.testing._patience import PATIENCE, eventually

from claude_code_runner.locks import LockTimeout, SlotPool, acquire, release


@pytest.mark.anyio
async def test_exclusive_and_timeout(tmp_path: Path) -> None:
    path = tmp_path / "locks" / "session-a.lock"
    fd = await acquire(path, wait=1.0, poll=0.02)
    assert path.stat().st_mode & 0o777 == 0o600
    start = time.monotonic()
    with pytest.raises(LockTimeout):
        await acquire(path, wait=0.2, poll=0.02)
    assert time.monotonic() - start >= 0.2
    release(fd)
    again = await acquire(path, wait=0.2, poll=0.02)
    release(again)


@pytest.mark.anyio
async def test_child_keeps_inherited_lock(tmp_path: Path) -> None:
    path = tmp_path / "locks" / "thread-t.lock"
    fd = await acquire(path, wait=1.0, poll=0.02)
    child = subprocess.Popen(
        [sys.executable, "-c", "import time; time.sleep(30)"], pass_fds=(fd,), close_fds=True
    )
    try:
        release(fd)  # the parent closes its copy; the child's copy keeps the lock
        with pytest.raises(LockTimeout):
            await acquire(path, wait=0.2, poll=0.02)
    finally:
        child.kill()
        child.wait()
    fd2 = await acquire(path, wait=1.0, poll=0.02)
    release(fd2)


def _open_fds() -> int:
    return sum(1 for _ in Path("/dev/fd").iterdir())


@pytest.mark.anyio
async def test_cancelled_wait_closes_the_descriptor(tmp_path: Path) -> None:
    path = tmp_path / "locks" / "session-c.lock"
    held = await acquire(path, wait=PATIENCE, poll=0.01)
    before = _open_fds()
    waiter = asyncio.ensure_future(acquire(path, wait=PATIENCE, poll=0.01))
    await eventually(lambda: _open_fds() == before + 1, "the waiter's open descriptor")
    waiter.cancel()
    with pytest.raises(asyncio.CancelledError):
        await waiter
    assert _open_fds() == before
    release(held)


@pytest.mark.anyio
async def test_on_busy_once_only_when_held(tmp_path: Path) -> None:
    """``on_busy`` reports the wait reason ``thread_busy`` once, and never for a free lock."""
    path = tmp_path / "locks" / "thread-a.lock"
    calls: list[str] = []
    fd = await acquire(path, wait=1.0, poll=0.02, on_busy=lambda: calls.append("busy"))
    assert calls == []
    with pytest.raises(LockTimeout):
        await acquire(path, wait=0.2, poll=0.02, on_busy=lambda: calls.append("busy"))
    assert calls == ["busy"]
    release(fd)


@pytest.mark.anyio
async def test_slot_pool_is_shared_by_every_holder() -> None:
    # 26 3.2 item 3: one pool of runner.max_parallel_runs for the runner's own agent and every
    # hosted agent; a third turn waits while two other agents hold both slots.
    pool = SlotPool(2)
    assert (pool.used, pool.total, pool.full) == (0, 2, False)
    await pool.acquire()  # the runner's own agent
    await pool.acquire()  # a hosted agent
    assert (pool.used, pool.full) == (2, True)
    third = asyncio.ensure_future(pool.acquire())  # another hosted agent
    await asyncio.sleep(0.05)
    assert not third.done()
    pool.release()
    await asyncio.wait_for(third, PATIENCE)
    assert pool.used == 2
    pool.release()
    pool.release()
    assert (pool.used, pool.full) == (0, False)


@pytest.mark.anyio
async def test_slot_pool_cancelled_wait_takes_nothing() -> None:
    pool = SlotPool(1)
    await pool.acquire()
    waiter = asyncio.ensure_future(pool.acquire())
    await asyncio.sleep(0.02)
    waiter.cancel()
    with pytest.raises(asyncio.CancelledError):
        await waiter
    assert pool.used == 1
    pool.release()
    assert pool.used == 0


def test_slot_pool_needs_one_slot() -> None:
    with pytest.raises(ValueError, match="at least one"):
        SlotPool(0)
