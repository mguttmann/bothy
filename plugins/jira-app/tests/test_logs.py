# SPDX-License-Identifier: Apache-2.0
"""No foreign text, key or secret in a log line (brief M2-50 Design note 16; SEC-LOG-2,
SEC-LOG-4, SEC-SEC-3).
"""

from __future__ import annotations

import io
import json
import logging
from pathlib import Path

import pytest

from jira_app.log import LOG_ATTRIBUTES, SDK_LOGGER, AppLog
from tests.fakes.chat import ALEX_ID, envelope, message_json, principal
from tests.fakes.jira_api import render_fixture
from tests.helpers import (
    SECRETS,
    SUP_FORUM,
    command_event,
    interaction_event,
    start_world,
)

pytestmark = pytest.mark.anyio

CANARIES = {
    "summary": "canary-summary-7f1",
    "description": "canary-description-7f2",
    "comment": "canary-comment-7f3",
    "title": "canary-title-7f4",
    "form": "canary-form-value-7f5",
    "actor": "canary-display-name-7f6",
    "chat_name": "canary-chat-name-7f7",
    "chat_text": "canary-chat-text-7f8",
    "from": "canary-from-7f9",
    "to": "canary-to-7fa",
    "account": "example-account-sam",
}


async def test_no_canary_reaches_a_log_line(tmp_path: Path) -> None:
    world = await start_world(tmp_path)
    try:
        created = render_fixture(
            "jira:issue_created",
            summary=CANARIES["summary"],
            text=CANARIES["description"],
            actor=CANARIES["actor"],
        )
        assert await world.deliver(created) == 202
        assert await world.deliver(created, alter=True) == 401
        assert await world.deliver(created, method="sha1") == 401
        comment = render_fixture(
            "comment_created", text=CANARIES["comment"], actor=CANARIES["actor"]
        )
        assert await world.deliver(comment) == 202
        updated = render_fixture(
            "jira:issue_updated",
            changelog=[
                {"field": "status", "fromString": CANARIES["from"], "toString": CANARIES["to"]}
            ],
        )
        assert await world.deliver(updated) == 202
        await world.settle()
        response = await world.server_request(
            "/chat/interactions", command_event(CANARIES["title"])
        )
        assert response.status_code == 202
        await world.settle()
        post = world.chat.thread_titled(CANARIES["title"])
        assert post is not None
        card = world.chat.card_of(post)
        assert card is not None
        submitted = interaction_event(
            custom_id=card["components"][0]["custom_id"],
            message_id=world.chat.message_id_of_card(post),
            values={"description": CANARIES["form"]},
        )
        assert (await world.server_request("/chat/interactions", submitted)).status_code == 202
        await world.settle()
        issue_thread = world.chat.thread_titled("SUP-12")
        assert issue_thread is not None
        reply = envelope(
            "message.created",
            {
                "message": message_json(
                    thread_id=issue_thread.thread_id,
                    channel_id=SUP_FORUM,
                    content=CANARIES["chat_text"],
                    author=principal(ALEX_ID, "user", CANARIES["chat_name"]),
                )
            },
            channel_id=SUP_FORUM,
        )
        assert (await world.server_request("/chat/events", reply)).status_code == 202
        await world.settle()
        assert len(world.jira.issues()) == 1
        assert len(world.jira.comments()) == 1
    finally:
        await world.close()
    text = "\n".join(world.handler.lines)
    assert len(world.handler.lines) > 20
    for name, value in {**CANARIES, **SECRETS}.items():
        assert value not in text, name
    assert "SUP" not in text
    assert "jira.example.com" not in text
    assert "http://127.0.0.1" not in text
    for line in world.logs:
        assert set(line) <= {"time", "level", "msg", *LOG_ATTRIBUTES}


def test_formatter_drops_unknown_attributes_and_sdk_text() -> None:
    out = io.StringIO()
    log = AppLog.to_stream(out)
    log.attach_sdk()
    try:
        log.warn("jira.delivery.refused", status=401, summary=CANARIES["summary"], url="x")
        # The text of an SDK record never reaches the line, whatever it holds.
        logging.getLogger(SDK_LOGGER).warning("call %s with %s", "getMe", CANARIES["chat_text"])
    finally:
        log.close()
    lines = [json.loads(line) for line in out.getvalue().splitlines()]
    assert lines[0] == {
        "time": lines[0]["time"],
        "level": "warn",
        "msg": "jira.delivery.refused",
        "status": 401,
    }
    assert lines[1]["msg"] == "sdk.client"
    assert set(lines[1]) == {"time", "level", "msg"}
    assert CANARIES["chat_text"] not in out.getvalue()
