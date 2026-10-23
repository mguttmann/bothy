# SPDX-License-Identifier: Apache-2.0
"""Time source of the hand layer.

Every time decision of the SDK (token expiry, throttle, backoff, autocomplete guard, JWKS age)
goes through a `Clock`, so tests drive time with ``bothy_sdk.testing.FakeClock`` instead of
sleeping (docs/DECISIONS.md D-M2-29-2).

Examples:
    >>> clock = SystemClock()
    >>> clock.now().tzinfo is not None
    True
"""

from __future__ import annotations

import asyncio
import time
from datetime import UTC, datetime
from typing import Protocol, runtime_checkable

__all__ = ["Clock", "SystemClock"]


@runtime_checkable
class Clock(Protocol):
    """Wall time for token checks, a monotonic time for waits, and an awaitable sleep."""

    def now(self) -> datetime:
        """Return the current UTC wall time (timezone aware)."""
        ...

    def monotonic(self) -> float:
        """Return a monotonic time in seconds for intervals."""
        ...

    async def sleep(self, seconds: float) -> None:
        """Wait for ``seconds`` (zero or negative returns at the next loop turn).

        Args:
            seconds: The wait in seconds.
        """
        ...


class SystemClock:
    """The real clock: `datetime.now` in UTC, `time.monotonic` and `asyncio.sleep`."""

    def now(self) -> datetime:
        """Return the current UTC wall time."""
        return datetime.now(UTC)

    def monotonic(self) -> float:
        """Return `time.monotonic`."""
        return time.monotonic()

    async def sleep(self, seconds: float) -> None:
        """Sleep with `asyncio.sleep`.

        Args:
            seconds: The wait in seconds; negative counts as zero.
        """
        await asyncio.sleep(max(0.0, seconds))
