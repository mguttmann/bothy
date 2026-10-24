# SPDX-License-Identifier: Apache-2.0
"""File locks per thread and per session (docs/spec/09-agents.md 18.2 "Locks", note 11).

A lock is ``fcntl.flock`` on ``<runner.state_dir>/locks/<name>.lock``. The descriptor is
inherited by the turn (``pass_fds``), so the lock lives as long as the turn, also when the
runner dies: no second turn of the same session can start.

Examples:
    >>> import asyncio, tempfile, pathlib
    >>> path = pathlib.Path(tempfile.mkdtemp()) / "locks" / "session-a.lock"
    >>> fd = asyncio.run(acquire(path, wait=1.0, poll=0.01))
    >>> release(fd)
"""

from __future__ import annotations

import asyncio
import contextlib
import fcntl
import os
import time
from pathlib import Path

from claude_code_runner.statedir import ensure_private_dir

__all__ = ["LockTimeout", "acquire", "release"]


class LockTimeout(Exception):  # noqa: N818 - the name says what happened
    """The lock stayed held for ``runner.session_lock_wait``."""


async def acquire(path: Path, wait: float, poll: float) -> int:
    """Take an exclusive lock, trying again every ``poll`` until ``wait`` has passed.

    Args:
        path: The lock file (created with mode ``0600``).
        wait: ``runner.session_lock_wait``.
        poll: ``runner.poll_interval``.

    Returns:
        The descriptor that holds the lock.

    Raises:
        LockTimeout: The lock was still held after ``wait``.
    """
    ensure_private_dir(path.parent)
    # A local lock file opens at once; an executor would leak the descriptor on cancel (D-M2-34-14).
    fd = os.open(path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)  # NOSONAR
    deadline = time.monotonic() + wait
    try:
        while True:
            try:
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
                return fd
            except BlockingIOError:
                if time.monotonic() >= deadline:
                    raise LockTimeout from None
            # Outside the handler: a cancellation during the wait reaches the close below.
            await asyncio.sleep(poll)
    except BaseException:
        os.close(fd)
        raise


def release(fd: int) -> None:
    """Close the runner's descriptor; a turn that inherited it keeps the lock until it ends.

    No explicit unlock: ``flock`` locks belong to the open file description, and an unlock here
    would also free the turn's inherited copy.

    Args:
        fd: The descriptor of `acquire`.
    """
    with contextlib.suppress(OSError):
        os.close(fd)
