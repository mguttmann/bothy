# SPDX-License-Identifier: Apache-2.0
"""Start and stop of the app (brief M2-50 Design notes 3, 7 and 9)."""

from __future__ import annotations

import logging
from pathlib import Path

import pytest
from bothy_sdk.errors import ProblemError

from jira_app.app import AppDeps, start_app
from jira_app.config import ConfigError
from jira_app.log import AppLog
from jira_app.store.base import StoreLockedError
from tests.fakes.chat import FakeChat, problem
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
