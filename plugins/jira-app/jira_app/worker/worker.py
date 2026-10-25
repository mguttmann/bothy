# SPDX-License-Identifier: Apache-2.0
"""The worker of the queue (brief M2-50 Design note 9).

One job at a time, strictly in creation order, so events of one issue keep their order and a
``/issue`` job records its issue before Jira's ``jira:issue_created`` event of that issue is
handled. A finished job is deleted; a job that met a transient failure stays queued and the
worker pauses ``sdk.retry_backoff_cap`` before it tries again. `Worker.stop` lets the running
step finish.

Examples:
    >>> Worker.__doc__.split(".")[0]
    'Works the queue of a store'
"""

from __future__ import annotations

import asyncio
import contextlib
from collections.abc import Awaitable, Callable
from typing import Literal

from bothy_sdk.clock import Clock

from jira_app.log import AppLog
from jira_app.store.base import JobRecord, Store, StoreError

__all__ = ["JobOutcome", "Worker"]

JobOutcome = Literal["done", "retry"]
"""``done``: the job ends and is deleted; ``retry``: it stays queued and the worker pauses."""


class Worker:
    """Works the queue of a store.

    Args:
        store: State and queue.
        run: Runs one job and decides its outcome.
        clock: Source of the pause after a transient failure.
        pause: ``sdk.retry_backoff_cap`` in seconds.
        log: The app's logger.
    """

    def __init__(
        self,
        store: Store,
        run: Callable[[JobRecord], Awaitable[JobOutcome]],
        clock: Clock,
        pause: float,
        log: AppLog,
    ) -> None:
        """Create the worker; nothing runs before `start`."""
        self._store = store
        self._run = run
        self._clock = clock
        self._pause = pause
        self._log = log
        self._wake = asyncio.Event()
        self._stopping = asyncio.Event()
        self._idle = asyncio.Event()
        self._task: asyncio.Task[None] | None = None

    def start(self) -> None:
        """Start the loop."""
        if self._task is None:
            self._task = asyncio.create_task(self._work())

    def wake(self) -> None:
        """Tell the worker that a job was queued."""
        self._idle.clear()
        self._wake.set()

    async def idle(self) -> None:
        """Return once the queue is empty and the worker waits for work (tests)."""
        if self._task is None:
            return
        await self._idle.wait()

    async def stop(self) -> None:
        """Stop the loop after the running step."""
        self._stopping.set()
        self._wake.set()
        if self._task is not None:
            await self._task
        self._idle.set()

    async def _wait(self, event: asyncio.Event, sleep: Awaitable[None] | None = None) -> None:
        waiters: list[asyncio.Task[object]] = [
            asyncio.create_task(event.wait()),
            asyncio.create_task(self._stopping.wait()),
        ]
        if sleep is not None:
            waiters.append(asyncio.ensure_future(sleep))
        try:
            await asyncio.wait(waiters, return_when=asyncio.FIRST_COMPLETED)
        finally:
            for waiter in waiters:
                waiter.cancel()
            for waiter in waiters:
                with contextlib.suppress(asyncio.CancelledError):
                    await waiter

    async def _work(self) -> None:
        while not self._stopping.is_set():
            self._wake.clear()
            try:
                job = await self._store.next_job()
            except StoreError:
                self._log.error("worker.failed", reason="store")
                await self._wait(self._stopping, self._clock.sleep(self._pause))
                continue
            if job is None:
                self._idle.set()
                await self._wait(self._wake)
                continue
            outcome = await self._outcome(job)
            if outcome == "retry" and not self._stopping.is_set():
                self._log.info(
                    "worker.paused", job_id=job.id, job_kind=job.kind, reason="transient"
                )
                await self._wait(self._stopping, self._clock.sleep(self._pause))

    async def _outcome(self, job: JobRecord) -> JobOutcome:
        try:
            outcome = await self._run(job)
        except Exception:
            self._log.error("worker.failed", job_id=job.id, job_kind=job.kind, reason="store")
            return "retry"
        if outcome == "done":
            try:
                await self._store.remove_job(job.id)
            except StoreError:
                self._log.error("worker.failed", job_id=job.id, job_kind=job.kind, reason="store")
                return "retry"
        return outcome
