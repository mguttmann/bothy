# SPDX-License-Identifier: Apache-2.0
"""The Jira REST client against an ``httpx.MockTransport`` (brief M2-50 Design note 15).

Proves the headers of Design note 15 (``Accept``, ``Content-Type``, ``Authorization: Basic``
with the standard base64 of the credentials line, ``User-Agent: jira-app/<VERSION>``), the
request bodies of the two calls, that no redirect is followed and a ``302`` is refused, the
time limit ``sdk.http_timeout`` over the whole call, that no response body is read on errors,
that the credentials file is checked at each use, and that an unreadable ``201`` or a transport
failure leaves the outcome unknown.
"""

from __future__ import annotations

import asyncio
import base64
import json
from collections.abc import AsyncIterator, Awaitable, Callable
from pathlib import Path
from typing import Any

import httpx
import pytest

from jira_app.config import read_jira_credentials
from jira_app.jira.adf import issue_description, mirror_comment
from jira_app.jira.client import JiraClient, JiraResult
from tests.helpers import CREDENTIALS, write_secret

pytestmark = pytest.mark.anyio

BASE = "https://jira.example.com"
VERSION = (Path(__file__).resolve().parents[1] / "VERSION").read_text(encoding="utf-8").strip()
TIMEOUT = 5.0
"""A generous ``sdk.http_timeout`` for the tests that must not time out."""

Handler = Callable[[httpx.Request], Awaitable[httpx.Response]]


class Recorder:
    """Records each request the transport sees and answers with ``handler``."""

    def __init__(self, handler: Handler) -> None:
        self.handler = handler
        self.requests: list[httpx.Request] = []

    async def __call__(self, request: httpx.Request) -> httpx.Response:
        await request.aread()
        self.requests.append(request)
        return await self.handler(request)


class WatchedStream(httpx.AsyncByteStream):
    """A response body that records whether anyone iterated it."""

    def __init__(self, body: bytes) -> None:
        self.body = body
        self.iterated = False
        self.closed = False

    async def __aiter__(self) -> AsyncIterator[bytes]:
        self.iterated = True
        yield self.body

    async def aclose(self) -> None:
        self.closed = True


def answer(status: int, data: Any = None, **kwargs: Any) -> Handler:
    async def handler(_request: httpx.Request) -> httpx.Response:
        if data is None:
            return httpx.Response(status, **kwargs)
        return httpx.Response(status, json=data, **kwargs)

    return handler


@pytest.fixture
def credentials_file(tmp_path: Path) -> Path:
    return write_secret(tmp_path / "jira-credentials", CREDENTIALS + "\n")


ClientFactory = Callable[..., JiraClient]


@pytest.fixture
async def make_client(credentials_file: Path) -> AsyncIterator[ClientFactory]:
    clients: list[JiraClient] = []

    def factory(recorder: Recorder, timeout: float = TIMEOUT) -> JiraClient:
        client = JiraClient(
            BASE + "/",
            lambda: read_jira_credentials(str(credentials_file)),
            timeout,
            transport=httpx.MockTransport(recorder),
        )
        clients.append(client)
        return client

    try:
        yield factory
    finally:
        for client in clients:
            await client.aclose()


async def create(client: JiraClient) -> JiraResult:
    return await client.create_issue(
        "SUP", "Task", "Login fails", issue_description("Users cannot log in.")
    )


async def test_headers_of_design_note_15(make_client: ClientFactory) -> None:
    recorder = Recorder(answer(201, {"id": "10001", "key": "SUP-1"}))
    client = make_client(recorder)
    await create(client)
    await client.create_comment("10001", mirror_comment("Alex", "hello"))
    assert len(recorder.requests) == 2
    expected = "Basic " + base64.standard_b64encode(CREDENTIALS.encode("utf-8")).decode("ascii")
    for request in recorder.requests:
        assert request.method == "POST"
        assert request.headers["Accept"] == "application/json"
        assert request.headers["Content-Type"] == "application/json"
        assert request.headers["Authorization"] == expected
        assert request.headers["User-Agent"] == f"jira-app/{VERSION}"
        assert len(request.headers.get_list("Authorization")) == 1


async def test_create_issue_request_and_result(make_client: ClientFactory) -> None:
    recorder = Recorder(answer(201, {"id": "10001", "key": "SUP-1", "self": BASE}))
    description = issue_description("Users cannot log in.\nSince the update.")
    result = await make_client(recorder).create_issue("SUP", "Task", "Login fails", description)
    assert result == JiraResult("created", status=201, object_id="10001", key="SUP-1")
    (request,) = recorder.requests
    assert str(request.url) == f"{BASE}/rest/api/3/issue"
    assert json.loads(request.content) == {
        "fields": {
            "project": {"key": "SUP"},
            "issuetype": {"name": "Task"},
            "summary": "Login fails",
            "description": description,
        }
    }


async def test_create_comment_request_and_result(make_client: ClientFactory) -> None:
    recorder = Recorder(answer(201, {"id": 10020}))
    body = mirror_comment("Robin", "a reply with `code`")
    result = await make_client(recorder).create_comment("10001", body)
    assert result == JiraResult("created", status=201, object_id="10020")
    (request,) = recorder.requests
    assert str(request.url) == f"{BASE}/rest/api/3/issue/10001/comment"
    assert json.loads(request.content) == {"body": body}


async def test_redirect_is_refused_and_not_followed(make_client: ClientFactory) -> None:
    recorder = Recorder(answer(302, headers={"Location": "http://127.0.0.1:9/elsewhere"}))
    result = await create(make_client(recorder))
    assert result.kind == "refused"
    assert result.status == 302
    assert len(recorder.requests) == 1


async def test_time_limit_gives_unknown_timeout(make_client: ClientFactory) -> None:
    never = asyncio.Event()
    cancelled = asyncio.Event()

    async def hang(_request: httpx.Request) -> httpx.Response:
        try:
            await never.wait()
        except asyncio.CancelledError:
            cancelled.set()
            raise
        return httpx.Response(201)  # pragma: no cover - the event is never set

    recorder = Recorder(hang)
    result = await create(make_client(recorder, timeout=0.05))
    assert result == JiraResult("unknown", reason="timeout")
    assert cancelled.is_set()
    assert len(recorder.requests) == 1


@pytest.mark.parametrize("status", [400, 401, 404, 500, 503])
async def test_no_body_read_on_errors(make_client: ClientFactory, status: int) -> None:
    stream = WatchedStream(b'{"errorMessages": ["secret detail"], "id": "10001"}')
    recorder = Recorder(answer(status, stream=stream))
    result = await create(make_client(recorder))
    assert result.kind == ("unknown" if status >= 500 else "refused")
    assert result.status == status
    assert (result.object_id, result.key, result.reason) == ("", "", "")
    assert not stream.iterated
    assert stream.closed


async def test_body_read_on_created(make_client: ClientFactory) -> None:
    stream = WatchedStream(b'{"id": "10001", "key": "SUP-1"}')
    result = await create(make_client(Recorder(answer(201, stream=stream))))
    assert result.kind == "created"
    assert stream.iterated


async def test_credentials_file_checked_at_each_use(
    make_client: ClientFactory, credentials_file: Path
) -> None:
    recorder = Recorder(answer(201, {"id": "10001", "key": "SUP-1"}))
    client = make_client(recorder)
    assert (await create(client)).kind == "created"
    credentials_file.chmod(0o644)
    assert await create(client) == JiraResult("not_sent", reason="credentials_file")
    assert await client.create_comment("10001", mirror_comment("Sam", "x")) == JiraResult(
        "not_sent", reason="credentials_file"
    )
    assert len(recorder.requests) == 1


async def test_credentials_line_read_again_at_each_use(
    make_client: ClientFactory, credentials_file: Path
) -> None:
    recorder = Recorder(answer(201, {"id": "10001", "key": "SUP-1"}))
    client = make_client(recorder)
    await create(client)
    write_secret(credentials_file, "robin@example.com:other-token")
    await create(client)
    tokens = [r.headers["Authorization"].removeprefix("Basic ") for r in recorder.requests]
    assert [base64.b64decode(t).decode("utf-8") for t in tokens] == [
        CREDENTIALS,
        "robin@example.com:other-token",
    ]


@pytest.mark.parametrize(
    "data",
    [
        {"id": "10001", "key": "sup-1"},
        {"id": "10001", "key": "SUP-12)"},
        {"id": "10001", "key": "SUP-0"},
        {"id": "10001"},
        {"key": "SUP-1"},
        {"id": "", "key": "SUP-1"},
        {"id": "10a01", "key": "SUP-1"},
        {"id": True, "key": "SUP-1"},
        {"id": None, "key": "SUP-1"},
        ["10001", "SUP-1"],
    ],
)
async def test_unreadable_created_issue_is_unknown(make_client: ClientFactory, data: Any) -> None:
    result = await create(make_client(Recorder(answer(201, data))))
    assert result == JiraResult("unknown", status=201, reason="unreadable")


async def test_unreadable_created_comment_is_unknown(make_client: ClientFactory) -> None:
    for handler in (answer(201, {"self": BASE}), answer(201, content=b"not json")):
        client = make_client(Recorder(handler))
        result = await client.create_comment("10001", mirror_comment("Sam", "x"))
        assert result == JiraResult("unknown", status=201, reason="unreadable")


@pytest.mark.parametrize(
    "error",
    [
        httpx.ConnectError("refused"),
        httpx.ReadError("dropped"),
        httpx.RemoteProtocolError("closed without an answer"),
    ],
)
async def test_transport_errors_are_unknown(
    make_client: ClientFactory, error: httpx.HTTPError
) -> None:
    async def fail(_request: httpx.Request) -> httpx.Response:
        raise error

    recorder = Recorder(fail)
    assert await create(make_client(recorder)) == JiraResult("unknown", reason="transport")
    assert len(recorder.requests) == 1


async def test_transport_timeout_is_unknown_timeout(make_client: ClientFactory) -> None:
    async def fail(_request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("slow")

    assert await create(make_client(Recorder(fail))) == JiraResult("unknown", reason="timeout")
