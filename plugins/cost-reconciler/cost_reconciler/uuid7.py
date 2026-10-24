# SPDX-License-Identifier: Apache-2.0
"""UUIDv7 values (RFC 9562 section 5.7) for the header `Idempotency-Key`.

The standard library of the supported Python versions may lack `uuid.uuid7`, so the app builds
them itself: 48 bits of Unix time in milliseconds, the version `7`, 12 bits `rand_a` used as a
counter within one millisecond (RFC 9562 section 6.2, method 1), the variant `10` and 62 random
bits. Values of one generator rise strictly, also when the clock stands still or goes back.

Example:
    >>> new_uuid7().version
    7
"""

from __future__ import annotations

import os
import threading
import time
import uuid
from collections.abc import Callable

_COUNTER_MAX = 0xFFF
# A fresh millisecond starts the counter in the lower half, so many values fit before it runs
# over into the next millisecond.
_COUNTER_SEED_MASK = 0x7FF
_RAND_B_BITS = 62


def _now_ms() -> int:
    return time.time_ns() // 1_000_000


class Uuid7Generator:
    """Creates strictly rising UUIDv7 values; safe to share between threads."""

    def __init__(
        self,
        now_ms: Callable[[], int] = _now_ms,
        random_bytes: Callable[[int], bytes] = os.urandom,
    ) -> None:
        """Create the generator.

        Args:
            now_ms: Unix time in milliseconds.
            random_bytes: Source of random bytes.
        """
        self._now_ms = now_ms
        self._random = random_bytes
        self._lock = threading.Lock()
        self._last_ms = -1
        self._counter = 0

    def __call__(self) -> uuid.UUID:
        """Return the next value."""
        with self._lock:
            now = self._now_ms()
            if now > self._last_ms:
                self._last_ms = now
                self._counter = int.from_bytes(self._random(2), "big") & _COUNTER_SEED_MASK
            else:
                self._counter += 1
                if self._counter > _COUNTER_MAX:
                    self._last_ms += 1
                    self._counter = 0
            millis, counter = self._last_ms, self._counter
        rand_b = int.from_bytes(self._random(8), "big") & ((1 << _RAND_B_BITS) - 1)
        value = (millis & ((1 << 48) - 1)) << 80
        value |= 0x7 << 76
        value |= counter << 64
        value |= 0b10 << 62
        value |= rand_b
        return uuid.UUID(int=value)


_DEFAULT = Uuid7Generator()


def new_uuid7() -> uuid.UUID:
    """Return a new UUIDv7 from the process-wide generator."""
    return _DEFAULT()
