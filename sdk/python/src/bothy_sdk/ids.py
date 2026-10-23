# SPDX-License-Identifier: Apache-2.0
"""UUIDv7 for the identifiers the adapter chooses (RFC 9562 section 5.7).

`client_msg_id`, `Idempotency-Key`, DPoP `jti` and run step IDs are UUIDv7: time ordered, so a
server index stays compact, and monotonic within one millisecond of one process (RFC 9562 6.2,
method 1: a 12-bit counter in ``rand_a`` seeded randomly each millisecond).

Examples:
    >>> a, b = uuid7(), uuid7()
    >>> (a.version, a < b)
    (7, True)
"""

from __future__ import annotations

import secrets
import threading
import time
from uuid import UUID

__all__ = ["uuid7"]

_lock = threading.Lock()
_last_ms = -1
_counter = 0
_COUNTER_MAX = 0xFFF


def uuid7() -> UUID:
    """Return a new UUIDv7, strictly greater than every earlier one of this process.

    Returns:
        The UUID (version 7, variant RFC 9562).
    """
    global _last_ms, _counter  # noqa: PLW0603 - one process-wide sequence by design
    with _lock:
        ms = time.time_ns() // 1_000_000
        if ms <= _last_ms:
            # Same (or an earlier, after a clock step back) millisecond: count on from the last
            # value so the order holds; an exhausted counter borrows the next millisecond.
            ms = _last_ms
            _counter += 1
            if _counter > _COUNTER_MAX:
                ms += 1
                _counter = secrets.randbits(11)
        else:
            # A random start in the lower half leaves room to count within the millisecond.
            _counter = secrets.randbits(11)
        _last_ms = ms
        rand_b = secrets.randbits(62)
        value = (ms & 0xFFFFFFFFFFFF) << 80
        value |= 0x7 << 76
        value |= _counter << 64
        value |= 0b10 << 62
        value |= rand_b
        return UUID(int=value)
