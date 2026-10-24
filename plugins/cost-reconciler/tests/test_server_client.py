# SPDX-License-Identifier: Apache-2.0
"""Server client: estimates with paging, submissions with bearer and UUIDv7 keys, statuses."""

from __future__ import annotations

import json
import secrets
import socket
import uuid
from collections.abc import Iterator
from datetime import UTC, datetime
from typing import Any

import pytest
from fakes import ESTIMATES_PATH, RECONCILIATIONS_PATH, FakeHTTP, FakeServer, Reply

from cost_reconciler.secretref import Secret
from cost_reconciler.server import Entry, Estimate, ServerClient, ServerError, SubmitResult
from cost_reconciler.transport import build_opener
from cost_reconciler.uuid7 import Uuid7Generator

START = datetime(2026, 10, 20, tzinfo=UTC)
END = datetime(2026, 10, 21, tzinfo=UTC)
A1 = "0192f4a0-7b1c-7250-807f-c268a7b21791"
A2 = "0192f4a0-7b1c-7250-807f-c268a7b21792"
PROBLEM = "application/problem+json"


@pytest.fixture
def server() -> Iterator[FakeServer]:
    fake = FakeServer()
    yield fake
    fake.close()


def client(url: str) -> ServerClient:
    return ServerClient(
        base_url=url, timeout_s=5.0, opener=build_opener(), new_key=Uuid7Generator()
    )


def token() -> Secret:
    return Secret("canary" + secrets.token_hex(12))


def item(n: int, agent: str = A1, **override: Any) -> dict[str, Any]:
    value: dict[str, Any] = {
        "run_id": f"00000000-0000-7000-8000-{n:012d}",
        "agent_id": agent,
        "provider": "anthropic",
        "currency": "USD",
        "estimate_micros": 1000 + n,
        "reconciled": False,
    }
    value.update(override)
    return value


def test_estimates_follow_the_cursor(server: FakeServer) -> None:
    bearer = token()
    server.estimates["2026-10-20T00:00:00.000Z"] = [
        item(n, A1 if n % 2 else A2) for n in range(150)
    ]
    got = client(server.url).list_estimates(bearer, START, END, [A2, A1])
    assert len(got) == 150
    assert got[0] == Estimate(
        run_id="00000000-0000-7000-8000-000000000000",
        agent_id=A2,
        provider="anthropic",
        currency="USD",
        estimate_micros=1000,
        reconciled=False,
    )
    first, second = server.requests
    for request in (first, second):
        assert request.method == "GET"
        assert request.path == ESTIMATES_PATH
        assert request.headers["authorization"] == f"Bearer {bearer.reveal()}"
        assert request.query["from"] == ["2026-10-20T00:00:00.000Z"]
        assert request.query["to"] == ["2026-10-21T00:00:00.000Z"]
        assert request.query["agent_id"] == [A1, A2]
        assert request.query["limit"] == ["100"]
        assert "idempotency-key" not in request.headers
    assert "cursor" not in first.query
    assert second.query["cursor"] == ["100"]


def test_estimates_without_items(server: FakeServer) -> None:
    assert client(server.url).list_estimates(token(), START, END, [A1]) == []


@pytest.mark.parametrize(
    "page",
    [
        [],
        {"items": {}},
        {},
        {"items": [item(1)], "next_cursor": 5},
        {"items": [item(1)], "next_cursor": ""},
        {"items": ["x"]},
        {"items": [{k: v for k, v in item(1).items() if k != "estimate_micros"}]},
        {"items": [item(1, estimate_micros="5")]},
        {"items": [item(1, estimate_micros=True)]},
        {"items": [item(1, estimate_micros=-1)]},
        {"items": [item(1, estimate_micros=2**63)]},
        {"items": [item(1, reconciled="no")]},
        {"items": [item(1, run_id=7)]},
        {"items": [item(1, provider=None)]},
        {"items": [item(1), item(1)]},
        {"items": [item(1, agent_id="0192f4a0-7b1c-7250-807f-c268a7b21799")]},
    ],
)
def test_malformed_estimates_fail(server: FakeServer, page: Any) -> None:
    server.estimates_reply = Reply.of_json(200, page)
    with pytest.raises(ServerError) as info:
        client(server.url).list_estimates(token(), START, END, [A1])
    assert info.value.kind == "malformed"
    assert info.value.stage == "estimates"


def test_estimates_error_status(server: FakeServer) -> None:
    server.estimates_reply = Reply.of_json(
        403, {"code": "forbidden", "reason": "scope_missing"}, PROBLEM
    )
    with pytest.raises(ServerError) as info:
        client(server.url).list_estimates(token(), START, END, [A1])
    assert (info.value.status, info.value.code, info.value.reason) == (
        403,
        "forbidden",
        "scope_missing",
    )
    assert info.value.stage == "estimates"


def test_submission_shape(server: FakeServer) -> None:
    bearer = token()
    api = client(server.url)
    entries = [Entry("00000000-0000-7000-8000-000000000001", 5, "USD")]
    result = api.submit(bearer, "anthropic", START, END, entries)
    assert result == SubmitResult(reconciled=1, unmatched_run_ids=())
    api.submit(bearer, "anthropic", START, END, entries)
    first, second = server.submissions()
    assert first.method == "POST"
    assert first.headers["authorization"] == f"Bearer {bearer.reveal()}"
    assert first.headers["content-type"] == "application/json"
    assert first.json() == {
        "provider": "anthropic",
        "period_start": "2026-10-20T00:00:00.000Z",
        "period_end": "2026-10-21T00:00:00.000Z",
        "entries": [
            {
                "run_id": "00000000-0000-7000-8000-000000000001",
                "amount_micros": 5,
                "currency": "USD",
            }
        ],
    }
    keys = [uuid.UUID(r.headers["idempotency-key"]) for r in (first, second)]
    for key in keys:
        assert key.version == 7
        assert key.variant == uuid.RFC_4122
    assert keys[0] != keys[1]
    assert keys[0] < keys[1]


def test_unmatched_run_ids_are_returned(server: FakeServer) -> None:
    run = "00000000-0000-7000-8000-000000000009"
    server.unmatched = {run}
    result = client(server.url).submit(token(), "anthropic", START, END, [Entry(run, 1, "USD")])
    assert result == SubmitResult(reconciled=0, unmatched_run_ids=(run,))


def test_validation_errors_carry_pointers_and_codes(server: FakeServer) -> None:
    server.submit_reply = Reply.of_json(
        400,
        {
            "type": "/api/v1/problems/validation_failed",
            "code": "validation_failed",
            "detail": "free text that is never logged",
            "errors": [
                {"pointer": "/entries/0/amount_micros", "code": "too_small", "detail": "x"},
                {"pointer": "/period_end", "code": "invalid"},
                {"pointer": 5, "code": "invalid"},
                "junk",
            ],
        },
        PROBLEM,
    )
    with pytest.raises(ServerError) as info:
        client(server.url).submit(token(), "anthropic", START, END, [])
    err = info.value
    assert err.status == 400
    assert err.code == "validation_failed"
    assert err.errors == (("/entries/0/amount_micros", "too_small"), ("/period_end", "invalid"))
    assert err.stage == "submit"
    assert "free text" not in str(err)


@pytest.mark.parametrize(
    ("status", "body"),
    [
        (401, {"code": "unauthenticated"}),
        (401, {"code": "unauthenticated", "reason": "app_not_installed"}),
        (403, {"code": "forbidden", "reason": "scope_missing"}),
        (409, {"code": "request_in_progress"}),
        (422, {"code": "idempotency_key_reuse"}),
        (429, {"code": "rate_limited"}),
        (500, {"code": "internal"}),
        (503, {"code": "unavailable"}),
    ],
)
def test_error_statuses(server: FakeServer, status: int, body: dict[str, str]) -> None:
    server.submit_reply = Reply.of_json(status, body, PROBLEM)
    with pytest.raises(ServerError) as info:
        client(server.url).submit(token(), "anthropic", START, END, [])
    assert info.value.status == status
    assert info.value.code == body["code"]
    assert info.value.reason == body.get("reason")
    assert info.value.kind == "status"
    assert len(server.submissions()) == 1


def test_error_without_problem_body(server: FakeServer) -> None:
    server.submit_reply = Reply(502, b"<html>gateway</html>", {"Content-Type": "text/html"})
    with pytest.raises(ServerError) as info:
        client(server.url).submit(token(), "anthropic", START, END, [])
    assert (info.value.status, info.value.code, info.value.reason) == (502, None, None)


def test_problem_members_must_be_strings(server: FakeServer) -> None:
    server.submit_reply = Reply.of_json(403, {"code": 5, "reason": ["x"]}, PROBLEM)
    with pytest.raises(ServerError) as info:
        client(server.url).submit(token(), "anthropic", START, END, [])
    assert (info.value.code, info.value.reason) == (None, None)


@pytest.mark.parametrize(
    "body",
    [
        b"[]",
        b"not json",
        json.dumps({"reconciled": "1", "unmatched_run_ids": []}).encode(),
        json.dumps({"reconciled": 1}).encode(),
        json.dumps({"reconciled": 1, "unmatched_run_ids": [1]}).encode(),
        json.dumps({"reconciled": -1, "unmatched_run_ids": []}).encode(),
    ],
)
def test_malformed_submit_answer(server: FakeServer, body: bytes) -> None:
    server.submit_reply = Reply(200, body, {"Content-Type": "application/json"})
    with pytest.raises(ServerError) as info:
        client(server.url).submit(token(), "anthropic", START, END, [])
    assert info.value.kind == "malformed"


def test_redirect_is_not_followed(server: FakeServer) -> None:
    target = FakeHTTP(lambda _: Reply.of_json(200, {"reconciled": 0, "unmatched_run_ids": []}))
    try:
        for status in (301, 302, 307, 308):
            server.submit_reply = Reply(
                status, b"", {"Location": target.url + RECONCILIATIONS_PATH}
            )
            with pytest.raises(ServerError) as info:
                client(server.url).submit(token(), "anthropic", START, END, [])
            assert info.value.status == status
            server.estimates_reply = Reply(status, b"", {"Location": target.url + ESTIMATES_PATH})
            with pytest.raises(ServerError):
                client(server.url).list_estimates(token(), START, END, [A1])
        assert target.requests == []
    finally:
        target.close()


def test_network_error() -> None:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    with pytest.raises(ServerError) as info:
        client(f"http://127.0.0.1:{port}").submit(token(), "anthropic", START, END, [])
    assert info.value.kind == "network"
    assert info.value.status is None


def test_uuid7_layout_and_order() -> None:
    now = 1_760_000_000_000
    generator = Uuid7Generator(now_ms=lambda: now, random_bytes=lambda n: b"\xff" * n)
    first = generator()
    assert first.version == 7
    assert first.variant == uuid.RFC_4122
    assert first.int >> 80 == now
    keys = [generator() for _ in range(5000)]
    assert keys == sorted(keys)
    assert len(set(keys)) == len(keys)
    assert all(k.version == 7 and k.variant == uuid.RFC_4122 for k in keys)


def test_uuid7_clock_going_back_keeps_order() -> None:
    times = iter([2000, 1000, 1000, 3000])
    generator = Uuid7Generator(now_ms=lambda: next(times))
    keys = [generator() for _ in range(4)]
    assert keys == sorted(keys)
    assert keys[3].int >> 80 == 3000


def test_uuid7_default_generator() -> None:
    from cost_reconciler.uuid7 import new_uuid7

    a, b = new_uuid7(), new_uuid7()
    assert a.version == 7
    assert a < b


def test_deeply_nested_answers_are_malformed(server: FakeServer) -> None:
    nested = Reply(200, b"[" * 200_000, {"Content-Type": "application/json"})
    server.estimates_reply = nested
    with pytest.raises(ServerError) as info:
        client(server.url).list_estimates(token(), START, END, [A1])
    assert info.value.kind == "malformed"
    server.submit_reply = nested
    with pytest.raises(ServerError) as info:
        client(server.url).submit(token(), "anthropic", START, END, [])
    assert info.value.kind == "malformed"
