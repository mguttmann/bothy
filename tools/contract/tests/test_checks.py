# SPDX-License-Identifier: Apache-2.0
"""Tests of the project checks of G7: each detects its negative and accepts its positive fixture."""

from __future__ import annotations

import json
from typing import Any

import checks
import pytest

CID = "4bf92f35-77b3-4da6-a3ce-929d0e0e4736"
CODES = {"not_found": 404, "validation_failed": 400}


def _problem(status: int = 404, **override: Any) -> checks.Answer:
    body: dict[str, Any] = {
        "type": "/api/v1/problems/not_found",
        "title": "Not found",
        "status": 404,
        "code": "not_found",
        "correlation_id": CID,
        "instance": "/api/v1/threads/0192f4a0-7b1c-7f89-8d6b-6960453241bc",
    }
    body.update(override)
    body = {k: v for k, v in body.items() if v is not ...}
    return checks.Answer(
        status,
        {"Content-Type": "application/problem+json", "Correlation-Id": CID},
        json.dumps(body).encode(),
    )


def test_problem_format_accepts_a_problem_and_success() -> None:
    checks.problem_format(_problem(), CODES)
    checks.problem_format(checks.Answer(200, {"Content-Type": "text/plain"}, b"x"), CODES)


@pytest.mark.parametrize(
    ("answer", "reason"),
    [
        (checks.Answer(404, {"Content-Type": "application/json"}, b"{}"), "media type"),
        (_problem(code=...), "lacks code"),
        (_problem(code="nope", type="/api/v1/problems/nope"), "not in the table"),
        (_problem(type="/api/v1/problems/validation_failed"), "does not match"),
        (_problem(status=400), "differs from HTTP status"),
        (
            checks.Answer(404, {"Content-Type": "application/problem+json"}, b"[1]"),
            "not a JSON object",
        ),
        (checks.Answer(404, {"Content-Type": "application/problem+json"}, b"{"), "not JSON"),
    ],
)
def test_problem_format_rejects(answer: checks.Answer, reason: str) -> None:
    with pytest.raises(checks.ContractError, match=reason):
        checks.problem_format(answer, CODES)


def test_correlation_id() -> None:
    checks.correlation_id(_problem())
    checks.correlation_id(checks.Answer(200, {"correlation-id": CID}, b""))
    with pytest.raises(checks.ContractError, match="no Correlation-Id"):
        checks.correlation_id(checks.Answer(200, {}, b""))
    with pytest.raises(checks.ContractError, match="not a lower-case UUID"):
        checks.correlation_id(checks.Answer(200, {"Correlation-Id": CID.upper()}, b""))
    with pytest.raises(checks.ContractError, match="differs from the header"):
        checks.correlation_id(_problem(correlation_id="0192f4a0-7b1c-7f89-8d6b-6960453241bc"))


def test_no_json_null() -> None:
    checks.no_json_null(
        checks.Answer(200, {"Content-Type": "application/json"}, b'{"a":[1,{"b":"c"}]}')
    )
    checks.no_json_null(checks.Answer(200, {"Content-Type": "text/plain"}, b"null"))
    with pytest.raises(checks.ContractError, match="null at /a/1/b"):
        checks.no_json_null(
            checks.Answer(200, {"Content-Type": "application/json"}, b'{"a":[1,{"b":null}]}')
        )
    with pytest.raises(checks.ContractError, match="null at /"):
        checks.no_json_null(
            checks.Answer(200, {"Content-Type": "application/json; charset=utf-8"}, b"null")
        )
    with pytest.raises(checks.ContractError, match="null at /x"):
        checks.no_json_null(
            checks.Answer(200, {"Content-Type": "application/x-ndjson"}, b'{"a":1}\n{"x":null}\n')
        )
    with pytest.raises(checks.ContractError, match="not JSON"):
        checks.no_json_null(checks.Answer(200, {"Content-Type": "application/json"}, b"{"))


def test_not_found_equality() -> None:
    other = _problem(
        correlation_id="5bf92f35-77b3-4da6-a3ce-929d0e0e4736",
        instance="/api/v1/threads/0192f4a0-7b1c-7f89-8d6b-000000000000",
    )
    other_headers = dict(
        other.headers, **{"Correlation-Id": "x", "DPoP-Nonce": "n", "RateLimit": "r", "Date": "d"}
    )
    checks.not_found_equality(_problem(), checks.Answer(404, other_headers, other.body))
    checks.not_found_equality(checks.Answer(200, {}, b"a"), checks.Answer(404, {}, b"b"))
    with pytest.raises(checks.ContractError, match="404 answers differ"):
        checks.not_found_equality(_problem(), _problem(detail="exists but hidden"))
    with pytest.raises(checks.ContractError, match="404 answers differ"):
        checks.not_found_equality(
            _problem(), checks.Answer(403, _problem().headers, _problem().body)
        )


def test_problem_codes() -> None:
    spec = {
        "components": {
            "schemas": {"Problem": {"x-problem-codes": [{"code": "gone", "status": 410}]}}
        }
    }
    assert checks.problem_codes(spec) == {"gone": 410}


def test_normalize_keeps_non_json() -> None:
    status, headers, body = checks.normalize_not_found(checks.Answer(404, {"X-A": "1"}, b"plain"))
    assert (status, headers, body) == (404, (("x-a", "1"),), "plain")
