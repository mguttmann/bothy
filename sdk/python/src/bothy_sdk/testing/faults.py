# SPDX-License-Identifier: Apache-2.0
"""Faults the FakeServer injects into one operation (docs/spec/09-agents.md 16.1).

A `Fault` describes what goes wrong; ``FakeServer.inject(target, fault, times)`` applies it to the
next ``times`` calls of an ``operationId`` or a gateway op:

* `Fault.stored_then_502`: the operation runs and stores its effect, then the answer is ``502``
  with a problem body, so a client that retries must not create a duplicate.
* `Fault.stored_late`: the answer comes after ``delay`` seconds of the FakeServer's clock and the
  effect is stored at the end of that wait.
* `Fault.rate_limited`: ``429 rate_limited`` with ``Retry-After``; nothing is stored.
* `Fault.disconnect`: the connection is dropped without an answer; nothing is stored. On a
  gateway op the WebSocket connection is dropped.
* `Fault.problem`: the answer is the problem ``code`` (with ``reason``) at its status; nothing is
  stored (M2-34: a refused renewal, a ``startRun`` that says ``expired``).

Examples:
    >>> fault = Fault.rate_limited(2)
    >>> (fault.kind, fault.retry_after)
    (<FaultKind.RATE_LIMITED: 'rate_limited'>, 2)
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum

__all__ = ["Fault", "FaultKind"]


class FaultKind(StrEnum):
    """The faults of the FakeServer."""

    STORED_THEN_502 = "stored_then_502"
    STORED_LATE = "stored_late"
    RATE_LIMITED = "rate_limited"
    DISCONNECT = "disconnect"
    PROBLEM = "problem"


@dataclass(frozen=True)
class Fault:
    """One fault; build it with the class methods.

    Attributes:
        kind: What goes wrong.
        delay: Seconds of the FakeServer's clock before a `FaultKind.STORED_LATE` answer.
        retry_after: Whole seconds of the ``Retry-After`` header of `FaultKind.RATE_LIMITED`.
        code: The problem code of `FaultKind.PROBLEM`.
        reason: The problem ``reason`` of `FaultKind.PROBLEM`, if any.
    """

    kind: FaultKind
    delay: float = 0.0
    retry_after: int = 0
    code: str = ""
    reason: str | None = None

    @classmethod
    def stored_then_502(cls) -> Fault:
        """Run and store the operation, then answer ``502`` with a problem body.

        Returns:
            The fault.
        """
        return cls(FaultKind.STORED_THEN_502)

    @classmethod
    def stored_late(cls, delay: float) -> Fault:
        """Answer after ``delay`` seconds of the clock and store at the end of the wait.

        Args:
            delay: The wait in seconds; must not be negative.

        Returns:
            The fault.

        Raises:
            ValueError: ``delay`` is negative.
        """
        if delay < 0:
            raise ValueError("delay must not be negative")
        return cls(FaultKind.STORED_LATE, delay=delay)

    @classmethod
    def rate_limited(cls, retry_after: int) -> Fault:
        """Answer ``429 rate_limited`` with ``Retry-After`` and store nothing.

        Args:
            retry_after: Whole seconds for the ``Retry-After`` header; must not be negative.

        Returns:
            The fault.

        Raises:
            ValueError: ``retry_after`` is negative.
        """
        if retry_after < 0:
            raise ValueError("retry_after must not be negative")
        return cls(FaultKind.RATE_LIMITED, retry_after=retry_after)

    @classmethod
    def disconnect(cls) -> Fault:
        """Drop the connection without an answer and store nothing.

        Returns:
            The fault.
        """
        return cls(FaultKind.DISCONNECT)

    @classmethod
    def problem(cls, code: str, reason: str | None = None) -> Fault:
        """Answer the problem ``code`` (its status follows the code) and store nothing.

        Args:
            code: A problem code of ``x-problem-codes``, for example ``agent_invocation_denied``.
            reason: The ``reason`` member, if the code has one.

        Returns:
            The fault.

        Raises:
            ValueError: ``code`` is empty.
        """
        if not code:
            raise ValueError("code must not be empty")
        return cls(FaultKind.PROBLEM, code=code, reason=reason)
