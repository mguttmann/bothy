# SPDX-License-Identifier: Apache-2.0
"""Start and stop of the app (brief M2-50 Design notes 3, 7 and 9)."""

from __future__ import annotations

import json
import logging
import time
from pathlib import Path

import httpx
import pytest
from bothy_sdk.errors import ProblemError

from jira_app.app import AppDeps, start_app
from jira_app.config import ConfigError
from jira_app.log import AppLog
from jira_app.store.base import JobRecord, StoreError, StoreLockedError
from jira_app.store.memory_store import MemoryStore
from jira_app.web import start_http
from tests.fakes.chat import FakeChat, problem
from tests.fakes.jira_api import render_fixture
from tests.helpers import ListHandler, make_config, start_world

pytestmark = pytest.mark.anyio


async def test_a_refused_start_releases_the_lock(tmp_path: Path) -> None:
    config = make_config(tmp_path, jira_base_url="http://127.0.0.1:9")
    chat = FakeChat()
    for operation in ("getMe", "getInstance", "getWebhookKeys"):
        chat.fail(operation, problem(401, "unauthenticated"))
        with pytest.raises(ProblemError):
            await start_app(config, AppDeps(chat=chat, log=AppLog(ListHandler())))
    app = await start_app(config, AppDeps(chat=chat, log=AppLog(ListHandler())))
    await app.stop()
    await app.stop()


async def test_a_second_app_on_the_state_directory_fails(tmp_path: Path) -> None:
    world = await start_world(tmp_path)
    try:
        with pytest.raises(StoreLockedError) as caught:
            await start_app(world.config, AppDeps(chat=FakeChat(), log=AppLog(ListHandler())))
        assert world.config.state_dir in str(caught.value)
    finally:
        await world.close()


async def test_a_secret_file_with_group_access_aborts_the_start(tmp_path: Path) -> None:
    config = make_config(tmp_path, jira_base_url="http://127.0.0.1:9")
    (tmp_path / "jira-credentials").chmod(0o640)
    handler = ListHandler()
    with pytest.raises(ConfigError) as caught:
        await start_app(config, AppDeps(chat=FakeChat(), log=AppLog(handler, logging.DEBUG)))
    assert "jira_app.jira_credentials_file" in str(caught.value)
    assert "sam@example.com" not in str(caught.value)


class FlakyStore(MemoryStore):
    """Fails the first write of a job's progress, as a full disk would."""

    def __init__(self) -> None:
        super().__init__(now=time.time, window=3600.0)
        self.failures = 1

    async def save_progress(self, job: JobRecord) -> None:
        if self.failures:
            self.failures -= 1
            raise StoreError("the state database failed")
        await super().save_progress(job)


async def test_store_failure_inside_a_job_keeps_it_queued(tmp_path: Path) -> None:
    store = FlakyStore()
    world = await start_world(tmp_path, store=store)
    try:
        assert await world.deliver(render_fixture("jira:issue_created")) == 202
        await world.settle()
        assert world.logs_with("job.transient")
        assert len(world.chat.threads) == 1
    finally:
        await world.close()


async def test_an_unexpected_failure_names_its_class_in_job_failed(tmp_path: Path) -> None:
    chat = FakeChat()
    chat.fail("createThread", LookupError("unexpected"))
    world = await start_world(tmp_path, chat=chat)
    try:
        assert await world.deliver(render_fixture("jira:issue_created")) == 202
        await world.settle()
        failed = [line for line in world.logs if line.get("msg") == "job.failed"]
        assert failed == [failed[0]]
        assert failed[0]["reason"] == "internal"
        assert failed[0]["code"] == "LookupError"
    finally:
        await world.close()


async def test_a_failing_route_names_its_class_in_http_failed() -> None:
    async def broken(request: object) -> object:
        del request
        raise TypeError("broken")

    handler = ListHandler()
    server = await start_http(
        "127.0.0.1",
        0,
        routes={"/x": broken},  # type: ignore[dict-item]
        max_body_bytes=1024,
        ready=lambda: True,
        log=AppLog(handler),
    )
    try:
        async with httpx.AsyncClient() as http:
            response = await http.post(f"http://127.0.0.1:{server.port}/x", json={})
        assert response.status_code == 503
        lines = [json.loads(line) for line in handler.lines]
        assert {"msg": "http.failed", "reason": "internal", "code": "TypeError"}.items() <= next(
            line for line in lines if line["msg"] == "http.failed"
        ).items()
    finally:
        await server.close()
