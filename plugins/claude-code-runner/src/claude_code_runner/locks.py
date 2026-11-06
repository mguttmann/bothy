# SPDX-License-Identifier: Apache-2.0
"""File locks per thread and per session, and the shared turn slots (09 18.2 "Locks", note 11).

A lock is ``fcntl.flock`` on ``<runner.state_dir>/locks/<name>.lock``. The descriptor is
inherited by the turn (``pass_fds``), so the lock lives as long as the turn, also when the
runner dies: no second turn of the same session can start. A hosted agent's locks live under its
own state folder (docs/spec/26-builtin-agents.md 3.2 item 2), so two agents never share a lock.

`SlotPool` holds the slots of ``runner.max_parallel_runs``; one pool serves every agent of the
process, the runner's own agent and each hosted agent, because the slots protect one host
(26 3.2 item 3).

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
from collections.abc import Callable
from pathlib import Path

from claude_code_runner.statedir import ensure_private_dir

__all__ = ["LockTimeout", "SlotPool", "acquire", "release"]


class LockTimeout(Exception):  # noqa: N818 - the name says what happened
    """The lock stayed held for ``runner.session_lock_wait``."""


async def acquire(
    path: Path, wait: float, poll: float, on_busy: Callable[[], None] | None = None
) -> int:
    """Take an exclusive lock, trying again every ``poll`` until ``wait`` has passed.

    Args:
        path: The lock file (created with mode ``0600``).
        wait: ``runner.session_lock_wait``.
        poll: ``runner.poll_interval``.
        on_busy: Called once when the first attempt finds the lock held, before the wait
            (the wait reason ``thread_busy``, 09 5.2.1).

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
                if on_busy is not None:
                    on_busy()
                    on_busy = None
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


class SlotPool:
    """The turn slots of ``runner.max_parallel_runs``, shared by every agent of the process.

    Every turn holds one slot from before ``startRun`` until it ends (09 18.2). The pool counts
    the slots in use, so each agent's heartbeat reports the shared totals in ``run_slots``
    (09 5.2.1, 26 3.2 item 3).

    Args:
        total: ``runner.max_parallel_runs``.

    Raises:
        ValueError: ``total`` is below one.

    Examples:
        >>> pool = SlotPool(2)
        >>> asyncio.run(pool.acquire())
        >>> pool.used, pool.total, pool.full
        (1, 2, False)
    """

    def __init__(self, total: int) -> None:
        """Create the pool with ``total`` free slots."""
        if total < 1:
            raise ValueError("a slot pool needs at least one slot")
        self.total = total
        self._free = asyncio.Semaphore(total)
        self._used = 0

    @property
    def used(self) -> int:
        """The slots that turns of any agent hold now."""
        return self._used

    @property
    def full(self) -> bool:
        """True when every slot is in use, so the next turn waits (``slots_full``)."""
        return self._free.locked()

    async def acquire(self) -> None:
        """Wait for a free slot and take it; a cancelled wait takes nothing."""
        await self._free.acquire()
        self._used += 1

    def release(self) -> None:
        """Give one slot back (once per `acquire`)."""
        self._used -= 1
        self._free.release()
