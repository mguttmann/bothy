# SPDX-License-Identifier: Apache-2.0
"""Waiting for a condition in tests without a wall-clock assumption (private).

A test waits for a state, never a fixed time or a fixed number of loop turns: on a loaded machine
or a slow CI runner a turn takes longer, so a bound by turns or by a few seconds fails a correct
program. The only upper bound is `PATIENCE`, the value of `quality.test.patience` (the Go tests'
`testkit.Patience`), which only ends a test that would otherwise hang.
"""

from __future__ import annotations

import asyncio
import inspect
import time
from collections.abc import Awaitable, Callable
from typing import Any, TypeVar

# quality.test.patience: the same 3m as services/server/internal/platform/testkit.Patience.
PATIENCE = 180.0

_T = TypeVar("_T")


class PatienceExceededError(AssertionError):
    """The condition did not hold within `PATIENCE`; the test hangs on a real defect."""


async def eventually(
    predicate: Callable[[], Any],
    what: str,
    *,
    step: Callable[[], Awaitable[None]] | None = None,
) -> None:
    """Wait until ``predicate`` holds; run ``step`` (for example a clock advance) between checks.

    Args:
        predicate: The condition; it may return an awaitable.
        what: Names the condition in the failure.
        step: Called between two checks; without it the loop only yields to the event loop.

    Raises:
        PatienceExceededError: The condition did not hold within `PATIENCE`.
    """
    deadline = time.monotonic() + PATIENCE
    while True:
        result = predicate()
        if inspect.isawaitable(result):
            result = await result
        if result:
            return
        if time.monotonic() > deadline:
            raise PatienceExceededError(f"waited {PATIENCE:.0f}s for {what}")
        if step is not None:
            await step()
        await asyncio.sleep(0.005)


async def within_patience(awaitable: Awaitable[_T]) -> _T:
    """Await ``awaitable`` with `PATIENCE` as the only bound.

    Args:
        awaitable: What to wait for.

    Returns:
        Its result.
    """
    return await asyncio.wait_for(awaitable, PATIENCE)
