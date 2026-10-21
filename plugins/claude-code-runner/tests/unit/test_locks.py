# SPDX-License-Identifier: Apache-2.0
"""File locks per thread and per session (09 18.2 "Locks", design note 11)."""

from __future__ import annotations

import subprocess
import sys
import time
from pathlib import Path

import pytest

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
