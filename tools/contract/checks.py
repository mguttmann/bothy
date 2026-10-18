# SPDX-License-Identifier: Apache-2.0
"""The project checks of G7 (06 "Contract tests and gate").

Problem format, ``Correlation-Id``, no JSON ``null`` (ADR 0053) and 404 equality for foreign and
missing IDs (R-AZ-01).

Every check takes an :class:`Answer` and raises :class:`ContractError` with a reason; the
live module wraps them as Schemathesis checks.
"""

from __future__ import annotations

import contextlib
import json
import re
from collections.abc import Iterator, Mapping
from dataclasses import dataclass
from typing import Any

PROBLEM_MEDIA_TYPE = "application/problem+json"
PROBLEM_TYPE_PREFIX = "/api/v1/problems/"
PROBLEM_REQUIRED = ("type", "title", "status", "code", "correlation_id")
CORRELATION_HEADER = "Correlation-Id"
# Schema CorrelationId: the W3C trace ID written as a lower-case UUID (06 "Basic rules", row
# Correlation: not a UUIDv7).
_UUID = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
_ANY_UUID = re.compile(
    r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
)
# Headers whose values differ between two equal answers (404 equality).
VOLATILE_HEADERS = frozenset(
    {"correlation-id", "date", "dpop-nonce", "ratelimit", "ratelimit-policy"}
)
JSON_MEDIA_TYPES = frozenset({"application/json", PROBLEM_MEDIA_TYPE})
NDJSON_MEDIA_TYPE = "application/x-ndjson"


class ContractError(AssertionError):
    """A response that breaks a project check."""


@dataclass(frozen=True)
class Answer:
    """One HTTP answer: status, headers (any case) and body."""

    status: int
    headers: Mapping[str, str]
    body: bytes

    def header(self, name: str) -> str | None:
        """Return the value of header ``name`` regardless of case, or None."""
        wanted = name.lower()
        for key, value in self.headers.items():
            if key.lower() == wanted:
                return value
        return None

    @property
    def media_type(self) -> str:
        """Return the media type of Content-Type without parameters, lower case."""
        return (self.header("Content-Type") or "").split(";")[0].strip().lower()


def problem_codes(spec: Mapping[str, Any]) -> dict[str, int]:
    """Return code -> status of ``x-problem-codes`` on the schema Problem (06 "Errors")."""
    problem = spec["components"]["schemas"]["Problem"]
    return {str(entry["code"]): int(entry["status"]) for entry in problem["x-problem-codes"]}


def problem_format(answer: Answer, codes: Mapping[str, int]) -> None:
    """Every answer with status 400 or higher is a problem of 06 "Errors".

    It has ``application/problem+json``, the required members of Problem, ``type`` equal to
    ``/api/v1/problems/<code>``, ``status`` equal to the HTTP status and a known ``code``.
    """
    if answer.status < 400:
        return
    if answer.media_type != PROBLEM_MEDIA_TYPE:
        raise ContractError(
            f"status {answer.status} with media type {answer.media_type!r}, "
            f"want {PROBLEM_MEDIA_TYPE}"
        )
    body = _json_object(answer)
    missing = [name for name in PROBLEM_REQUIRED if name not in body]
    if missing:
        raise ContractError("problem lacks " + ", ".join(missing))
    code = body["code"]
    if code not in codes:
        raise ContractError(f"problem code {code!r} is not in the table of 06 Errors")
    if body["type"] != PROBLEM_TYPE_PREFIX + str(code):
        raise ContractError(f"problem type {body['type']!r} does not match code {code!r}")
    if body["status"] != answer.status:
        raise ContractError(
            f"problem status {body['status']!r} differs from HTTP status {answer.status}"
        )


def correlation_id(answer: Answer) -> None:
    """Every answer has ``Correlation-Id`` (schema CorrelationId); a problem repeats it."""
    value = answer.header(CORRELATION_HEADER)
    if value is None:
        raise ContractError("no Correlation-Id header")
    if not _UUID.match(value):
        raise ContractError(f"Correlation-Id {value!r} is not a lower-case UUID")
    if answer.media_type == PROBLEM_MEDIA_TYPE:
        body = _json_object(answer)
        if body.get("correlation_id") != value:
            raise ContractError(
                f"problem correlation_id {body.get('correlation_id')!r} "
                f"differs from the header {value!r}"
            )


def no_json_null(answer: Answer) -> None:
    """No JSON body contains ``null`` at any depth (ADR 0053); NDJSON line by line."""
    for document in _json_documents(answer):
        path = _find_null(document, "")
        if path is not None:
            raise ContractError(f"JSON null at {path or '/'}")


def normalize_not_found(answer: Answer) -> tuple[int, tuple[tuple[str, str], ...], Any]:
    """Return ``answer`` without the values that differ between two equal 404 answers.

    Drops the headers Correlation-Id, Date, DPoP-Nonce, RateLimit and RateLimit-Policy, the member
    ``correlation_id`` and replaces every UUID in ``instance``.
    """
    headers = tuple(
        sorted(
            (k.lower(), v) for k, v in answer.headers.items() if k.lower() not in VOLATILE_HEADERS
        )
    )
    body: Any = answer.body.decode("utf-8", "replace")
    if answer.media_type in JSON_MEDIA_TYPES:
        with contextlib.suppress(ValueError):
            body = json.loads(answer.body)
    if isinstance(body, dict):
        body = {k: v for k, v in body.items() if k != "correlation_id"}
        if isinstance(body.get("instance"), str):
            body["instance"] = _ANY_UUID.sub("<id>", body["instance"])
    return answer.status, headers, body


def not_found_equality(first: Answer, second: Answer) -> None:
    """A 404 for one ID equals the 404 for fresh random IDs after normalization (R-AZ-01)."""
    if first.status != 404:
        return
    a, b = normalize_not_found(first), normalize_not_found(second)
    if a != b:
        raise ContractError(f"404 answers differ: {a!r} versus {b!r}")


def _json_object(answer: Answer) -> dict[str, Any]:
    try:
        body = json.loads(answer.body)
    except ValueError as exc:
        raise ContractError(f"body is not JSON: {exc}") from None
    if not isinstance(body, dict):
        raise ContractError("body is not a JSON object")
    return body


def _json_documents(answer: Answer) -> Iterator[Any]:
    if answer.media_type == NDJSON_MEDIA_TYPE:
        lines = [line for line in answer.body.splitlines() if line.strip()]
    elif answer.media_type in JSON_MEDIA_TYPES and answer.body:
        lines = [answer.body]
    else:
        return
    for line in lines:
        try:
            yield json.loads(line)
        except ValueError:
            raise ContractError("body is not JSON") from None


def _find_null(value: Any, path: str) -> str | None:
    if value is None:
        return path
    if isinstance(value, dict):
        for key, item in value.items():
            found = _find_null(item, f"{path}/{key}")
            if found is not None:
                return found
    elif isinstance(value, list):
        for index, item in enumerate(value):
            found = _find_null(item, f"{path}/{index}")
            if found is not None:
                return found
    return None
