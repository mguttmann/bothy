# SPDX-License-Identifier: Apache-2.0
"""The control protocol of the Fake-Agent: JSON lines between the conductor and the process.

The conductor of the agent conformance suite (``services/server/test/agents``) writes one
`Request` per line to the stdin of the Fake-Agent and reads one `Reply` per line from its stdout,
in request order (docs/spec/09-agents.md 16.1 row "Fake agent"). This module holds the typed
forms and their line codec; it never sends anything itself.

Examples:
    >>> line = '{"id": 1, "op": "connect", "args": {}, "now": "2030-01-01T00:00:00.000Z"}'
    >>> req = parse_request(line)
    >>> (req.id, req.op, req.now.year)
    (1, 'connect', 2030)
    >>> encode_reply(Reply.success(1, {"head_seq": 7}))
    '{"id": 1, "ok": true, "result": {"head_seq": 7}}'
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any

__all__ = [
    "ControlError",
    "ErrorKind",
    "Reply",
    "Request",
    "encode_reply",
    "format_now",
    "parse_request",
]


class ErrorKind(StrEnum):
    """The kinds of an error reply."""

    USAGE = "usage"
    """Unknown op, malformed line, missing ``hello`` or a bad argument."""
    TRANSPORT = "transport"
    """No HTTP answer: the connection was closed or reset."""
    INTERNAL = "internal"
    """Anything else."""


class ControlError(Exception):
    """A request that cannot be served; the reply carries `kind` and `detail`.

    Args:
        kind: The error kind.
        detail: A short text without any token or credential.
    """

    def __init__(self, kind: ErrorKind, detail: str) -> None:
        """Create the error; see the class docstring."""
        super().__init__(detail)
        self.kind = kind
        self.detail = detail


@dataclass(frozen=True)
class Request:
    """One request of the conductor.

    Attributes:
        id: The request ID the reply repeats.
        op: The op, for example ``receive``.
        args: The arguments of the op.
        now: The harness clock at the moment of the request (UTC).
    """

    id: int
    op: str
    args: dict[str, Any] = field(default_factory=dict)
    now: datetime | None = None


@dataclass(frozen=True)
class Reply:
    """One reply of the Fake-Agent.

    Attributes:
        id: The ID of the request (``0`` for a line that did not parse).
        ok: True for a result, False for an error.
        result: The result members of the op (``ok`` only).
        error_kind: The kind of the error (error only).
        error_detail: The detail of the error (error only).
    """

    id: int
    ok: bool
    result: dict[str, Any] = field(default_factory=dict)
    error_kind: ErrorKind | None = None
    error_detail: str = ""

    @classmethod
    def success(cls, request_id: int, result: dict[str, Any]) -> Reply:
        """Return a result reply.

        Args:
            request_id: The request ID.
            result: The result members.

        Returns:
            The reply.
        """
        return cls(id=request_id, ok=True, result=result)

    @classmethod
    def failure(cls, request_id: int, kind: ErrorKind, detail: str) -> Reply:
        """Return an error reply.

        Args:
            request_id: The request ID.
            kind: The error kind.
            detail: A short text without secrets.

        Returns:
            The reply.
        """
        return cls(id=request_id, ok=False, error_kind=kind, error_detail=detail)


def parse_request(line: str) -> Request:
    """Decode one request line.

    Args:
        line: A JSON object with ``id`` (integer), ``op`` (text), optional ``args`` (object)
            and optional ``now`` (RFC 3339 UTC).

    Returns:
        The request.

    Raises:
        ControlError: The line is not such an object (kind ``usage``).
    """
    try:
        data = json.loads(line)
    except ValueError as exc:
        raise ControlError(ErrorKind.USAGE, "malformed line") from exc
    if not isinstance(data, dict):
        raise ControlError(ErrorKind.USAGE, "a request is a JSON object")
    request_id, op, args = data.get("id"), data.get("op"), data.get("args", {})
    if not isinstance(request_id, int) or isinstance(request_id, bool) or request_id < 1:
        raise ControlError(ErrorKind.USAGE, "id must be a positive integer")
    if not isinstance(op, str) or not op:
        raise ControlError(ErrorKind.USAGE, "op must be a text")
    if not isinstance(args, dict):
        raise ControlError(ErrorKind.USAGE, "args must be an object")
    now = None
    if "now" in data:
        raw = data["now"]
        try:
            now = datetime.fromisoformat(str(raw).replace("Z", "+00:00")).astimezone(UTC)
        except ValueError as exc:
            raise ControlError(ErrorKind.USAGE, "now must be RFC 3339") from exc
    return Request(id=request_id, op=op, args=args, now=now)


def encode_reply(reply: Reply) -> str:
    """Encode a reply as one line without the line end.

    Args:
        reply: The reply.

    Returns:
        The JSON text.
    """
    if reply.ok:
        data: dict[str, Any] = {"id": reply.id, "ok": True, "result": reply.result}
    else:
        kind = reply.error_kind or ErrorKind.INTERNAL
        data = {
            "id": reply.id,
            "ok": False,
            "error": {"kind": kind.value, "detail": reply.error_detail},
        }
    return json.dumps(data, separators=(", ", ": "))


def format_now(now: datetime) -> str:
    """Format a time like the conductor does: RFC 3339 UTC with milliseconds.

    Args:
        now: A timezone aware time.

    Returns:
        The text, for example ``2030-01-01T00:00:00.000Z``.
    """
    return now.astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%S.") + f"{now.microsecond // 1000:03d}Z"
