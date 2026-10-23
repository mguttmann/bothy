# SPDX-License-Identifier: Apache-2.0
"""REST client: problem mapping, retries, idempotency, timeout, deprecation, SR9 and logs."""

from __future__ import annotations

import contextlib
import json
import logging
import random
import warnings
from collections.abc import Callable
from pathlib import Path
from uuid import UUID

import httpx
import pytest
from conftest import spec

from bothy_sdk import errors
from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import INVOCATION_TOKEN_HEADER, Client
from bothy_sdk.errors import (
    PROBLEM_STATUS,
    ConflictError,
    NotFoundError,
    ProblemError,
    RateLimitedError,
    problem_error,
)
from bothy_sdk.generated.api.agent_runtime import finish_run, start_run
from bothy_sdk.generated.api.runs import cancel_run, get_run
from bothy_sdk.generated.models import (
    BillingMode,
    RunCancel,
    RunFinish,
    RunFinishState,
    RunStart,
    ToolClass,
)
from bothy_sdk.retry import EXEMPT_ROUTES, backoff, is_retryable_request
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing.doubles import FakeClock

BASE = "https://example.com/api/v1"
CORRELATION = "0192f4a0-7b1c-7000-8a00-000000000009"
RUN = UUID("0192f4a0-7b1c-7000-8a00-000000000001")
CANARY = "canary-secret-token-777"
DISCOVERY = json.loads((Path(__file__).parent / "data" / "discovery.json").read_text())


def seeded(seed: int) -> random.Random:
    return random.Random(seed)  # noqa: S311 - deterministic jitter for the test


def _problem(status: int, code: str, **extra: object) -> dict[str, object]:
    return {
        "type": "about:blank",
        "title": code,
        "status": status,
        "code": code,
        "correlation_id": "0192f4a0-7b1c-7000-8a00-000000000009",
        **extra,
    }


def _client(handler: Callable[[httpx.Request], httpx.Response], **kw: object) -> Client:
    return Client(BASE, Credentials(Secret(CANARY)), transport=httpx.MockTransport(handler), **kw)  # type: ignore[arg-type]


def test_problem_mapping() -> None:
    codes = {
        c["code"]: c["status"]
        for c in spec()["components"]["schemas"]["Problem"]["x-problem-codes"]
    }
    assert dict(PROBLEM_STATUS) == codes
    for code, cls in errors._BY_CODE.items():
        assert code in codes, code
        err = problem_error(codes[code], _problem(codes[code], code))
        assert type(err) is cls
    unknown = problem_error(409, _problem(409, "brand_new_code"))
    assert type(unknown) is ConflictError
    assert unknown.code == "brand_new_code"
    assert type(problem_error(502, _problem(502, "bad_gateway"))) is errors.ServerError
    assert type(problem_error(418, {})) is ProblemError
    err = problem_error(403, _problem(403, "invocation_token_invalid", reason="expired"))
    expected = "403 invocation_token_invalid reason=expired correlation_id=" + CORRELATION
    assert str(err) == expected


def test_not_found_docstring_and_mapping() -> None:
    assert "not there or not visible" in (NotFoundError.__doc__ or "")
    assert type(problem_error(404, _problem(404, "not_found"))) is NotFoundError


def test_retry_policy() -> None:
    def req(method: str, path: str, **headers: str) -> httpx.Request:
        return httpx.Request(method, "https://example.com/api/v1" + path, headers=headers)

    assert is_retryable_request(req("GET", "/runs/x"))
    assert is_retryable_request(req("DELETE", "/commands/x"))
    assert is_retryable_request(req("POST", "/runs/x/cancel", **{"Idempotency-Key": "k"}))
    assert is_retryable_request(req("POST", "/threads/t/messages"))  # client_id
    assert is_retryable_request(req("POST", "/runs/r/start"))  # agent_runtime
    assert not is_retryable_request(req("POST", "/runs/r/cancel"))
    assert not is_retryable_request(req("POST", "/auth/login"))  # auth_flow is exempt, not retried
    rng = seeded(7)
    s = SdkSettings(retry_backoff_base=0.5, retry_backoff_cap=4.0)
    for attempt in range(8):
        assert 0 <= backoff(attempt, s, rng) <= min(4.0, 0.5 * 2**attempt)


@pytest.mark.anyio
async def test_retry_counts_and_retry_after() -> None:
    clock = FakeClock()
    seen: list[str] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request.method)
        if request.method == "GET":
            return httpx.Response(
                429, json=_problem(429, "rate_limited"), headers={"Retry-After": "7"}
            )
        return httpx.Response(500, json=_problem(500, "internal"))

    slept: list[float] = []
    real_sleep = clock.sleep

    async def record(seconds: float) -> None:
        slept.append(seconds)
        await clock.advance(seconds)

    clock.sleep = record  # type: ignore[method-assign]
    client = _client(
        handler, clock=clock, settings=SdkSettings(retry_max_attempts=3), rng=seeded(1)
    )
    with pytest.raises(RateLimitedError):
        await client.call(get_run.asyncio_detailed, run_id=RUN)
    assert seen == ["GET"] * 3
    assert all(s >= 7 for s in slept)
    seen.clear()
    body = RunCancel()
    with pytest.raises(errors.ServerError):
        await client.call(cancel_run.asyncio_detailed, run_id=RUN, body=body)
    assert len(seen) == 3  # an Idempotency-Key makes the cancel repeatable
    clock.sleep = real_sleep  # type: ignore[method-assign]
    await client.aclose()


def test_exempt_routes_match_spec() -> None:
    expected = set()
    for path, item in spec()["paths"].items():
        for method, op in item.items():
            if isinstance(op, dict) and "x-idempotency-exempt" in op:
                expected.add((method.upper(), path, op["x-idempotency-exempt"]))
    assert {(r.method, r.path, r.reason) for r in EXEMPT_ROUTES} == expected


@pytest.mark.anyio
async def test_idempotency_key_reused_across_retries() -> None:
    keys: list[str | None] = []

    def handler(request: httpx.Request) -> httpx.Response:
        keys.append(request.headers.get("Idempotency-Key"))
        if len(keys) < 3:
            return httpx.Response(503, json=_problem(503, "unavailable"))
        return httpx.Response(202, json={})

    clock = FakeClock()

    async def instant(seconds: float) -> None:
        await clock.advance(seconds)

    clock.sleep = instant  # type: ignore[method-assign]
    async with _client(handler, clock=clock) as client:
        with contextlib.suppress(ProblemError, TypeError, KeyError):
            await client.call(cancel_run.asyncio_detailed, run_id=RUN, body=RunCancel())
        assert len(keys) == 3
        assert keys[0] is not None
        assert len(set(keys)) == 1
        keys.clear()
        starts: list[httpx.Request] = []

        def no_key(request: httpx.Request) -> httpx.Response:
            starts.append(request)
            return httpx.Response(409, json=_problem(409, "already_started"))

        client2 = _client(no_key)
        body = RunStart(
            invocation_token="x",  # noqa: S106 - a placeholder, not a token
            billing_mode_used=BillingMode.ORGANIZATION,
            tool_class_used=ToolClass.CHAT,
        )
        with pytest.raises(errors.AlreadyStarted):
            await client2.call(start_run.asyncio_detailed, run_id=RUN, body=body)
        assert "Idempotency-Key" not in starts[0].headers  # agent_runtime routes are exempt
        await client2.aclose()


def test_timeout_from_settings() -> None:
    client = Client(BASE, Credentials(Secret(CANARY)), settings=SdkSettings(http_timeout=4.5))
    assert client.http.timeout == httpx.Timeout(4.5)


@pytest.mark.anyio
async def test_deprecation_warned_once_per_operation(caplog: pytest.LogCaptureFixture) -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json=DISCOVERY, headers={"Deprecation": "@1767225600"})

    async with _client(handler) as client:
        with warnings.catch_warnings(record=True) as caught:
            warnings.simplefilter("always")
            for _ in range(3):
                await client.discover()
    assert len([w for w in caught if issubclass(w.category, errors.ApiDeprecationWarning)]) == 1
    assert caplog.text.count("deprecated operation") == 1


@pytest.mark.anyio
async def test_invocation_header_only_inside_bind_run() -> None:
    seen: list[str | None] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request.headers.get(INVOCATION_TOKEN_HEADER))
        return httpx.Response(200, json=DISCOVERY)

    async with _client(handler) as client:
        await client.discover()
        with client.bind_run(RUN, Secret("token-one")):
            await client.discover()
            client.set_run_token(RUN, Secret("token-two"))
            await client.discover()
        await client.discover()
    assert seen == [None, "token-one", "token-two", None]


def test_invocation_header_name_matches_spec() -> None:
    param = spec()["components"]["parameters"]["InvocationToken"]
    assert (param["in"], param["name"]) == ("header", INVOCATION_TOKEN_HEADER)


@pytest.mark.anyio
async def test_logs_without_secrets(caplog: pytest.LogCaptureFixture) -> None:
    content = "secret content of a message 31337"

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            403, json=_problem(403, "invocation_token_invalid", reason="invalid", detail=content)
        )

    with caplog.at_level(logging.DEBUG):
        async with _client(handler) as client:
            with client.bind_run(RUN, Secret("bound-token-4242")):
                body = RunFinish(state=RunFinishState.FAILED, summary=content)
                with pytest.raises(errors.InvocationTokenInvalid) as info:
                    await client.call(finish_run.asyncio_detailed, run_id=RUN, body=body)
    text = caplog.text + str(info.value) + repr(info.value)
    for secret in (CANARY, "bound-token-4242", content):
        assert secret not in text
    assert "invocation_token_invalid" in caplog.text


@pytest.mark.parametrize(
    ("status", "body", "cls"),
    [
        (502, b"<html>bad gateway</html>", errors.ServerError),
        (
            503,
            b'{"code": "unavailable", "status": 503, "title": "x", "type": "about:blank"}',
            errors.Unavailable,
        ),
        (404, b"", NotFoundError),
    ],
)
@pytest.mark.anyio
async def test_undecodable_problem_body_maps_by_status(status: int, body: bytes, cls: type) -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(status, content=body, headers={"Content-Type": "text/html"})

    async with _client(handler, settings=SdkSettings(retry_max_attempts=1)) as client:
        with pytest.raises(cls):
            await client.call(get_run.asyncio_detailed, run_id=RUN)


def test_origin_url_keeps_the_token_on_the_api_origin() -> None:
    client = _client(lambda request: httpx.Response(200))
    assert client.origin_url("/api/v1/instance/jwks") == "https://example.com/api/v1/instance/jwks"
    assert client.origin_url("https://example.com/keys") == "https://example.com/keys"
    for foreign in (
        "http://example.com/api/v1/instance/jwks",
        "https://keys.example.org/jwks",
        "https://example.com:8443/jwks",
    ):
        with pytest.raises(ValueError, match="origin"):
            client.origin_url(foreign)
