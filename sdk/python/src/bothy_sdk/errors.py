# SPDX-License-Identifier: Apache-2.0
"""Typed errors after the problem format of RFC 9457 (docs/spec/06-api.md 5).

`ProblemError` carries what a ``Problem`` body says; one subclass per HTTP status family and,
below them, one class per code the hand layer reacts to. An unknown code maps to the class of
its status ("Clients treat unknown codes by HTTP status", schema ``Problem``). A ``404`` means
"not there or not visible" and is never treated as surely missing (docs/spec/11-extensions.md
11.2). The text of an error shows status, code, reason and ``correlation_id``, never a token or a
message content (SEC-LOG-2).

Examples:
    >>> err = problem_error(409, {"code": "already_started", "title": "Already started",
    ...     "status": 409, "correlation_id": "0192f4a0-7b1c-7000-8a00-000000000001"})
    >>> (type(err).__name__, isinstance(err, ConflictError))
    ('AlreadyStarted', True)
"""

from __future__ import annotations

import json
from collections.abc import Mapping, Sequence
from typing import Any

from bothy_sdk.generated.models.field_error import FieldError
from bothy_sdk.generated.models.problem import Problem
from bothy_sdk.generated.models.run import Run

__all__ = [
    "PROBLEM_STATUS",
    "AgentInvocationDenied",
    "AlreadyStarted",
    "ApiDeprecationWarning",
    "BadRequestError",
    "BaseVersionConflict",
    "BillingModeMismatch",
    "ConflictError",
    "ForbiddenError",
    "GatewayOpError",
    "GoneError",
    "InvocationTokenInvalid",
    "NotFoundError",
    "PreconditionFailedError",
    "ProblemError",
    "RateLimitedError",
    "RequestInProgress",
    "ServerError",
    "ToolClassMismatch",
    "UnauthenticatedError",
    "Unavailable",
    "UseDpopNonce",
    "ValidationFailed",
    "VersionConflict",
    "problem_error",
    "unparsable_problem",
]

# Every code of `x-problem-codes` of the schema Problem in docs/api/openapi.yaml with its status;
# tests/test_client.py holds this table equal to the specification.
_PROBLEM_CODES: tuple[tuple[str, int], ...] = (
    ("cursor_invalid", 400),
    ("malformed_request", 400),
    ("validation_failed", 400),
    ("webhook_host_not_allowed", 400),
    ("invalid_dpop_proof", 401),
    ("session_revoked", 401),
    ("step_up_required", 401),
    ("token_expired", 401),
    ("unauthenticated", 401),
    ("use_dpop_nonce", 401),
    ("agent_invocation_denied", 403),
    ("channel_read_only", 403),
    ("device_pending", 403),
    ("feature_disabled", 403),
    ("forbidden", 403),
    ("invocation_token_invalid", 403),
    ("scope_missing", 403),
    ("terminal_denied", 403),
    ("not_found", 404),
    ("method_not_allowed", 405),
    ("already_started", 409),
    ("base_version_conflict", 409),
    ("billing_mode_mismatch", 409),
    ("component_used", 409),
    ("conflict", 409),
    ("file_blocked", 409),
    ("file_not_ready", 409),
    ("fingerprint_mismatch", 409),
    ("host_unavailable", 409),
    ("i1_violation", 409),
    ("i6_violation", 409),
    ("invalid_state_transition", 409),
    ("last_owner", 409),
    ("operator_mismatch", 409),
    ("request_in_progress", 409),
    ("title_manual", 409),
    ("tool_class_mismatch", 409),
    ("gone", 410),
    ("message_deleted", 410),
    ("resync_required", 410),
    ("version_conflict", 412),
    ("payload_too_large", 413),
    ("unsupported_media_type", 415),
    ("range_not_satisfiable", 416),
    ("idempotency_key_reuse", 422),
    ("precondition_required", 428),
    ("rate_limited", 429),
    ("internal_error", 500),
    ("maintenance", 503),
    ("setup_required", 503),
    ("unavailable", 503),
)
PROBLEM_STATUS: Mapping[str, int] = dict(_PROBLEM_CODES)


class ApiDeprecationWarning(DeprecationWarning):
    """The server marked an operation with ``Deprecation`` or ``Sunset`` (11 12)."""


class ProblemError(Exception):
    """A failed API call, decoded from its ``Problem`` body.

    Attributes:
        status: The HTTP status.
        code: The stable machine code (may be a code this SDK does not know).
        reason: The optional finer reason (for example ``expired``).
        title: The short title.
        detail: The optional detail text.
        correlation_id: The correlation ID for support, as text.
        errors: Field errors of ``400 validation_failed``.
        current: The current resource of a conflict, as a raw mapping.
        retry_after: Seconds of ``Retry-After``, if the answer had it.
    """

    def __init__(  # noqa: PLR0913 - the members of RFC 9457 plus the extensions of 06 5
        self,
        status: int,
        code: str,
        *,
        reason: str | None = None,
        title: str = "",
        detail: str | None = None,
        correlation_id: str | None = None,
        errors: Sequence[FieldError] = (),
        current: Mapping[str, Any] | None = None,
        retry_after: float | None = None,
    ) -> None:
        """Create the error; prefer `problem_error`."""
        self.status = status
        self.code = code
        self.reason = reason
        self.title = title
        self.detail = detail
        self.correlation_id = correlation_id
        self.errors = list(errors)
        self.current = current
        self.retry_after = retry_after
        super().__init__(self._text())

    def _text(self) -> str:
        parts = [f"{self.status} {self.code}"]
        if self.reason:
            parts.append(f"reason={self.reason}")
        if self.correlation_id:
            parts.append(f"correlation_id={self.correlation_id}")
        return " ".join(parts)

    def __str__(self) -> str:
        """Return status, code, reason and ``correlation_id``."""
        return self._text()


class BadRequestError(ProblemError):
    """HTTP 400."""


class UnauthenticatedError(ProblemError):
    """HTTP 401."""


class ForbiddenError(ProblemError):
    """HTTP 403."""


class NotFoundError(ProblemError):
    """HTTP 404: the resource is not there or not visible.

    The server answers ``404`` for a target the caller may not see as for one that does not
    exist (docs/spec/07-permissions.md), so this error never proves that a resource is missing.
    """


class ConflictError(ProblemError):
    """HTTP 409."""


class GoneError(ProblemError):
    """HTTP 410."""


class PreconditionFailedError(ProblemError):
    """HTTP 412."""


class RateLimitedError(ProblemError):
    """HTTP 429."""


class ServerError(ProblemError):
    """HTTP 5xx."""


class ValidationFailed(BadRequestError):  # noqa: N818 - named after the problem code
    """``400 validation_failed``; `errors` names the fields (``pointer``, ``code``)."""


class UseDpopNonce(UnauthenticatedError):  # noqa: N818 - named after the problem code
    """``401 use_dpop_nonce`` after the one retry with the new nonce."""


class InvocationTokenInvalid(ForbiddenError):  # noqa: N818 - named after the problem code
    """``403 invocation_token_invalid`` with its `reason`.

    The reason is ``expired``, ``superseded``, ``invalid`` or ``run_inactive``
    (docs/spec/09-agents.md 4.5, 07 SR9).
    """


class AgentInvocationDenied(ForbiddenError):  # noqa: N818 - named after the problem code
    """``403 agent_invocation_denied``: the renewal check failed and the server ended the run."""


class AlreadyStarted(ConflictError):  # noqa: N818 - named after the problem code
    """``409 already_started``: the run started elsewhere or is no longer waiting."""


class BillingModeMismatch(ConflictError):  # noqa: N818 - named after the problem code
    """``409 billing_mode_mismatch``: the reported mode differs; the server failed the run."""


class ToolClassMismatch(ConflictError):  # noqa: N818 - named after the problem code
    """``409 tool_class_mismatch``: the reported tool class is above the claim."""


class BaseVersionConflict(ConflictError):  # noqa: N818 - named after the problem code
    """``409 base_version_conflict``; `run` is the current document decoded as ``Run``."""

    @property
    def run(self) -> Run | None:
        """The run of ``current`` as the generated model, if the answer had it."""
        if self.current is None:
            return None
        return Run.from_dict(dict(self.current))


class VersionConflict(PreconditionFailedError):  # noqa: N818 - named after the problem code
    """``412 version_conflict``: ``If-Match`` names an outdated version."""


class RequestInProgress(ConflictError):  # noqa: N818 - named after the problem code
    """``409 request_in_progress``: the same ``Idempotency-Key`` is still being processed."""


class Unavailable(ServerError):  # noqa: N818 - named after the problem code
    """``503 unavailable``."""


class GatewayOpError(ProblemError):
    """An ``error`` frame of the gateway for one op (04 6.2).

    Attributes:
        op: The op that failed.
        ref: The ``ref`` the client sent with the op, if any.
    """

    def __init__(self, op: str, code: str, detail: str | None = None, ref: str | None = None):
        """Create the error; the status comes from `PROBLEM_STATUS` (0 for an unknown code)."""
        self.op = op
        self.ref = ref
        super().__init__(PROBLEM_STATUS.get(code, 0), code, detail=detail)

    def _text(self) -> str:
        return f"gateway op {self.op}: {super()._text()}"


_BY_CODE: Mapping[str, type[ProblemError]] = {
    "validation_failed": ValidationFailed,
    "use_dpop_nonce": UseDpopNonce,
    "invocation_token_invalid": InvocationTokenInvalid,
    "agent_invocation_denied": AgentInvocationDenied,
    "already_started": AlreadyStarted,
    "billing_mode_mismatch": BillingModeMismatch,
    "tool_class_mismatch": ToolClassMismatch,
    "base_version_conflict": BaseVersionConflict,
    "version_conflict": VersionConflict,
    "request_in_progress": RequestInProgress,
    "unavailable": Unavailable,
}

_BY_STATUS: Mapping[int, type[ProblemError]] = {
    400: BadRequestError,
    401: UnauthenticatedError,
    403: ForbiddenError,
    404: NotFoundError,
    409: ConflictError,
    410: GoneError,
    412: PreconditionFailedError,
    429: RateLimitedError,
}


def error_class(status: int, code: str) -> type[ProblemError]:
    """Return the class for ``code``, else for the status family of ``status``.

    Args:
        status: The HTTP status.
        code: The problem code.

    Returns:
        The most specific class.
    """
    cls = _BY_CODE.get(code)
    if cls is not None and PROBLEM_STATUS.get(code) == status:
        return cls
    if status >= 500:
        return ServerError
    return _BY_STATUS.get(status, ProblemError)


def _text(value: object) -> str | None:
    return value if isinstance(value, str) and value else None


def unparsable_problem(status: int, body: bytes, retry_after: float | None) -> ProblemError | None:
    """Return the error of a failed answer whose body the generated core cannot decode.

    A proxy can answer ``502`` with HTML, or a problem can miss a member the schema requires;
    the generated parser would then raise ``ValueError`` or ``KeyError`` instead of a typed
    error. Such an answer becomes the error class of its status (06 5: unknown codes by status).

    Args:
        status: The HTTP status (at least ``400``).
        body: The raw body.
        retry_after: Seconds of ``Retry-After``.

    Returns:
        The error to raise, or ``None`` when the generated ``Problem`` decodes the body.
    """
    try:
        data = json.loads(body) if body else None
    except ValueError:
        data = None
    if isinstance(data, Mapping):
        try:
            Problem.from_dict(dict(data))
        except (KeyError, TypeError, ValueError):
            pass
        else:
            return None
    return problem_error(status, data, retry_after=retry_after)


def problem_error(status: int, body: object, *, retry_after: float | None = None) -> ProblemError:
    """Build the typed error of a failed answer.

    Args:
        status: The HTTP status.
        body: The decoded JSON body; anything that is not a problem object gives a code
            ``unknown``.
        retry_after: Seconds of ``Retry-After``.

    Returns:
        The error, not raised.
    """
    data: Mapping[str, Any] = body if isinstance(body, Mapping) else {}
    code = _text(data.get("code")) or "unknown"
    raw_errors = data.get("errors")
    errors: list[FieldError] = []
    if isinstance(raw_errors, list):
        for item in raw_errors:
            if isinstance(item, Mapping):
                try:
                    errors.append(FieldError.from_dict(dict(item)))
                except (KeyError, TypeError, ValueError):
                    continue
    current = data.get("current")
    cls = error_class(status, code)
    return cls(
        status,
        code,
        reason=_text(data.get("reason")),
        title=_text(data.get("title")) or "",
        detail=_text(data.get("detail")),
        correlation_id=_text(data.get("correlation_id")),
        errors=errors,
        current=current if isinstance(current, Mapping) else None,
        retry_after=retry_after,
    )
