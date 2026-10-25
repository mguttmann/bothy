# SPDX-License-Identifier: Apache-2.0
"""The Fake-Jira API itself (brief M2-50, Design note 18; Tests row ``test_fake_jira.py``).

Proves that the fake's deliveries verify with the app's check
(`jira_app.jira.signature.verify_signature`) and that each delivery option breaks the signature
in the intended way; that each REST fault kind behaves as described (``500``, ``400``, a
``302`` to a trap that never receives a request when the client does not follow redirects,
``hang`` released by `FakeJira.stop`, ``drop``); that the checks of the REST side answer as the
Design note says; that requests are recorded; that `FakeJira.auto_echo` and
`FakeJira.echo_before_answer` deliver the echoes; and that `render_fixture` applies its
overrides.
"""

from __future__ import annotations

import asyncio
import base64
import hashlib
import hmac
import json
from collections.abc import AsyncIterator
from dataclasses import dataclass, field
from typing import Any

import httpx
import pytest
from aiohttp import web

from jira_app.jira.signature import HEADER, sign_body, verify_signature
from tests.fakes.jira_api import FakeJira, load_fixture, render_fixture
from tests.helpers import CREDENTIALS, PATIENCE, SECRETS, eventually

pytestmark = pytest.mark.anyio

SECRET = SECRETS["webhook_secret"]
ISSUE_PATH = "/rest/api/3/issue"


@dataclass
class Delivery:
    """One request the receiver got."""

    body: bytes
    signatures: list[str]
    content_type: str


@dataclass
class Receiver:
    """A tiny aiohttp receiver that records raw bodies and ``X-Hub-Signature`` values."""

    url: str = ""
    status: int = 202
    deliveries: list[Delivery] = field(default_factory=list)
    entered: int = 0
    gate: asyncio.Event | None = None

    async def handle(self, request: web.Request) -> web.StreamResponse:
        self.entered += 1
        if self.gate is not None:
            await self.gate.wait()
        body = await request.read()
        self.deliveries.append(
            Delivery(
                body,
                list(request.headers.getall(HEADER, [])),
                request.headers.get("Content-Type", ""),
            )
        )
        return web.Response(status=self.status)

    def events(self) -> list[dict[str, Any]]:
        loaded: list[dict[str, Any]] = [json.loads(d.body) for d in self.deliveries]
        return loaded


@pytest.fixture
async def receiver() -> AsyncIterator[Receiver]:
    rec = Receiver()
    app = web.Application()
    app.router.add_route("*", "/{tail:.*}", rec.handle)
    runner = web.AppRunner(app, access_log=None, handle_signals=False)
    await runner.setup()
    try:
        site = web.TCPSite(runner, "127.0.0.1", 0)
        await site.start()
        host, port = runner.addresses[0][:2]
        rec.url = f"http://{host}:{port}/webhook"
        yield rec
    finally:
        if rec.gate is not None:
            rec.gate.set()
        await runner.cleanup()


@pytest.fixture
async def fake() -> AsyncIterator[FakeJira]:
    jira = await FakeJira.start(credentials=CREDENTIALS, secret=SECRET)
    try:
        yield jira
    finally:
        await jira.stop()


@pytest.fixture
async def client() -> AsyncIterator[httpx.AsyncClient]:
    http = httpx.AsyncClient(timeout=PATIENCE, follow_redirects=False)
    try:
        yield http
    finally:
        await http.aclose()


def good_headers(**changes: str | None) -> dict[str, str]:
    """The headers of the app's Jira client; a ``None`` value removes the header."""
    headers: dict[str, str | None] = {
        "Authorization": "Basic " + base64.b64encode(CREDENTIALS.encode()).decode(),
        "Accept": "application/json",
        "Content-Type": "application/json",
        "User-Agent": "jira-app/0.0.0-test",
    }
    headers.update(changes)
    return {k: v for k, v in headers.items() if v is not None}


def adf(text: str) -> dict[str, Any]:
    return {
        "type": "doc",
        "version": 1,
        "content": [{"type": "paragraph", "content": [{"type": "text", "text": text}]}],
    }


def issue_body(
    project: str = "SUP",
    issue_type: str = "Task",
    summary: str = "Login fails",
    description: Any = None,
) -> dict[str, Any]:
    return {
        "fields": {
            "project": {"key": project},
            "issuetype": {"name": issue_type},
            "summary": summary,
            "description": adf("Created from chat.") if description is None else description,
        }
    }


async def create_issue(
    client: httpx.AsyncClient, fake: FakeJira, body: Any = None, **headers: str | None
) -> httpx.Response:
    return await client.post(
        fake.url + ISSUE_PATH,
        json=issue_body() if body is None else body,
        headers=good_headers(**headers),
    )


async def create_comment(
    client: httpx.AsyncClient, fake: FakeJira, issue_id: str, body: Any = None
) -> httpx.Response:
    return await client.post(
        f"{fake.url}{ISSUE_PATH}/{issue_id}/comment",
        json={"body": adf("Thanks") if body is None else body},
        headers=good_headers(),
    )


def verdict(delivery: Delivery, secret: str = SECRET) -> str | None:
    return verify_signature(secret.encode(), delivery.body, delivery.signatures)


# -- deliveries ----------------------------------------------------------------------------


async def test_delivery_verifies_with_the_app_check(fake: FakeJira, receiver: Receiver) -> None:
    payload = render_fixture("jira:issue_created")
    assert await fake.deliver(receiver.url, payload) == 202
    (delivery,) = receiver.deliveries
    assert delivery.body == json.dumps(payload).encode("utf-8")
    assert delivery.content_type == "application/json"
    assert len(delivery.signatures) == 1
    assert delivery.signatures[0] == sign_body(SECRET.encode(), delivery.body)
    assert verdict(delivery) is None
    assert verdict(delivery, secret="another secret") == "mismatch"


async def test_delivery_of_raw_bytes_and_the_receiver_status(
    fake: FakeJira, receiver: Receiver
) -> None:
    receiver.status = 204
    raw = b'{"webhookEvent": "jira:issue_deleted"}'
    assert await fake.deliver(receiver.url, raw) == 204
    (delivery,) = receiver.deliveries
    assert delivery.body == raw
    assert verdict(delivery) is None


async def test_delivery_with_another_secret_mismatches(fake: FakeJira, receiver: Receiver) -> None:
    await fake.deliver(receiver.url, render_fixture("comment_created"), secret="not the secret")
    (delivery,) = receiver.deliveries
    assert verdict(delivery) == "mismatch"
    assert verdict(delivery, secret="not the secret") is None


async def test_omitted_signature_is_missing(fake: FakeJira, receiver: Receiver) -> None:
    await fake.deliver(receiver.url, render_fixture("jira:issue_created"), omit=True)
    (delivery,) = receiver.deliveries
    assert delivery.signatures == []
    assert verdict(delivery) == "missing"


async def test_altered_signature_mismatches(fake: FakeJira, receiver: Receiver) -> None:
    payload = render_fixture("jira:issue_created")
    await fake.deliver(receiver.url, payload, alter=True)
    (delivery,) = receiver.deliveries
    (value,) = delivery.signatures
    good = sign_body(SECRET.encode(), delivery.body)
    assert len(value) == len(good)
    assert value[:-1] == good[:-1]
    assert value[-1] != good[-1]
    assert verdict(delivery) == "mismatch"


async def test_truncated_signature_is_malformed(fake: FakeJira, receiver: Receiver) -> None:
    await fake.deliver(receiver.url, render_fixture("jira:issue_created"), truncate=True)
    (delivery,) = receiver.deliveries
    (value,) = delivery.signatures
    good = sign_body(SECRET.encode(), delivery.body)
    assert value == good[:-8]
    assert verdict(delivery) == "malformed"


@pytest.mark.parametrize("method", ["sha1", "sha512"])
async def test_other_method_is_unsupported(fake: FakeJira, receiver: Receiver, method: str) -> None:
    await fake.deliver(receiver.url, render_fixture("jira:issue_created"), method=method)
    (delivery,) = receiver.deliveries
    (value,) = delivery.signatures
    digest = hmac.new(SECRET.encode(), delivery.body, getattr(hashlib, method)).hexdigest()
    assert value == f"{method}={digest}"
    assert verdict(delivery) == "unsupported_method"


async def test_second_header_is_malformed(fake: FakeJira, receiver: Receiver) -> None:
    await fake.deliver(receiver.url, render_fixture("jira:issue_created"), second_header=True)
    (delivery,) = receiver.deliveries
    good = sign_body(SECRET.encode(), delivery.body)
    assert delivery.signatures == [good, good]
    assert verdict(delivery) == "malformed"


async def test_explicit_signature_is_sent_verbatim(fake: FakeJira, receiver: Receiver) -> None:
    raw = b'{"webhookEvent": "comment_created"}'
    await fake.deliver(receiver.url, raw, signature="sha256=")
    await fake.deliver(receiver.url, raw, signature=sign_body(SECRET.encode(), raw), alter=True)
    first, second = receiver.deliveries
    assert first.signatures == ["sha256="]
    assert verdict(first) == "malformed"
    # The explicit value replaces the computed one, also after ``alter``.
    assert second.signatures == [sign_body(SECRET.encode(), raw)]
    assert verdict(second) is None


async def test_content_type_option(fake: FakeJira, receiver: Receiver) -> None:
    await fake.deliver(
        receiver.url, render_fixture("jira:issue_created"), content_type="text/plain"
    )
    (delivery,) = receiver.deliveries
    assert delivery.content_type == "text/plain"
    assert verdict(delivery) is None


# -- REST faults ---------------------------------------------------------------------------


@pytest.mark.parametrize("status", ["500", "400"])
async def test_status_fault_applies_to_one_call(
    fake: FakeJira, client: httpx.AsyncClient, status: str
) -> None:
    fake.fail(status)
    first = await create_issue(client, fake)
    assert first.status_code == int(status)
    assert "errorMessages" in first.json()
    second = await create_issue(client, fake)
    assert second.status_code == 201
    assert second.json()["id"] == "10001"
    assert len(fake.issues()) == 2


async def test_redirect_fault_never_reaches_the_trap(
    fake: FakeJira, client: httpx.AsyncClient
) -> None:
    fake.fail("302")
    response = await create_issue(client, fake)
    assert response.status_code == 302
    assert response.headers["Location"] == fake.trap_url + ISSUE_PATH
    assert fake.trap_requests == []
    assert len(fake.requests) == 1


async def test_trap_records_a_followed_redirect(fake: FakeJira) -> None:
    # The trap is a live listener: a client that follows redirects would be seen there.
    fake.fail("302")
    async with httpx.AsyncClient(timeout=PATIENCE, follow_redirects=True) as following:
        response = await create_issue(following, fake)
    assert response.status_code == 599
    # httpx turns the POST into a GET on a 302, as browsers do.
    assert [r.path for r in fake.trap_requests] == [ISSUE_PATH]


async def test_hang_fault_is_released_by_stop(client: httpx.AsyncClient) -> None:
    jira = await FakeJira.start(credentials=CREDENTIALS, secret=SECRET)
    task = asyncio.ensure_future(create_issue(client, jira))
    try:
        jira.fail("hang")
        await eventually(lambda: len(jira.requests) == 1, "the held request")
        assert not task.done()
    finally:
        await jira.stop()
    response = await task
    assert response.status_code == 201
    assert jira.issues()[0].path == ISSUE_PATH


async def test_drop_fault_closes_the_connection(fake: FakeJira, client: httpx.AsyncClient) -> None:
    fake.fail("drop")
    with pytest.raises(httpx.TransportError):
        await create_issue(client, fake)
    assert len(fake.requests) == 1
    assert (await create_issue(client, fake)).status_code == 201


async def test_faults_are_used_in_order(fake: FakeJira, client: httpx.AsyncClient) -> None:
    fake.fail("500", "400")
    assert (await create_issue(client, fake)).status_code == 500
    assert (await create_issue(client, fake)).status_code == 400
    assert (await create_issue(client, fake)).status_code == 201
    assert fake.faults == []


# -- REST checks ---------------------------------------------------------------------------


async def test_credentials_are_required(fake: FakeJira, client: httpx.AsyncClient) -> None:
    assert (await create_issue(client, fake, Authorization=None)).status_code == 401
    wrong = "Basic " + base64.b64encode(b"alex@example.com:wrong").decode()
    assert (await create_issue(client, fake, Authorization=wrong)).status_code == 401
    assert fake.known_issues.keys() == {"10012", "10013"}


@pytest.mark.parametrize(
    "changes",
    [
        {"Accept": "text/html"},
        {"Accept": None},
        {"User-Agent": "curl/8.0"},
        {"Content-Type": "text/plain"},
    ],
)
async def test_wrong_headers_give_400(
    fake: FakeJira, client: httpx.AsyncClient, changes: dict[str, str | None]
) -> None:
    response = await client.post(
        fake.url + ISSUE_PATH,
        content=json.dumps(issue_body()).encode(),
        headers=good_headers(**changes),
    )
    assert response.status_code == 400


@pytest.mark.parametrize(
    "body",
    [
        issue_body(project="OPS"),
        issue_body(issue_type="Bug"),
        issue_body(project="WEB", issue_type="Task"),
        issue_body(summary=""),
        issue_body(summary="   "),
        issue_body(description="Created from chat."),
        issue_body(description={"type": "doc", "version": 2, "content": []}),
        issue_body(description={"type": "paragraph", "version": 1, "content": []}),
        {"fields": "SUP"},
        {},
    ],
)
async def test_invalid_issue_gives_400(
    fake: FakeJira, client: httpx.AsyncClient, body: dict[str, Any]
) -> None:
    response = await create_issue(client, fake, body)
    assert response.status_code == 400
    assert fake.known_issues.keys() == {"10012", "10013"}


async def test_comment_on_unknown_issue_gives_404(
    fake: FakeJira, client: httpx.AsyncClient
) -> None:
    assert (await create_comment(client, fake, "99999")).status_code == 404


async def test_comment_needs_an_adf_body(fake: FakeJira, client: httpx.AsyncClient) -> None:
    assert (await create_comment(client, fake, "10012", "plain text")).status_code == 400


async def test_created_ids_and_keys(fake: FakeJira, client: httpx.AsyncClient) -> None:
    first = await create_issue(client, fake)
    second = await create_issue(client, fake)
    web_issue = await create_issue(client, fake, issue_body(project="WEB", issue_type="Bug"))
    assert [r.status_code for r in (first, second, web_issue)] == [201, 201, 201]
    assert [(r.json()["id"], r.json()["key"]) for r in (first, second, web_issue)] == [
        ("10001", "SUP-1"),
        ("10002", "SUP-2"),
        ("10003", "WEB-1"),
    ]
    assert first.json()["self"] == f"{fake.url}{ISSUE_PATH}/10001"
    comment = await create_comment(client, fake, "10001")
    assert comment.status_code == 201
    assert comment.json() == {"id": "10004"}
    fixture_comment = await create_comment(client, fake, "10012")
    assert fixture_comment.json() == {"id": "10005"}
    assert fake.known_issues["10003"] == "WEB-1"


# -- recording -----------------------------------------------------------------------------


async def test_requests_are_recorded(fake: FakeJira, client: httpx.AsyncClient) -> None:
    await create_issue(client, fake)
    await create_comment(client, fake, "10001")
    await create_issue(client, fake, Authorization=None)
    await client.get(fake.url + "/rest/api/3/myself", headers=good_headers())
    assert [(r.method, r.path) for r in fake.requests] == [
        ("POST", ISSUE_PATH),
        ("POST", f"{ISSUE_PATH}/10001/comment"),
        ("POST", ISSUE_PATH),
        ("GET", "/rest/api/3/myself"),
    ]
    issue = fake.requests[0]
    assert issue.body == issue_body()
    assert fake.requests[1].body == {"body": adf("Thanks")}
    assert fake.requests[3].body is None
    for name in ("authorization", "accept", "content-type", "user-agent"):
        assert name in issue.header_names
    assert issue.header_names == sorted(issue.header_names)
    assert "authorization" not in fake.requests[2].header_names
    assert fake.issues() == [fake.requests[0], fake.requests[2]]
    assert fake.comments() == [fake.requests[1]]


# -- auto_echo -----------------------------------------------------------------------------


async def test_issue_creation_echoes_issue_created(
    fake: FakeJira, client: httpx.AsyncClient, receiver: Receiver
) -> None:
    fake.auto_echo = receiver.url
    response = await create_issue(client, fake, issue_body(summary="Printer offline"))
    assert response.status_code == 201
    await fake.echoes_done()
    assert not fake.pending_echoes()
    assert fake.echo_statuses == [202]
    (delivery,) = receiver.deliveries
    assert verdict(delivery) is None
    (event,) = receiver.events()
    assert event["webhookEvent"] == "jira:issue_created"
    assert event["issue"]["id"] == response.json()["id"]
    assert event["issue"]["key"] == response.json()["key"] == "SUP-1"
    assert event["issue"]["fields"]["project"]["key"] == "SUP"
    assert event["issue"]["fields"]["summary"] == "Printer offline"
    assert event["user"]["displayName"] == "Sam"


async def test_comment_creation_echoes_comment_and_update(
    fake: FakeJira, client: httpx.AsyncClient, receiver: Receiver
) -> None:
    fake.auto_echo = receiver.url
    body = adf("It works again.")
    response = await create_comment(client, fake, "10012", body)
    assert response.status_code == 201
    await fake.echoes_done()
    assert fake.echo_statuses == [202, 202]
    assert all(verdict(d) is None for d in receiver.deliveries)
    events = {e["webhookEvent"]: e for e in receiver.events()}
    assert events.keys() == {"comment_created", "jira:issue_updated"}
    comment = events["comment_created"]
    assert comment["comment"]["id"] == response.json()["id"]
    assert comment["comment"]["body"] == body
    assert comment["comment"]["author"]["displayName"] == "Sam"
    assert comment["issue"]["id"] == "10012"
    assert comment["issue"]["key"] == "SUP-12"
    updated = events["jira:issue_updated"]
    assert "changelog" not in updated
    assert updated["issue"]["id"] == "10012"
    assert updated["issue"]["key"] == "SUP-12"


async def test_no_echo_without_auto_echo(
    fake: FakeJira, client: httpx.AsyncClient, receiver: Receiver
) -> None:
    await create_issue(client, fake)
    await create_comment(client, fake, "10001")
    assert not fake.pending_echoes()
    await fake.echoes_done()
    assert receiver.entered == 0
    assert fake.echo_statuses == []


async def test_echo_follows_the_answer_by_default(
    fake: FakeJira, client: httpx.AsyncClient, receiver: Receiver
) -> None:
    receiver.gate = asyncio.Event()
    fake.auto_echo = receiver.url
    # The receiver holds the echo, and still the creation is answered.
    response = await create_issue(client, fake)
    assert response.status_code == 201
    assert fake.pending_echoes()
    assert receiver.deliveries == []
    receiver.gate.set()
    await fake.echoes_done()
    assert len(receiver.deliveries) == 1
    assert fake.echo_statuses == [202]


async def test_echo_before_answer(
    fake: FakeJira, client: httpx.AsyncClient, receiver: Receiver
) -> None:
    receiver.gate = asyncio.Event()
    fake.auto_echo = receiver.url
    fake.echo_before_answer = True
    task = asyncio.ensure_future(create_issue(client, fake))
    try:
        await eventually(lambda: receiver.entered == 1, "the echo reaching the receiver")
        # The creation is not answered while its echo is held.
        assert not task.done()
        receiver.gate.set()
        response = await task
    finally:
        receiver.gate.set()
        if not task.done():
            task.cancel()
    assert response.status_code == 201
    assert len(receiver.deliveries) == 1
    assert fake.echo_statuses == [202]
    assert not fake.pending_echoes()
    assert receiver.events()[0]["issue"]["key"] == response.json()["key"]


# -- render_fixture ------------------------------------------------------------------------


def test_render_without_overrides_equals_the_fixture() -> None:
    for name in ("jira:issue_created", "jira:issue_updated", "comment_created"):
        assert render_fixture(name) == load_fixture(name)


def test_render_returns_a_copy() -> None:
    rendered = render_fixture("jira:issue_created")
    rendered["issue"]["fields"]["summary"] = "changed"
    assert render_fixture("jira:issue_created")["issue"]["fields"]["summary"] == "Login fails"


def test_render_issue_overrides() -> None:
    doc = adf("New text")
    data = render_fixture(
        "jira:issue_created",
        issue_id="10077",
        key="WEB-7",
        summary="Page is blank",
        text=doc,
        actor="Alex",
        timestamp=1760000000000,
    )
    issue = data["issue"]
    assert issue["id"] == "10077"
    assert issue["key"] == "WEB-7"
    assert issue["fields"]["project"]["key"] == "WEB"
    assert issue["fields"]["summary"] == "Page is blank"
    assert issue["fields"]["description"] == doc
    assert data["user"]["displayName"] == "Alex"
    assert data["timestamp"] == 1760000000000


def test_render_explicit_project_wins_over_the_key() -> None:
    data = render_fixture("jira:issue_created", key="SUP-40", project="WEB")
    assert data["issue"]["key"] == "SUP-40"
    assert data["issue"]["fields"]["project"]["key"] == "WEB"
    only_project = render_fixture("jira:issue_created", project="WEB")
    assert only_project["issue"]["key"] == "SUP-12"
    assert only_project["issue"]["fields"]["project"]["key"] == "WEB"


def test_render_text_none_is_kept_as_none() -> None:
    data = render_fixture("jira:issue_created", text=None)
    assert data["issue"]["fields"]["description"] is None


def test_render_comment_overrides() -> None:
    data = render_fixture("comment_created", comment_id="10099", actor="Alex", text="A new comment")
    assert data["comment"]["id"] == "10099"
    assert data["comment"]["author"]["displayName"] == "Alex"
    assert data["comment"]["body"] == "A new comment"
    # The issue description is untouched for a comment event.
    assert "description" not in data["issue"]["fields"]


def test_render_changelog_overrides() -> None:
    removed = render_fixture("jira:issue_updated", changelog=None)
    assert "changelog" not in removed
    items = [{"field": "status", "fromString": "To Do", "toString": "Done"}]
    replaced = render_fixture("jira:issue_updated", changelog=items)
    assert replaced["changelog"] == {"id": "10101", "items": items}
    kept = render_fixture("jira:issue_updated")
    assert [i["field"] for i in kept["changelog"]["items"]] == ["status", "summary", "description"]
