# SPDX-License-Identifier: Apache-2.0
"""The SDK adapter of the chat port against an ``httpx.MockTransport`` (brief M2-50 Design
notes 9 and 10): request bodies without JSON ``null``, the stored ``client_msg_id`` and
``Idempotency-Key`` on a rerun, ``Authorization`` sent, and a ``ProblemError`` with status and
code.
"""

from __future__ import annotations

import json
from collections.abc import Callable
from typing import Any

import httpx
import pytest
from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.client import Client as CoreClient
from bothy_sdk.generated.models.interaction_respond import InteractionRespond
from bothy_sdk.generated.models.message_create import MessageCreate
from bothy_sdk.ids import uuid7
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing.webhooks import webhook_public_key

from jira_app.chat.sdk_adapter import SdkChatPort
from tests.fakes.chat import APP_ID, SEED, message_json, principal
from tests.helpers import SECRETS, SUP_FORUM

pytestmark = pytest.mark.anyio

BASE = "https://chat.example.com/api/v1"
NOW = "2026-10-13T09:00:00Z"


def thread_json(thread_id: str) -> dict[str, Any]:
    return {
        "id": thread_id,
        "channel_id": SUP_FORUM,
        "kind": "thread",
        "title": "SUP-12 Login fails",
        "title_source": "manual",
        "state": "open",
        "last_message_seq": 1,
        "last_activity_at": NOW,
        "message_count": 1,
        "created_by": APP_ID,
        "created_at": NOW,
        "version": 1,
        "tag_ids": [],
    }


def answer(request: httpx.Request) -> httpx.Response:
    path = request.url.path
    if path.endswith("/me"):
        return httpx.Response(
            200,
            json={
                "principal": principal(APP_ID, "app", "Jira"),
                "is_owner": False,
                "permission_version": 1,
            },
        )
    if path.endswith("/instance/webhook-keys"):
        key = {"kid": "whk-1", "public_key": webhook_public_key(SEED), "state": "active"}
        return httpx.Response(200, json={"keys": [key]}, headers={"Cache-Control": "max-age=60"})
    if path.endswith("/threads") and request.method == "POST":
        return httpx.Response(201, json={"thread": thread_json(str(uuid7()))})
    if path.endswith("/messages") and request.method == "POST":
        body = json.loads(request.content)
        return httpx.Response(
            201,
            json=message_json(
                thread_id=path.split("/")[-2],
                channel_id=SUP_FORUM,
                content=body.get("content_md", ""),
                author=principal(APP_ID, "app", "Jira"),
                origin="app",
                client_msg_id=body["client_msg_id"],
            ),
        )
    if path.endswith("/respond"):
        return httpx.Response(409, json={"code": "conflict", "title": "conflict", "status": 409})
    return httpx.Response(404, json={"code": "not_found", "title": "not found", "status": 404})


def port(
    seen: list[httpx.Request], handler: Callable[[httpx.Request], httpx.Response] = answer
) -> SdkChatPort:
    def record(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return handler(request)

    transport = httpx.MockTransport(record)
    client = Client(
        BASE,
        Credentials(Secret(SECRETS["app_token"])),
        settings=SdkSettings(retry_max_attempts=1),
        transport=transport,
    )
    public = CoreClient(base_url=BASE).set_async_httpx_client(
        httpx.AsyncClient(base_url=BASE, transport=transport)
    )
    return SdkChatPort(client, public)


def no_null(value: Any) -> bool:
    if value is None:
        return False
    if isinstance(value, dict):
        return all(no_null(v) for v in value.values())
    if isinstance(value, list):
        return all(no_null(v) for v in value)
    return True


async def test_writes_carry_no_null_and_the_stored_identifiers() -> None:
    seen: list[httpx.Request] = []
    chat = port(seen)
    msg_id, key = uuid7(), str(uuid7())
    first = MessageCreate(client_msg_id=msg_id, content_md="Hello")
    for _ in range(2):
        await chat.create_forum_thread(SUP_FORUM, "SUP-12 Login fails", first, key)
    posts = [r for r in seen if r.url.path.endswith("/threads")]
    assert len(posts) == 2
    for request in posts:
        body = json.loads(request.content)
        assert no_null(body)
        assert body["first_message"]["client_msg_id"] == str(msg_id)
        assert request.headers["Idempotency-Key"] == key
        assert request.headers["Authorization"] == f"Bearer {SECRETS['app_token']}"
        assert (
            request.headers["User-Agent"].startswith("bothy-sdk/")
            or "bothy-sdk/" in (request.headers["User-Agent"])
        )
    thread_id = str(uuid7())
    posted = await chat.post_message(thread_id, MessageCreate(client_msg_id=msg_id, content_md="x"))
    assert str(posted.client_msg_id) == str(msg_id)
    message_post = seen[-1]
    assert no_null(json.loads(message_post.content))


async def test_respond_sends_its_key_and_a_conflict_reaches_the_job() -> None:
    seen: list[httpx.Request] = []
    chat = port(seen)
    key = str(uuid7())
    body = InteractionRespond.from_dict({"state": "acknowledged", "response": {"text": "ok"}})
    with pytest.raises(ProblemError) as caught:
        await chat.respond_interaction(str(uuid7()), body, key)
    assert caught.value.status == 409
    assert caught.value.code == "conflict"
    assert seen[-1].headers["Idempotency-Key"] == key
    assert no_null(json.loads(seen[-1].content))


async def test_problem_error_carries_status_and_code() -> None:
    seen: list[httpx.Request] = []

    def refuse(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            403,
            json={"code": "scope_missing", "title": "scope missing", "status": 403},
        )

    chat = port(seen, refuse)
    with pytest.raises(ProblemError) as caught:
        await chat.create_forum_thread(
            SUP_FORUM, "t", MessageCreate(client_msg_id=uuid7(), content_md="x"), str(uuid7())
        )
    assert (caught.value.status, caught.value.code) == (403, "scope_missing")


async def test_me_limits_and_keys() -> None:
    seen: list[httpx.Request] = []
    chat = port(seen)
    assert str((await chat.me()).id) == APP_ID
    keys = await chat.webhook_keys()
    assert len(keys) == 1
    key_requests = [r for r in seen if r.url.path.endswith("/webhook-keys")]
    # getWebhookKeys is public: the key list is fetched without the app's token.
    assert "Authorization" not in key_requests[0].headers
    await chat.webhook_keys()
    assert len([r for r in seen if r.url.path.endswith("/webhook-keys")]) == 1
    with pytest.raises(ProblemError):
        await chat.get_message(str(uuid7()))
    await chat.aclose()
