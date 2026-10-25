# SPDX-License-Identifier: Apache-2.0
"""The test world: secret files of mode ``0600``, a loopback-only configuration, the fakes and
the app started with a captured logger and small SDK waits.

`make_config` refuses a ``jira_base_url`` or ``server_url`` outside the loopback host (brief
M2-50 Design note 19). Waits poll for a state with `PATIENCE` as the only upper bound.
"""

from __future__ import annotations

import asyncio
import json
import logging
import time
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

import httpx
from bothy_sdk.clock import Clock
from bothy_sdk.ids import uuid7
from bothy_sdk.settings import SdkSettings

from jira_app.app import AppDeps, RunningApp, start_app
from jira_app.config import AppConfig, is_loopback_host, load_config
from jira_app.log import AppLog
from jira_app.store.base import Store
from tests.fakes.chat import SEED, FakeChat, body_of, envelope
from tests.fakes.jira_api import FakeJira

PATIENCE = 120.0
"""The only upper bound of a wait for a condition (never reached on a healthy run)."""

SUP_FORUM = "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90"
WEB_FORUM = "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d91"
OTHER_FORUM = "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d92"

SECRETS = {
    "webhook_secret": f"jira-secret-{uuid.uuid4()}",
    "jira_email": "sam@example.com",
    "jira_token": f"jira-api-token-{uuid.uuid4()}",
    "app_token": f"app-token-{uuid.uuid4()}",
}
"""Values the logs test looks for; every one is a secret."""

CREDENTIALS = f"{SECRETS['jira_email']}:{SECRETS['jira_token']}"


def write_secret(path: Path, text: str, mode: int = 0o600) -> Path:
    """Write a secret file with ``mode``."""
    path.write_text(text, encoding="utf-8")
    path.chmod(mode)
    return path


def make_config(
    directory: Path, *, jira_base_url: str, server_url: str = "http://127.0.0.1:9", **members: Any
) -> AppConfig:
    """Write the secret files and a configuration into ``directory`` and load it.

    Raises:
        ValueError: ``jira_base_url`` or ``server_url`` is not on the loopback host.
    """
    for key, value in (("jira_base_url", jira_base_url), ("server_url", server_url)):
        if not is_loopback_host(urlsplit(value).hostname or ""):
            raise ValueError(f"the test configuration needs a loopback {key}")
    token = directory / "app-token"
    if not token.exists():
        write_secret(token, SECRETS["app_token"])
        write_secret(directory / "webhook-secret", SECRETS["webhook_secret"])
        write_secret(directory / "jira-credentials", CREDENTIALS)
    document = {
        "listen": "127.0.0.1:0",
        "server_url": server_url,
        "token_file": str(token),
        "webhook_secret_file": str(directory / "webhook-secret"),
        "jira_base_url": jira_base_url,
        "jira_credentials_file": str(directory / "jira-credentials"),
        "projects": [
            {"project": "SUP", "channel_id": SUP_FORUM, "issue_type": "Task"},
            {"project": "WEB", "channel_id": WEB_FORUM, "issue_type": "Bug"},
        ],
        "state_dir": str(directory / "state"),
        **members,
    }
    file = directory / "config.json"
    file.write_text(json.dumps({"jira_app": document}), encoding="utf-8")
    return load_config(file)


class ListHandler(logging.Handler):
    """Collects formatted lines."""

    def __init__(self) -> None:
        super().__init__()
        self.lines: list[str] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.lines.append(self.format(record))


async def eventually(check: Callable[[], bool | Awaitable[bool]], what: str) -> None:
    """Wait until ``check`` holds, with `PATIENCE` as the only bound."""
    deadline = time.monotonic() + PATIENCE
    while True:
        result = check()
        if not isinstance(result, bool):
            result = await result
        if result:
            return
        if time.monotonic() > deadline:
            raise AssertionError(f"timed out waiting for {what}")
        await asyncio.sleep(0.01)


@dataclass
class World:
    """A running world of the unit tests."""

    directory: Path
    config: AppConfig
    chat: FakeChat
    jira: FakeJira
    app: RunningApp
    handler: ListHandler
    settings: SdkSettings
    clock: Clock | None = None
    store: Store | None = None
    http: httpx.AsyncClient = field(default_factory=lambda: httpx.AsyncClient(timeout=PATIENCE))
    stopped: bool = False

    @property
    def logs(self) -> list[dict[str, Any]]:
        """Every log line, parsed."""
        return [json.loads(line) for line in self.handler.lines]

    def logs_with(self, value: str) -> list[dict[str, Any]]:
        """Lines whose ``msg`` or ``reason`` equals ``value``."""
        return [line for line in self.logs if value in (line.get("msg"), line.get("reason"))]

    async def deliver(self, payload: dict[str, Any] | bytes, **options: Any) -> int:
        """A Jira delivery to the app."""
        status = await self.jira.deliver(f"{self.app.url}/jira/webhook", payload, **options)
        return status

    async def server_request(
        self,
        path: str,
        event: dict[str, Any],
        *,
        webhook_id: str | None = None,
        timestamp: int | None = None,
        seed: bytes = SEED,
        body: bytes | None = None,
        tamper: Callable[[bytes], bytes] | None = None,
    ) -> httpx.Response:
        """A signed request of the server to the app."""
        wid = webhook_id or str(event.get("id") or uuid7())
        raw = body if body is not None else body_of({**event, "id": wid})
        if timestamp is None:
            timestamp = int(self.clock.now().timestamp()) if self.clock else int(time.time())
        headers = self.chat.sign(raw, wid, timestamp=timestamp, seed=seed)
        sent = tamper(raw) if tamper else raw
        return await self.http.post(f"{self.app.url}{path}", content=sent, headers=headers)

    async def settle(self) -> None:
        """Wait until the queue is empty and every echo of the Fake-Jira was answered."""
        while True:
            await self.app.idle()
            if not self.jira.pending_echoes():
                return
            await self.jira.echoes_done()

    async def stop_app(self) -> None:
        """Stop the app (the fakes keep running)."""
        if not self.stopped:
            self.stopped = True
            await self.app.stop()

    async def restart(self) -> None:
        """Stop the app and start a new one on the same state directory."""
        await self.stop_app()
        self.app = await start_app(
            self.config,
            AppDeps(
                chat=self.chat,
                log=AppLog(self.handler, logging.DEBUG),
                settings=self.settings,
                clock=self.clock,
                store=self.store,
            ),
        )
        self.stopped = False
        self.jira.auto_echo = f"{self.app.url}/jira/webhook"

    async def close(self) -> None:
        """Stop the app and the fakes."""
        await self.stop_app()
        await self.jira.stop()
        await self.http.aclose()


async def start_world(
    directory: Path,
    *,
    members: dict[str, Any] | None = None,
    chat: FakeChat | None = None,
    clock: Clock | None = None,
    store: Store | None = None,
    settings: SdkSettings | None = None,
) -> World:
    """Start the fakes and the app in ``directory``."""
    jira = await FakeJira.start(credentials=CREDENTIALS, secret=SECRETS["webhook_secret"])
    config = make_config(directory, jira_base_url=jira.url, **(members or {}))
    fake_chat = chat or FakeChat()
    handler = ListHandler()
    # No answer of the fakes comes close to the patience; the pause after a transient failure
    # is short, because the fakes recover at once.
    sdk_settings = settings or SdkSettings(http_timeout=PATIENCE, retry_backoff_cap=0.01)
    app = await start_app(
        config,
        AppDeps(
            chat=fake_chat,
            log=AppLog(handler, logging.DEBUG),
            settings=sdk_settings,
            clock=clock,
            store=store,
        ),
    )
    jira.auto_echo = f"{app.url}/jira/webhook"
    return World(directory, config, fake_chat, jira, app, handler, sdk_settings, clock, store)


def command_event(title: str, *, channel_id: str = SUP_FORUM) -> dict[str, Any]:
    """A ``command.invoked`` event of ``/issue``."""
    return envelope(
        "command.invoked",
        {
            "invocation_id": str(uuid7()),
            "command_id": str(uuid7()),
            "command_name": "issue",
            "control": {"kind": "app"},
            "channel_id": channel_id,
            "invoked_by": {
                "id": "0198a4f2-6c1e-7a3b-9d2e-000000000a01",
                "kind": "user",
                "handle": "alex",
                "display_name": "Alex",
            },
            "options": [{"name": "title", "value": title}],
        },
        channel_id=channel_id,
    )


def interaction_event(
    *, custom_id: str, message_id: str, values: dict[str, Any], channel_id: str = SUP_FORUM
) -> dict[str, Any]:
    """An ``interaction.created`` event of a submitted form."""
    return envelope(
        "interaction.created",
        {
            "interaction": {
                "id": str(uuid7()),
                "component_id": str(uuid7()),
                "custom_id": custom_id,
                "message_id": message_id,
                "principal_id": "0198a4f2-6c1e-7a3b-9d2e-000000000a01",
                "client_interaction_id": str(uuid7()),
                "values": values,
                "state": "pending",
                "created_at": "2026-10-13T09:00:00Z",
            }
        },
        channel_id=channel_id,
    )
