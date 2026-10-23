# SPDX-License-Identifier: Apache-2.0
"""The synchronous hull `SyncClient` against ``httpx.MockTransport``.

docs/spec/11-extensions.md 11.3 and Design note 5 of the brief: the same discovery, problem
mapping, retry policy (with the injected blocking sleep), automatic ``Idempotency-Key``, SR9
header, deprecation warning and loopback rule as the asynchronous client.
"""

from __future__ import annotations

import json
import random
import warnings
from collections.abc import Callable, Iterator
from pathlib import Path
from typing import Any
from uuid import UUID

import httpx
import pytest

from bothy_sdk import errors
from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import IDEMPOTENCY_KEY_HEADER, INVOCATION_TOKEN_HEADER
from bothy_sdk.errors import ApiDeprecationWarning, ProblemError
from bothy_sdk.generated.api.agent_runtime import start_run
from bothy_sdk.generated.api.runs import cancel_run, get_run
from bothy_sdk.generated.models import (
    BillingMode,
    Discovery,
    RunCancel,
    RunStart,
    ToolClass,
)
from bothy_sdk.settings import SdkSettings
from bothy_sdk.sync import SyncClient
from bothy_sdk.testing import fingerprint

BASE = "https://example.com/api/v1"
RUN = UUID("0192f4a0-7b1c-7000-8a00-000000000001")
OTHER_RUN = UUID("0192f4a0-7b1c-7000-8a00-000000000002")
# Placeholders: the tests compare fingerprints, the values carry no meaning.
API_TOKEN = "example-api-token-for-sync"  # noqa: S105 - not a credential
RUN_TOKEN = "example-run-token-for-sync"  # noqa: S105 - not a credential
DISCOVERY = json.loads((Path(__file__).parent / "data" / "discovery.json").read_text())

Handler = Callable[[httpx.Request], httpx.Response]


def seeded(seed: int) -> random.Random:
    return random.Random(seed)  # noqa: S311 - deterministic jitter for the test


def _problem(status: int, code: str, **headers: str) -> httpx.Response:
    body = {
        "type": "about:blank",
        "title": code,
        "status": status,
        "code": code,
        "correlation_id": "0192f4a0-7b1c-7000-8a00-000000000009",
    }
    return httpx.Response(
        status, json=body, headers={"Content-Type": "application/problem+json", **headers}
    )


class _Recorder:
    """Answers with a queue of responses (the last repeats) and records every request."""

    def __init__(self, *answers: httpx.Response | Callable[[], httpx.Response]) -> None:
        self.answers = list(answers)
        self.requests: list[httpx.Request] = []

    def __call__(self, request: httpx.Request) -> httpx.Response:
        self.requests.append(request)
        answer = self.answers.pop(0) if len(self.answers) > 1 else self.answers[0]
        return answer() if callable(answer) else answer


def _sync(handler: Handler, *, sleeps: list[float] | None = None, **kw: Any) -> SyncClient:
    recorded = sleeps if sleeps is not None else []
    return SyncClient(
        BASE,
        Credentials(Secret(API_TOKEN)),
        transport=httpx.MockTransport(handler),
        sleep=recorded.append,
        rng=seeded(7),
        **kw,
    )


def _start_body() -> RunStart:
    return RunStart(
        invocation_token="x",  # noqa: S106 - a placeholder, not a token
        billing_mode_used=BillingMode.ORGANIZATION,
        tool_class_used=ToolClass.CHAT,
    )


@pytest.fixture
def no_warnings() -> Iterator[list[warnings.WarningMessage]]:
    """Record warnings instead of turning them into errors."""
    with warnings.catch_warnings(record=True) as caught:
        warnings.simplefilter("always")
        yield caught


# Discovery and problems ------------------------------------------------------------------------


def test_discover() -> None:
    recorder = _Recorder(httpx.Response(200, json=DISCOVERY))
    with _sync(recorder) as client:
        discovery = client.discover()
    assert isinstance(discovery, Discovery)
    assert str(discovery.instance_id) == DISCOVERY["instance_id"]
    [request] = recorder.requests
    assert request.method == "GET"
    assert request.url.path == "/api/v1/instance/discovery"
    assert "bothy-sdk/" in request.headers["User-Agent"]


def test_user_agent_names_the_adapter() -> None:
    recorder = _Recorder(httpx.Response(200, json=DISCOVERY))
    with _sync(recorder, user_agent="example-adapter/1.2.3") as client:
        client.discover()
    assert recorder.requests[0].headers["User-Agent"].startswith("example-adapter/1.2.3 bothy-sdk/")


@pytest.mark.parametrize(
    ("status", "code", "expected"),
    [
        (404, "not_found", errors.NotFoundError),
        (409, "already_started", errors.AlreadyStarted),
        (403, "invocation_token_invalid", errors.InvocationTokenInvalid),
        (400, "validation_failed", errors.ValidationFailed),
        (409, "brand_new_code", errors.ConflictError),
        (418, "brand_new_code", ProblemError),
    ],
)
def test_problem_mapping(status: int, code: str, expected: type[ProblemError]) -> None:
    recorder = _Recorder(_problem(status, code))
    with _sync(recorder) as client, pytest.raises(ProblemError) as raised:
        client.call(get_run.sync_detailed, run_id=RUN)
    assert type(raised.value) is expected
    assert raised.value.status == status
    assert raised.value.code == code
    assert raised.value.correlation_id is not None
    assert len(recorder.requests) == 1  # 4xx other than 409 request_in_progress is final


def test_call_as_narrows_and_refuses_other_models() -> None:
    recorder = _Recorder(httpx.Response(200, json=DISCOVERY))
    with _sync(recorder) as client:
        assert isinstance(client.call_as(Discovery, _discovery_op()), Discovery)
        with pytest.raises(TypeError):
            client.call_as(RunCancel, _discovery_op())


def _discovery_op() -> Callable[..., Any]:
    from bothy_sdk.generated.api.discovery import get_discovery

    return get_discovery.sync_detailed


# Retries ---------------------------------------------------------------------------------------


def test_get_retried_with_injected_sleep() -> None:
    sleeps: list[float] = []
    recorder = _Recorder(
        _problem(503, "unavailable"),
        _problem(500, "internal"),
        httpx.Response(200, json=DISCOVERY),
    )
    settings = SdkSettings(retry_backoff_base=0.5, retry_backoff_cap=30.0)
    with _sync(recorder, sleeps=sleeps, settings=settings) as client:
        client.discover()
    assert len(recorder.requests) == 3
    assert len(sleeps) == 2
    # Full jitter: uniform(0, min(cap, base * 2**n)).
    assert 0 <= sleeps[0] <= 0.5
    assert 0 <= sleeps[1] <= 1.0


def test_retry_after_respected_and_attempts_bounded() -> None:
    sleeps: list[float] = []
    recorder = _Recorder(_problem(429, "rate_limited", **{"Retry-After": "7"}))
    settings = SdkSettings(retry_max_attempts=3, retry_backoff_base=0.01, retry_backoff_cap=0.02)
    with (
        _sync(recorder, sleeps=sleeps, settings=settings) as client,
        pytest.raises(errors.RateLimitedError) as raised,
    ):
        client.discover()
    assert raised.value.retry_after == 7
    assert len(recorder.requests) == 3
    assert sleeps == [7.0, 7.0]


def test_setup_required_not_retried() -> None:
    sleeps: list[float] = []
    recorder = _Recorder(_problem(503, "setup_required"))
    with _sync(recorder, sleeps=sleeps) as client, pytest.raises(ProblemError):
        client.discover()
    assert len(recorder.requests) == 1
    assert sleeps == []


def test_network_error_retried_then_raised() -> None:
    sleeps: list[float] = []
    calls: list[httpx.Request] = []

    def refuse(request: httpx.Request) -> httpx.Response:
        calls.append(request)
        raise httpx.ConnectError("refused", request=request)

    settings = SdkSettings(retry_max_attempts=2)
    with (
        _sync(refuse, sleeps=sleeps, settings=settings) as client,
        pytest.raises(httpx.ConnectError),
    ):
        client.discover()
    assert len(calls) == 2
    assert len(sleeps) == 1


def test_agent_runtime_post_retried_without_key() -> None:
    sleeps: list[float] = []
    recorder = _Recorder(_problem(502, "bad_gateway"), _problem(409, "already_started"))
    with _sync(recorder, sleeps=sleeps) as client, pytest.raises(errors.AlreadyStarted):
        client.call(start_run.sync_detailed, run_id=RUN, body=_start_body())
    assert len(recorder.requests) == 2
    assert all(IDEMPOTENCY_KEY_HEADER not in r.headers for r in recorder.requests)


# Idempotency-Key -------------------------------------------------------------------------------


def test_idempotency_key_added_and_repeated() -> None:
    sleeps: list[float] = []
    recorder = _Recorder(
        _problem(503, "unavailable"),
        _problem(409, "request_in_progress"),
        _problem(404, "not_found"),
    )
    with _sync(recorder, sleeps=sleeps) as client, pytest.raises(errors.NotFoundError):
        client.call(cancel_run.sync_detailed, run_id=RUN, body=RunCancel())
    keys = [r.headers.get(IDEMPOTENCY_KEY_HEADER) for r in recorder.requests]
    assert len(keys) == 3
    assert keys[0] is not None
    assert UUID(keys[0]).version == 7
    assert keys == [keys[0]] * 3
    assert len(sleeps) == 2

    # The next logical call gets a new key.
    with _sync(recorder) as client, pytest.raises(errors.NotFoundError):
        client.call(cancel_run.sync_detailed, run_id=RUN, body=RunCancel())
    assert recorder.requests[-1].headers[IDEMPOTENCY_KEY_HEADER] != keys[0]


# SR9 -------------------------------------------------------------------------------------------


def test_invocation_header_only_inside_bind_run() -> None:
    recorder = _Recorder(httpx.Response(200, json=DISCOVERY))
    with _sync(recorder) as client:
        client.discover()
        with client.bind_run(RUN, Secret(RUN_TOKEN)):
            client.discover()
            with client.bind_run(OTHER_RUN, Secret("other-" + RUN_TOKEN)):
                client.discover()
            client.discover()
        client.discover()
    headers = [r.headers.get(INVOCATION_TOKEN_HEADER) for r in recorder.requests]
    assert headers[0] is None
    assert headers[4] is None
    assert [fingerprint(h) for h in headers[1:4] if h is not None] == [
        fingerprint(RUN_TOKEN),
        fingerprint("other-" + RUN_TOKEN),
        fingerprint(RUN_TOKEN),
    ]


def test_bind_run_released_on_exception() -> None:
    recorder = _Recorder(httpx.Response(200, json=DISCOVERY))
    with _sync(recorder) as client:
        with pytest.raises(RuntimeError), client.bind_run(RUN, Secret(RUN_TOKEN)):
            raise RuntimeError("handler failed")
        client.discover()
    assert INVOCATION_TOKEN_HEADER not in recorder.requests[0].headers


# Deprecation -----------------------------------------------------------------------------------


def test_deprecation_warned_once(
    no_warnings: list[warnings.WarningMessage], caplog: pytest.LogCaptureFixture
) -> None:
    deprecated = httpx.Response(
        200,
        json=DISCOVERY,
        headers={"Deprecation": "true", "Sunset": "Wed, 30 Jun 2027 00:00:00 GMT"},
    )
    recorder = _Recorder(deprecated)
    with caplog.at_level("WARNING", logger="bothy_sdk"), _sync(recorder) as client:
        client.discover()
        client.discover()
        client.discover()
    found = [w for w in no_warnings if issubclass(w.category, ApiDeprecationWarning)]
    assert len(found) == 1
    assert issubclass(ApiDeprecationWarning, DeprecationWarning)
    assert "GET /api/v1/instance/discovery" in str(found[0].message)
    assert len([r for r in caplog.records if "deprecated" in r.getMessage()]) == 1


def test_deprecation_warned_once_per_path_template(
    no_warnings: list[warnings.WarningMessage],
) -> None:
    run = json.loads(
        (Path(__file__).parents[1] / "src/bothy_sdk/testing/fixtures/run.json").read_text()
    )
    recorder = _Recorder(httpx.Response(200, json=run, headers={"Sunset": "x"}))
    with _sync(recorder) as client:
        client.call(get_run.sync_detailed, run_id=RUN)
        client.call(get_run.sync_detailed, run_id=OTHER_RUN)
    found = [w for w in no_warnings if issubclass(w.category, ApiDeprecationWarning)]
    assert len(found) == 1
    assert "/api/v1/runs/{id}" in str(found[0].message)


# Transport rules -------------------------------------------------------------------------------


@pytest.mark.parametrize(
    "url",
    ["http://127.0.0.1:8443/api/v1", "http://localhost/api/v1", "http://[::1]:9/api/v1", BASE],
)
def test_cleartext_allowed_only_on_loopback(url: str) -> None:
    with SyncClient(url, Credentials(Secret(API_TOKEN))) as client:
        assert client.base_url == url


@pytest.mark.parametrize(
    "url", ["http://example.com/api/v1", "http://192.0.2.1/api/v1", "ftp://example.com/api"]
)
def test_cleartext_refused_off_loopback(url: str) -> None:
    with pytest.raises(ValueError, match="loopback"):
        SyncClient(url, Credentials(Secret(API_TOKEN)))


def test_timeout_from_settings() -> None:
    with SyncClient(
        BASE, Credentials(Secret(API_TOKEN)), settings=SdkSettings(http_timeout=4.5)
    ) as client:
        timeout = client._http.timeout
    assert (timeout.connect, timeout.read, timeout.write, timeout.pool) == (4.5, 4.5, 4.5, 4.5)


def test_token_not_in_repr_or_logs(caplog: pytest.LogCaptureFixture) -> None:
    recorder = _Recorder(_problem(404, "not_found"))
    with caplog.at_level("DEBUG"), _sync(recorder) as client:
        assert API_TOKEN not in repr(client.auth)
        with client.bind_run(RUN, Secret(RUN_TOKEN)), pytest.raises(errors.NotFoundError) as raised:
            client.call(get_run.sync_detailed, run_id=RUN)
    assert API_TOKEN not in str(raised.value)
    assert RUN_TOKEN not in str(raised.value)
    assert API_TOKEN not in caplog.text
    assert RUN_TOKEN not in caplog.text
