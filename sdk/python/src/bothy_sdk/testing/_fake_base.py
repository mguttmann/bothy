# SPDX-License-Identifier: Apache-2.0
"""Shared pieces of the FakeServer: problems, times, timings and JSON helpers (private)."""

from __future__ import annotations

import base64
import json
from collections.abc import Mapping
from datetime import UTC, datetime
from typing import Any

from bothy_sdk.errors import PROBLEM_STATUS
from bothy_sdk.ids import uuid7

# The server keys the FakeServer plays, with the defaults of its tests (seconds or counts).
_TIMING_DEFAULTS: tuple[tuple[str, float], ...] = (
    ("agent.invocation_token_ttl", 60),
    ("auth.clock_leeway", 30),
    ("agent.ack_timeout", 30),
    ("agent.ack_persist_interval", 1),
    ("sync.heartbeat_interval", 30),
    ("sync.resume_window", 300),
    ("agent.max_inflight_events", 100),
    ("command.autocomplete_timeout", 3),
    ("agent.jwks_max_age", 300),
    ("agent.cancel_grace", 30),
    ("auth.dpop_nonce_ttl", 300),
    ("auth.dpop_proof_max_age", 60),
)
DEFAULT_TIMINGS: Mapping[str, float] = dict(_TIMING_DEFAULTS)


_TITLE_PAIRS: tuple[tuple[str, str], ...] = (
    ("validation_failed", "Validation failed"),
    ("unauthenticated", "Unauthenticated"),
    ("use_dpop_nonce", "Use DPoP nonce"),
    ("invalid_dpop_proof", "Invalid DPoP proof"),
    ("invocation_token_invalid", "Invocation token invalid"),
    ("agent_invocation_denied", "Agent invocation denied"),
    ("forbidden", "Forbidden"),
    ("not_found", "Not found"),
    ("already_started", "Already started"),
    ("invalid_state_transition", "Invalid state transition"),
    ("conflict", "Conflict"),
    ("base_version_conflict", "Base version conflict"),
    ("billing_mode_mismatch", "Billing mode mismatch"),
    ("tool_class_mismatch", "Tool class mismatch"),
    ("version_conflict", "Version conflict"),
    ("precondition_required", "Precondition required"),
    ("idempotency_key_reuse", "Idempotency key reuse"),
    ("request_in_progress", "Request in progress"),
    ("rate_limited", "Rate limited"),
    ("resync_required", "Resync required"),
    ("gone", "Gone"),
    ("internal_error", "Internal error"),
    ("bad_gateway", "Bad gateway"),
)
_TITLES = dict(_TITLE_PAIRS)


def timings_with_defaults(overrides: Mapping[str, float] | None) -> dict[str, float]:
    """Merge ``overrides`` into the default timings.

    Raises:
        ValueError: A key is not one the FakeServer plays.
    """
    merged = dict(DEFAULT_TIMINGS)
    for key, value in (overrides or {}).items():
        if key not in DEFAULT_TIMINGS:
            raise ValueError(f"unknown FakeServer timing: {key}")
        merged[key] = value
    return merged


class FakeProblem(Exception):  # noqa: N818 - it is the answer, not an error of the fake itself
    """An error answer in the problem format; raised by a rule, rendered by the router."""

    def __init__(  # noqa: PLR0913 - the members of the schema Problem
        self,
        code: str,
        *,
        status: int | None = None,
        detail: str | None = None,
        reason: str | None = None,
        errors: list[dict[str, str]] | None = None,
        current: Mapping[str, Any] | None = None,
        headers: Mapping[str, str] | None = None,
    ) -> None:
        super().__init__(code)
        self.code = code
        self.status = status or PROBLEM_STATUS.get(code, 500)
        self.detail = detail
        self.reason = reason
        self.errors = errors
        self.current = current
        self.headers = dict(headers or {})

    def body(self, instance: str) -> dict[str, Any]:
        """The problem body, without empty members."""
        out: dict[str, Any] = {
            "type": f"/api/v1/problems/{self.code}",
            "title": _TITLES.get(self.code, self.code.replace("_", " ").capitalize()),
            "status": self.status,
            "code": self.code,
            "correlation_id": str(uuid7()),
            "instance": instance,
        }
        for name in ("detail", "errors", "current", "reason"):
            value = getattr(self, name)
            if value:
                out[name] = value
        return out


def field_error(pointer: str, code: str, detail: str = "") -> FakeProblem:
    """A ``400 validation_failed`` with one field error."""
    error = {"pointer": pointer, "code": code}
    if detail:
        error["detail"] = detail
    return FakeProblem("validation_failed", errors=[error])


def iso(at: datetime) -> str:
    """UTC time with milliseconds and ``Z``, as the run views of the server write it."""
    return at.astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%S.") + f"{at.microsecond // 1000:03d}Z"


def b64url(data: bytes) -> str:
    """Base64url without padding."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def b64url_decode(text: str) -> bytes:
    """Strict base64url without padding.

    Raises:
        ValueError: Padding or a character outside the alphabet.
    """
    if "=" in text or any(c not in _B64URL for c in text):
        raise ValueError("not base64url")
    return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))


_B64URL = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_")


def compact_json(value: object) -> bytes:
    """JSON without spaces, the way the server encodes JOSE parts."""
    return json.dumps(value, separators=(",", ":")).encode("utf-8")


def strict_object(raw: bytes) -> dict[str, Any]:
    """Decode a JSON object and refuse duplicate members.

    Raises:
        ValueError: Not a JSON object, or a member appears twice.
    """

    def pairs(items: list[tuple[str, Any]]) -> dict[str, Any]:
        out: dict[str, Any] = {}
        for key, value in items:
            if key in out:
                raise ValueError("duplicate member")
            out[key] = value
        return out

    value = json.loads(raw, object_pairs_hook=pairs)
    if not isinstance(value, dict):
        raise ValueError("not an object")
    return value
