# SPDX-License-Identifier: Apache-2.0
"""Thread actions of the agent on the FakeServer: title, done, reopen (09 12, 14; 01 6.1)."""

from __future__ import annotations

import json
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from uuid import UUID

import pytest
from adapter_kit import settings_for
from bothy_sdk.testing.fakeserver import FakeServer
from bothy_sdk.testing.faults import Fault

from bothy_hermes_adapter.__main__ import build_parser, platform_extra, run_thread_action
from bothy_hermes_adapter.reports import standalone_send
from bothy_hermes_adapter.threads import thread_action

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _extra(fake: FakeServer, root: Path) -> dict[str, Any]:
    settings = settings_for(fake, root)
    return {
        "hermes_adapter.server_url": fake.base_url,
        "hermes_adapter.instance_id": str(fake.instance_id),
        "hermes_adapter.token_file": str(settings.token_file),
    }


async def test_ticket_post_title_done_answer_and_reopen(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        extra = _extra(fake, tmp_path)
        forum = fake.add_forum("tickets", None)
        posted = await standalone_send(SimpleNamespace(extra=extra), str(forum), "Printer\nJam")
        thread = posted["message_id"]

        # The status in the title, set again and again on the agent's own post.
        for title in ("[IN PROGRESS] Printer", "[DONE] Printer"):
            result = await thread_action(extra, "title", thread, title)
            assert result["ok"] is True, result
            assert (result["title"], result["title_source"]) == (title, "auto")

        # Done: the state of the app's Done button.
        closed = await thread_action(SimpleNamespace(extra=extra), "close", thread)
        assert (closed["ok"], closed["state"]) == (True, "closed")
        assert fake.thread(UUID(thread))["closed_by"] == str(fake.world.lotse.id)
        assert (await thread_action(extra, "close", thread))["state"] == "closed"

        # A person's answer in the done post reopens it and starts a run of the agent.
        answer = await fake.trigger("It jams again", channel_id=forum, thread_id=UUID(thread))
        assert fake.thread(UUID(thread))["state"] == "open"
        assert fake.reopened_by_message == [UUID(thread)]
        assert fake.run_state(answer.run_id).state == "queued"

        # The agent writes into a done post: accepted, and the post is open again.
        await thread_action(extra, "close", thread)
        sent = await standalone_send(SimpleNamespace(extra=extra), str(forum), "Note", thread)
        assert sent.get("success") is True, sent
        assert fake.thread(UUID(thread))["state"] == "open"

        # An explicit reopen after a close.
        await thread_action(extra, "close", thread)
        reopened = await thread_action(extra, "reopen", thread)
        assert (reopened["ok"], reopened["state"]) == (True, "open")
        assert (await thread_action(extra, "reopen", thread))["state"] == "open"
        agents = {
            q.headers["user-agent"].split("/")[0]
            for q in fake.requests
            if q.operation_id in ("closeThread", "reopenThread")
        }
        assert agents == {"hermes-adapter-thread"}


async def test_refusals_and_bad_input(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        extra = _extra(fake, tmp_path)
        forum = fake.add_forum("tickets", None)
        thread = (await standalone_send(SimpleNamespace(extra=extra), str(forum), "T"))[
            "message_id"
        ]
        before = len(fake.requests)
        assert (await thread_action(extra, "delete", thread))["error"] == "unknown action: delete"
        assert (await thread_action(extra, "title", thread, "  "))["error"] == "the title is empty"
        assert (await thread_action(extra, "close", "#7"))["error"] == (
            "the thread is not a thread ID of this server"
        )
        assert (await thread_action({}, "close", thread))["error"] == (
            "the platform configuration is not usable"
        )
        assert len(fake.requests) == before

        # A title a person set stays: the operator agent gets title_manual (09 12).
        fake.set_manual_title(UUID(thread), "Printer on floor 2")
        manual = await thread_action(extra, "title", thread, "[DONE] Printer")
        assert manual == {
            "ok": False,
            "error": "refused by the server: title_manual",
            "code": "title_manual",
            "status": 409,
        }
        fake.inject("closeThread", Fault.problem("forbidden"))
        refused = await thread_action(extra, "close", thread)
        assert (refused["ok"], refused["code"]) == (False, "forbidden")
        assert fake.thread(UUID(thread))["state"] == "open"


def test_thread_command_line() -> None:
    args = build_parser().parse_args(
        [
            "thread",
            "title",
            "0192f4a0-7b1c-7000-8000-000000000001",
            "--title",
            "Done",
            "--profile-home",
            "/srv/hermes",
        ]
    )
    assert (args.command, args.action, args.title, args.platform_key) == (
        "thread",
        "title",
        "Done",
        "bothy",
    )
    config = {"gateway": {"platforms": {"support": {"extra": {"k": "v"}}}}}
    assert platform_extra(config, "support") == {"k": "v"}
    assert platform_extra(config, "bothy") == {}
    assert platform_extra({"gateway": None}, "bothy") == {}


def test_thread_command_with_a_broken_config(capsys: pytest.CaptureFixture[str]) -> None:
    class Broken:
        def read_config(self) -> dict[str, Any]:
            raise ValueError("bad YAML")

    args = build_parser().parse_args(
        ["thread", "close", "0192f4a0-7b1c-7000-8000-000000000001", "--profile-home", "/srv/h"]
    )
    assert run_thread_action(Broken(), args) == 1
    assert json.loads(capsys.readouterr().out) == {
        "ok": False,
        "error": "the platform configuration is not usable",
    }
