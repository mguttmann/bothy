# SPDX-License-Identifier: Apache-2.0
"""Event log, consumer cursors and delivery windows of the FakeServer (private).

The rules are those of docs/spec/04-sync-protocol.md 15.1 and 15.2 and docs/spec/09-agents.md
4.3 as the server's consumer engine builds them: a cursor per consumer name created at the head
by the first takeover, delivery from ``acked_seq + 1`` in ``seq`` order, at most
``agent.max_inflight_events`` unacknowledged, and after ``agent.ack_timeout`` (counted from the
oldest unacknowledged delivery) a new delivery from ``acked_seq + 1`` with ``redelivered``.
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any


@dataclass
class LogEntry:
    """One durable event: its envelope without delivery fields and the run it concerns."""

    seq: int
    type: str
    envelope: dict[str, Any]
    run_id: str | None = None


@dataclass
class ConsumerCursor:
    """The stored acknowledgement of one consumer."""

    acked: int


@dataclass
class Window:
    """What one holder (a gateway connection or the REST window) has on the way.

    Attributes:
        acked: The acknowledged ``seq`` known to the window.
        read: The highest ``seq`` the window checked.
        inflight: Unacknowledged ``seq`` values with their delivery time (clock monotonic).
        redeliver_up_to: Deliveries up to this ``seq`` carry ``redelivered``.
        highest_sent: The highest ``seq`` ever delivered by this window.
    """

    acked: int
    read: int = 0
    inflight: dict[int, float] = field(default_factory=dict)
    redeliver_up_to: int = 0
    highest_sent: int = 0

    def __post_init__(self) -> None:
        """Start reading after the acknowledgement."""
        self.read = max(self.read, self.acked)

    def ack(self, seq: int) -> None:
        """Apply a cumulative, monotone acknowledgement."""
        if seq <= self.acked:
            return
        self.acked = seq
        self.inflight = {s: t for s, t in self.inflight.items() if s > seq}
        self.read = max(self.read, seq)

    def deadline(self, ack_timeout: float) -> float | None:
        """The monotonic time at which the oldest delivery times out, if any."""
        return min(self.inflight.values()) + ack_timeout if self.inflight else None

    def expire(self, now: float, ack_timeout: float) -> bool:
        """Start a new delivery from ``acked + 1`` when the oldest delivery timed out.

        Returns:
            Whether the window expired.
        """
        due = self.deadline(ack_timeout)
        if due is None or now < due:
            return False
        self.redeliver_up_to = max(self.redeliver_up_to, self.highest_sent)
        self.inflight.clear()
        self.read = self.acked
        return True

    def budget(self, max_inflight: int) -> int:
        """How many more deliveries may be on the way."""
        return max(0, max_inflight - len(self.inflight))

    def sent(self, seq: int, now: float) -> bool:
        """Record a delivery of ``seq``.

        Returns:
            The delivery field ``redelivered``.
        """
        self.inflight[seq] = now
        self.read = seq
        self.highest_sent = max(self.highest_sent, seq)
        return seq <= self.redeliver_up_to


# A delivery hook decides per event: None skips it (it counts as checked), else the envelope
# with its delivery fields.
Hook = Callable[[LogEntry], dict[str, Any] | None]


def fill(  # noqa: PLR0913 - the inputs of one fill pass
    window: Window,
    log: list[LogEntry],
    hook: Hook,
    *,
    now: float,
    max_inflight: int,
    limit: int | None = None,
) -> list[dict[str, Any]]:
    """Deliver from ``window.read + 1`` while the budget lasts.

    Returns:
        The envelopes to send, in ``seq`` order.
    """
    out: list[dict[str, Any]] = []
    for entry in log:
        if entry.seq <= window.read:
            continue
        if entry.seq <= window.acked:
            window.read = entry.seq
            continue
        if window.budget(max_inflight) == 0 or (limit is not None and len(out) >= limit):
            break
        envelope = hook(entry)
        if envelope is None:
            window.read = entry.seq
            continue
        if window.sent(entry.seq, now):
            envelope["redelivered"] = True
        out.append(envelope)
    return out
