# SPDX-License-Identifier: Apache-2.0
"""uuid7: version 7, RFC 9562 variant, strictly increasing within one process."""

from __future__ import annotations

import time

from bothy_sdk.ids import uuid7


def test_uuid7_version_variant_and_time() -> None:
    before = time.time_ns() // 1_000_000
    u = uuid7()
    after = time.time_ns() // 1_000_000
    assert u.version == 7
    assert (u.int >> 62) & 0b11 == 0b10
    assert before <= u.int >> 80 <= after + 1


def test_uuid7_monotonic_within_a_millisecond() -> None:
    ids = [uuid7() for _ in range(5000)]
    assert ids == sorted(ids)
    assert len(set(ids)) == len(ids)
