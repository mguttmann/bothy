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

from claude_code_runner.locks import LockTimeout, acquire, release


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
