# SPDX-License-Identifier: Apache-2.0
"""``/issue`` with a form against the Fake-Jira API (brief M2-50 Design notes 7 and 12; PLAN
row: "form against the fake API").
"""

from __future__ import annotations

import time
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.settings import SdkSettings

from jira_app.store.base import NewJob, SeenKey
from jira_app.store.sqlite_store import SqliteStore
from tests.fakes.chat import OTHER_SEED, envelope, problem
from tests.fakes.jira_api import render_fixture
from tests.helpers import (
    PATIENCE,
    SUP_FORUM,
    World,
    command_event,
    interaction_event,
    start_world,
)

pytestmark = pytest.mark.anyio


@pytest.fixture
async def world(tmp_path: Path) -> AsyncIterator[World]:
    w = await start_world(tmp_path)
    try:
        yield w
    finally:
        await w.close()


async def open_form(world: World, title: str = "Crash on start") -> tuple[Any, dict[str, Any]]:
    response = await world.server_request("/chat/interactions", command_event(title))
    assert response.status_code == 202
    await world.settle()
    thread = world.chat.thread_titled(title)
    assert thread is not None
    card = world.chat.card_of(thread)
    assert card is not None
    return thread, card


async def submit(
    world: World, thread: Any, card: dict[str, Any], values: dict[str, Any]
) -> dict[str, Any]:
    event = interaction_event(
        custom_id=card["components"][0]["custom_id"],
        message_id=world.chat.message_id_of_card(thread),
        values=values,
    )
    response = await world.server_request("/chat/interactions", event)
    assert response.status_code == 202
    await world.settle()
    return event


def last_paragraph(adf: dict[str, Any]) -> str:
    return str(adf["content"][-1]["content"][0]["text"])


async def test_form_creates_one_issue_and_answers_in_the_thread(world: World) -> None:
    thread, card = await open_form(world)
    assert thread.channel_id == SUP_FORUM
    assert thread.messages[0]["content_md"] == "New Jira issue. Fill in the form below."
    component = card["components"][0]
    assert component["type"] == "form"
    assert component["single_use"] is True
    assert component["restricted_to"] == "any"
    names = [f["name"] for f in component["definition"]["fields"]]
    assert names == ["description"]
    await submit(world, thread, card, {"description": "It fails at boot."})
    issues = world.jira.issues()
    assert len(issues) == 1
    fields = issues[0].body["fields"]
    assert fields["project"] == {"key": "SUP"}
    assert fields["issuetype"] == {"name": "Task"}
    assert fields["summary"] == "Crash on start"
    assert fields["description"]["type"] == "doc"
    assert last_paragraph(fields["description"]) == "Created from chat."
    assert fields["description"]["content"][0]["content"][0]["text"] == "It fails at boot."
    assert [r[1] for r in world.chat.responses] == [
        {"state": "acknowledged", "response": {"text": "Created issue SUP-1"}}
    ]
    texts = [m["content_md"] for m in thread.messages]
    assert texts[-1].startswith("Created Jira issue [SUP-1](")
    assert texts[-1].endswith("/browse/SUP-1).")
    # The echo of the created issue adds no message.
    assert len(thread.messages) == 3
    assert world.logs_with("own_jira_object")
    # A later comment of Sam lands in the post of /issue.
    issue_id = next(k for k, v in world.jira.known_issues.items() if v == "SUP-1")
    status = await world.deliver(
        render_fixture(
            "comment_created", issue_id=issue_id, key="SUP-1", comment_id="30001", actor="Sam"
        )
    )
    assert status == 202
    await world.settle()
    assert thread.messages[-1]["content_md"].startswith("`Sam` commented on [SUP-1](")
    assert len(world.chat.threads) == 1


async def test_echo_delivered_before_the_create_call_returned(world: World) -> None:
    world.jira.echo_before_answer = True
    thread, card = await open_form(world)
    await submit(world, thread, card, {"description": ""})
    assert world.jira.echo_statuses == [202]
    assert len(world.jira.issues()) == 1
    assert len(thread.messages) == 3
    assert world.logs_with("own_jira_object")
    assert len(world.chat.threads) == 1


async def test_form_offers_a_project_choice_when_a_forum_has_two(tmp_path: Path) -> None:
    w = await start_world(
        tmp_path,
        members={
            "projects": [
                {"project": "SUP", "channel_id": SUP_FORUM, "issue_type": "Task"},
                {"project": "WEB", "channel_id": SUP_FORUM, "issue_type": "Bug"},
            ]
        },
    )
    try:
        thread, card = await open_form(w)
        fields = card["components"][0]["definition"]["fields"]
        assert [f["name"] for f in fields] == ["description", "project"]
        assert fields[1]["choices"] == ["SUP", "WEB"]
        await submit(w, thread, card, {"project": "WEB", "description": "x"})
        assert w.jira.issues()[0].body["fields"]["project"] == {"key": "WEB"}
        assert w.jira.issues()[0].body["fields"]["issuetype"] == {"name": "Bug"}
    finally:
        await w.close()


@pytest.mark.parametrize("fault", ["400", "302"])
async def test_refused_issue_gets_failed_and_a_new_card(world: World, fault: str) -> None:
    thread, card = await open_form(world)
    world.jira.fail(fault)
    await submit(world, thread, card, {"description": "x"})
    status = 302 if fault == "302" else 400
    assert world.chat.responses[-1][1] == {
        "state": "failed",
        "response": {"text": f"Jira refused the issue (status {status})."},
    }
    cards = [m for m in thread.messages if m["kind"] == "card"]
    assert len(cards) == 2
    assert world.jira.trap_requests == []
    # The new card works: the second submission creates the issue.
    await submit(world, thread, world.chat.card_of(thread) or {}, {"description": "y"})
    assert len(world.jira.issues()) == 2
    assert world.chat.responses[-1][1]["state"] == "acknowledged"


async def test_server_error_answers_failed_without_new_card(world: World) -> None:
    thread, card = await open_form(world)
    world.jira.fail("500")
    await submit(world, thread, card, {"description": "x"})
    assert world.chat.responses[-1][1] == {
        "state": "failed",
        "response": {
            "text": "Jira did not confirm the issue. Check the project before you try again."
        },
    }
    assert len([m for m in thread.messages if m["kind"] == "card"]) == 1
    assert len(world.jira.issues()) == 1
    # The draft is gone: a second submission of the old card expired.
    await submit(world, thread, card, {"description": "x"})
    assert len(world.jira.issues()) == 1
    assert world.chat.responses[-1][1]["state"] == "rejected"


async def test_time_out_answers_failed_without_second_post(tmp_path: Path) -> None:
    # sdk.http_timeout is the limit of the Jira call; the Fake-Jira never answers in time.
    w = await start_world(tmp_path, settings=SdkSettings(http_timeout=0.5, retry_backoff_cap=0.01))
    try:
        thread, card = await open_form(w)
        w.jira.fail("hang")
        await submit(w, thread, card, {"description": "x"})
        assert w.chat.responses[-1][1]["state"] == "failed"
        assert "did not confirm" in w.chat.responses[-1][1]["response"]["text"]
        assert len(w.jira.issues()) == 1
        assert len([m for m in thread.messages if m["kind"] == "card"]) == 1
    finally:
        await w.close()


async def test_restart_after_attempted_sends_no_second_post(world: World) -> None:
    thread, card = await open_form(world)
    await world.stop_app()
    # A crash after the job marked its Jira step attempted: the job is still queued.
    store = SqliteStore(world.config.state_dir, now=time.time, window=PATIENCE)
    await store.open()
    data = {
        "interaction_id": "0198a4f2-6c1e-7a3b-9d2e-0000000000c1",
        "custom_id": card["components"][0]["custom_id"],
        "message_id": world.chat.message_id_of_card(thread),
        "values": {"description": "x"},
    }
    result = await store.admit(SeenKey("webhook_id", "crash"), 202, NewJob("issue_form", data), 10)
    job = await store.next_job()
    assert job is not None
    assert job.id == result.job_id
    job.progress["jira"] = "attempted"
    await store.save_progress(job)
    await store.close()
    await world.restart()
    await world.settle()
    assert world.jira.issues() == []
    assert world.chat.responses[-1][1]["state"] == "failed"
    assert "did not confirm" in world.chat.responses[-1][1]["response"]["text"]


async def test_same_webhook_id_twice_gives_one_job(world: World) -> None:
    event = command_event("Twice")
    first = await world.server_request("/chat/interactions", event)
    second = await world.server_request("/chat/interactions", event)
    assert (first.status_code, second.status_code) == (202, 202)
    await world.settle()
    assert len(world.chat.calls_of("createThread")) == 1


async def test_unknown_custom_id_is_rejected(world: World) -> None:
    event = interaction_event(
        custom_id="issue:0198a4f2-6c1e-7a3b-9d2e-0000000000ff",
        message_id="0198a4f2-6c1e-7a3b-9d2e-0000000000fe",
        values={},
    )
    assert (await world.server_request("/chat/interactions", event)).status_code == 202
    await world.settle()
    assert world.chat.responses[-1][1] == {
        "state": "rejected",
        "response": {"text": "This form expired. Run /issue again."},
    }
    assert world.jira.issues() == []


async def test_respond_conflict_counts_as_done(world: World) -> None:
    thread, card = await open_form(world)
    world.chat.fail("respondToInteraction", problem(409, "conflict"))
    await submit(world, thread, card, {"description": "x"})
    assert len(world.jira.issues()) == 1
    assert thread.messages[-1]["content_md"].startswith("Created Jira issue")


async def test_autocomplete_answers_empty_choices(world: World) -> None:
    event = envelope(
        "command.autocomplete_requested",
        {"command_name": "issue", "option": "title", "value": "x", "channel_id": SUP_FORUM},
        channel_id=SUP_FORUM,
    )
    response = await world.server_request("/chat/interactions", event)
    assert response.status_code == 200
    assert response.json() == {"choices": []}


async def test_unknown_request_is_202(world: World) -> None:
    event = envelope("command.invoked", {"command_name": "other"}, channel_id=SUP_FORUM)
    response = await world.server_request("/chat/interactions", event)
    assert response.status_code == 202
    assert world.logs_with("unknown_request")


async def test_command_in_an_unmapped_forum_ends(world: World) -> None:
    event = command_event("Elsewhere", channel_id="0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d99")
    assert (await world.server_request("/chat/interactions", event)).status_code == 202
    await world.settle()
    assert world.chat.threads == {}
    assert world.logs_with("channel_not_mapped")


async def test_forged_server_requests_are_refused(world: World) -> None:
    unknown_key = await world.server_request(
        "/chat/interactions", command_event("A"), seed=OTHER_SEED
    )
    too_old = await world.server_request(
        "/chat/interactions",
        command_event("B"),
        timestamp=int(time.time()) - int(world.settings.webhook_receiver_tolerance) - 60,
    )
    changed = await world.server_request(
        "/chat/interactions",
        command_event("C"),
        tamper=lambda body: body.replace(b'"C"', b'"D"'),
    )
    assert [r.status_code for r in (unknown_key, too_old, changed)] == [401, 401, 401]
    await world.settle()
    assert world.chat.calls_of("createThread") == []
    assert not world.logs_with("delivery.accepted")
    codes = sorted(line["code"] for line in world.logs_with("verification"))
    assert codes == ["signature_mismatch", "signature_mismatch", "timestamp_out_of_tolerance"]


async def test_invalid_payload_of_the_server_is_400(world: World) -> None:
    event = envelope("interaction.created", {"interaction": {"id": 5}}, channel_id=SUP_FORUM)
    assert (await world.server_request("/chat/interactions", event)).status_code == 400
    raw = b'{"id": "x", "type": "command.invoked", "type": "interaction.created"}'
    response = await world.server_request(
        "/chat/interactions", {}, webhook_id="0198a4f2-6c1e-7a3b-9d2e-0000000000aa", body=raw
    )
    assert response.status_code == 400
