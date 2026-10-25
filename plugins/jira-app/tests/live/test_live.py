# SPDX-License-Identifier: Apache-2.0
"""The app against the real server (brief M2-50 Design note 20; PLAN 8 Exit criterion 9).

The server of this tree through ``sdkserver`` with a TLS front for the app's addresses, the app
and the Fake-Jira API in this process, every operator step through the public API. Waits poll
for a state with the patience as the only bound. The tests run in file order and share one
installation.
"""

from __future__ import annotations

import json
import logging
import uuid
from collections.abc import AsyncIterator
from dataclasses import dataclass, field
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.ids import uuid7
from bothy_sdk.settings import SdkSettings

from jira_app.app import AppDeps, RunningApp, start_app
from jira_app.log import AppLog
from tests.fakes.jira_api import FakeJira, render_fixture
from tests.helpers import ListHandler, eventually, make_config, write_secret
from tests.live.sdkserver import HOST, LiveServer

pytestmark = pytest.mark.anyio

APP_DIR = Path(__file__).resolve().parents[2]
REQUIRED = ["messages:write", "commands:write", "components:write"]
WEBHOOK_SECRET = f"jira-live-secret-{uuid.uuid4()}"
JIRA_CREDENTIALS = f"sam@example.com:jira-live-token-{uuid.uuid4()}"


@dataclass
class Live:
    """The shared state of the live tests."""

    server: LiveServer
    jira: FakeJira
    directory: Path
    forum: str
    manifest: dict[str, Any]
    handler: ListHandler = field(default_factory=ListHandler)
    app: RunningApp | None = None
    app_id: str = ""
    subscription_id: str = ""

    @property
    def logs(self) -> list[dict[str, Any]]:
        return [json.loads(line) for line in self.handler.lines]

    async def new_token(self, scopes: list[str] | None = None) -> None:
        body: dict[str, Any] = {
            "name": f"jira-app-{uuid7()}",
            "expires_at": (datetime.now(UTC) + timedelta(days=1)).isoformat(),
        }
        if scopes is not None:
            body["scopes"] = scopes
        created = await self.server.call(
            "owner", "createToken", path={"principal_id": self.app_id}, body=body
        )
        assert created.status == 201
        write_secret(self.directory / "app-token", str(created.body["secret"]))

    async def run_app(self) -> None:
        if self.app is not None:
            await self.app.stop()
        config = make_config(
            self.directory,
            jira_base_url=self.jira.url,
            server_url=self.server.origin,
            projects=[{"project": "SUP", "channel_id": self.forum, "issue_type": "Task"}],
        )
        self.app = await start_app(
            config,
            AppDeps(
                log=AppLog(self.handler, logging.DEBUG),
                settings=SdkSettings(retry_backoff_base=0.1, retry_backoff_cap=1.0),
            ),
        )
        await self.server.app_backend(self.app.port)
        self.jira.auto_echo = f"{self.app.url}/jira/webhook"

    async def deliver(self, payload: dict[str, Any]) -> int:
        assert self.app is not None
        return await self.jira.deliver(f"{self.app.url}/jira/webhook", payload)

    async def threads(self) -> list[dict[str, Any]]:
        page = await self.server.call("alex", "listChannelThreads", path={"channel_id": self.forum})
        assert page.status == 200
        return list(page.body.get("items", []))

    async def messages(self, thread_id: str) -> list[dict[str, Any]]:
        page = await self.server.call("alex", "listThreadMessages", path={"thread_id": thread_id})
        assert page.status == 200
        return list(page.body.get("items", []))

    async def thread_titled(self, prefix: str) -> dict[str, Any] | None:
        for thread in await self.threads():
            if str(thread.get("title", "")).startswith(prefix):
                return thread
        return None

    async def settle(self) -> None:
        assert self.app is not None
        while True:
            await self.app.idle()
            if not self.jira.pending_echoes():
                return
            await self.jira.echoes_done()


@pytest.fixture(scope="module")
async def live(
    live_server: LiveServer, tmp_path_factory: pytest.TempPathFactory
) -> AsyncIterator[Live]:
    directory = tmp_path_factory.mktemp("jira-app-live")
    directory.chmod(0o700)
    write_secret(directory / "webhook-secret", WEBHOOK_SECRET)
    write_secret(directory / "jira-credentials", JIRA_CREDENTIALS)
    jira = await FakeJira.start(
        credentials=JIRA_CREDENTIALS, secret=WEBHOOK_SECRET, projects={"SUP": "Task"}
    )
    forum = await live_server.create_forum("issues")
    manifest = json.loads((APP_DIR / "manifest.json").read_text(encoding="utf-8"))
    base = f"https://{HOST}:{live_server.front_port}"
    manifest["delivery"] = {
        "mode": "webhook",
        "events_url": f"{base}/chat/events",
        "interactions_url": f"{base}/chat/interactions",
    }
    state = Live(live_server, jira, directory, forum, manifest)
    try:
        yield state
    finally:
        if state.app is not None:
            await state.app.stop()
        await jira.stop()


async def test_installs_with_exactly_the_required_scopes(live: Live) -> None:
    valid = await live.server.call("owner", "validateManifest", body=live.manifest)
    assert valid.status == 200
    assert valid.body["valid"] is True
    short = await live.server.call(
        "owner",
        "installApp",
        body={
            "manifest": live.manifest,
            "granted_scopes": ["messages:write", "commands:write"],
            "reason": "test",
        },
    )
    assert short.status == 400
    assert short.body["code"] == "validation_failed"
    assert "/granted_scopes" in json.dumps(short.body.get("errors"))
    installed = await live.server.call(
        "owner",
        "installApp",
        body={"manifest": live.manifest, "granted_scopes": REQUIRED, "reason": "test"},
    )
    assert installed.status == 201
    live.app_id = str(installed.body["id"])
    grant = await live.server.call(
        "owner",
        "createGrant",
        body={
            "subject_kind": "app",
            "subject_id": live.app_id,
            "target_kind": "channel",
            "target_id": live.forum,
            "level": "write",
            "reason": "test",
        },
    )
    assert grant.status == 201
    await live.new_token()
    await live.run_app()


async def test_one_thread_per_issue_with_messages_of_origin_app(live: Live) -> None:
    assert await live.deliver(render_fixture("jira:issue_created")) == 202
    assert await live.deliver(render_fixture("comment_created")) == 202
    status = render_fixture(
        "jira:issue_updated",
        changelog=[{"field": "status", "fromString": "To Do", "toString": "In Progress"}],
    )
    assert await live.deliver(status) == 202
    created = render_fixture("jira:issue_created", issue_id="10013", key="SUP-13", summary="Crash")
    assert await live.deliver(created) == 202
    await live.settle()
    posts = await live.threads()
    assert sorted(str(t["title"]).split(" ")[0] for t in posts) == ["SUP-12", "SUP-13"]
    thread12 = await live.thread_titled("SUP-12 ")
    assert thread12 is not None
    in_thread = await live.messages(str(thread12["id"]))
    assert [m["origin"] for m in in_thread] == ["app", "app", "app"]


async def test_issue_command_creates_an_issue_with_the_form(live: Live) -> None:
    commands = await live.server.call(
        "alex", "listChannelCommands", path={"channel_id": live.forum}
    )
    assert commands.status == 200
    command = next(c for c in commands.body["items"] if c["name"] == "issue")
    before = len(await live.threads())
    invoked = await live.server.call(
        "alex",
        "invokeCommand",
        path={"channel_id": live.forum, "command_id": str(command["id"])},
        body={"options": [{"name": "title", "value": "Crash on start"}]},
    )
    assert invoked.status == 202

    async def post_with_card() -> bool:
        post = await live.thread_titled("Crash on start")
        if post is None:
            return False
        return any(m["kind"] == "card" for m in await live.messages(str(post["id"])))

    await eventually(post_with_card, "the post of /issue with its form")
    post = await live.thread_titled("Crash on start")
    assert post is not None
    card = next(m for m in await live.messages(str(post["id"])) if m["kind"] == "card")
    component = card["components"][0]
    submitted = await live.server.call(
        "alex",
        "createInteraction",
        path={"component_id": str(component["id"])},
        body={"client_interaction_id": str(uuid7()), "values": {"description": "It fails."}},
    )
    assert submitted.status == 201
    await eventually(lambda: len(live.jira.issues()) == 1, "the issue at the Fake-Jira")

    async def confirmed() -> bool:
        return any(
            str(m["content_md"]).startswith("Created Jira issue")
            for m in await live.messages(str(post["id"]))
        )

    await eventually(confirmed, "the confirmation in the post")
    await live.settle()
    assert any(line.get("reason") == "own_jira_object" for line in live.logs)
    assert len(await live.messages(str(post["id"]))) == 3
    assert len(await live.threads()) == before + 1


async def test_mirrors_a_reply_once_and_never_the_own_message(live: Live) -> None:
    scopes = await live.server.call(
        "owner",
        "setAppScopes",
        path={"app_id": live.app_id},
        body={
            "granted_scopes": [*REQUIRED, "messages:read", "events:subscribe"],
            "event_subscriptions": ["message.created"],
            "reason": "test",
        },
    )
    assert scopes.status == 200
    subscription = await live.server.call(
        "owner",
        "createWebhookSubscription",
        path={"app_id": live.app_id},
        body={
            "url": live.manifest["delivery"]["events_url"],
            "event_types": ["message.created"],
        },
    )
    assert subscription.status == 201
    live.subscription_id = str(subscription.body["subscription"]["id"])
    await live.new_token()
    await live.run_app()

    thread12 = await live.thread_titled("SUP-12 ")
    assert thread12 is not None
    thread_id = str(thread12["id"])
    before = len(await live.messages(thread_id))
    comment = render_fixture("comment_created", comment_id="10060", text="One more detail.")
    assert await live.deliver(comment) == 202

    async def app_comment() -> bool:
        return len(await live.messages(thread_id)) == before + 1

    await eventually(app_comment, "the app's comment message")

    async def delivered() -> bool:
        page = await live.server.call(
            "owner", "listWebhookDeliveries", path={"subscription_id": live.subscription_id}
        )
        return any(d.get("status") == "succeeded" for d in page.body.get("items", []))

    await eventually(delivered, "the delivery of the app's own message.created")
    await eventually(
        lambda: any(line.get("reason") == "own_message" for line in live.logs), "own_message"
    )
    await live.settle()
    assert live.jira.comments() == []

    reply = await live.server.call(
        "alex",
        "createMessage",
        path={"thread_id": thread_id},
        body={"client_msg_id": str(uuid7()), "content_md": "Thanks, looking into it."},
    )
    assert reply.status == 201
    await eventually(lambda: len(live.jira.comments()) == 1, "the mirrored comment")
    mirrored = live.jira.comments()[0]
    assert mirrored.path == "/rest/api/3/issue/10012/comment"
    assert "Thanks, looking into it." in json.dumps(mirrored.body)
    await live.settle()
    after = (await live.messages(thread_id))[before:]
    # The world's agent may answer the reply with a run message; the app adds only its one
    # comment message, and the echoes of the mirrored comment add none.
    kinds = [f"{m['origin']}/{m['kind']}" for m in after if m["origin"] != "agent"]
    assert kinds == ["app/text", "human/text"]


async def test_scope_missing_is_logged_once(live: Live) -> None:
    await live.new_token(["commands:write", "components:write"])
    await live.run_app()
    at = len(live.handler.lines)
    created = render_fixture("jira:issue_created", issue_id="10014", key="SUP-14", summary="Gone")
    assert await live.deliver(created) == 202
    await eventually(
        lambda: any(line.get("reason") == "scope_missing" for line in live.logs[at:]),
        "the scope_missing line",
    )
    await live.settle()
    assert len([line for line in live.logs[at:] if line.get("reason") == "scope_missing"]) == 1
    assert await live.thread_titled("SUP-14 ") is None


async def test_no_secret_in_the_logs_and_only_the_fakes_reached(live: Live) -> None:
    text = "\n".join(live.handler.lines)
    token = (live.directory / "app-token").read_text(encoding="utf-8")
    for secret in (WEBHOOK_SECRET, JIRA_CREDENTIALS, token):
        assert secret not in text
    for request in live.jira.requests:
        assert request.path.startswith("/rest/api/3/issue")
    assert live.jira.trap_requests == []
