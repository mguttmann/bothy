# SPDX-License-Identifier: Apache-2.0
"""Operator and binding changes reach a running runner without a restart (09 18.1).

On the server an operator change writes only ``access.changed`` to the old and the new operator
(04 8.2), never ``agent.config_changed``; a binding change writes ``agent.config_changed`` to
the effective operator only. The runner reloads its served channels on ``access.changed`` and
on a timer (``runner.channels_resync_interval``) and reports the new readiness at once.
"""

from __future__ import annotations

from typing import Any
from uuid import UUID

import pytest

from .conftest import World

pytestmark = pytest.mark.anyio


def readiness_of(beat: dict[str, Any], channel: UUID) -> dict[str, Any] | None:
    for entry in beat.get("project_readiness") or []:
        if entry["channel_id"] == str(channel):
            return {k: v for k, v in entry.items() if k != "channel_id"}
    return None


def ready_state(beat: dict[str, Any], channel: UUID) -> dict[str, Any] | None:
    """The readiness entry without the folder check of 18.15, which the trash timer adds."""
    entry = readiness_of(beat, channel)
    return None if entry is None else {k: v for k, v in entry.items() if k != "trash_check"}


def _ready_since(world: World, count: int, channel: UUID) -> bool:
    return any(ready_state(b, channel) == {"ready": True} for b in world.fake.heartbeats[count:])


def _gone_since(world: World, count: int, channel: UUID) -> bool:
    beats = world.fake.heartbeats[count:]
    return bool(beats) and readiness_of(beats[-1], channel) is None


def _app_folder(world: World) -> None:
    (world.projects / "app" / ".git").mkdir(parents=True)


async def test_new_operator_is_served_without_restart_and_unbinding_stops_it(
    world: World,
) -> None:
    fake = world.fake
    _app_folder(world)
    # A forum another agent operates, bound before the runner becomes its operator: the binding
    # event went to that other agent, so the runner hears nothing of it.
    forum = fake.add_forum("Projekt-C", None, served=False)
    fake.update_forum(forum, workspace_ref="app")
    await world.start()
    assert all(readiness_of(b, forum) is None for b in fake.heartbeats)

    count = len(fake.heartbeats)
    fake.set_operator(forum)
    await world.until(lambda: _ready_since(world, count, forum))
    assert len(world.runners) == 1  # the same runner process, never restarted

    run = await fake.trigger("Please add a readme", channel_id=forum)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    assert turn["cwd"] == str((world.projects / "app").resolve())
    assert [m for m in fake.messages if m["run_id"] == str(run.run_id)]

    count = len(fake.heartbeats)
    fake.unbind_forum(forum)
    await world.until(lambda: _gone_since(world, count, forum))
    refused = await fake.trigger("And now?", channel_id=forum)
    assert await world.ended(refused.run_id) == "failed"
    assert fake.run_state(refused.run_id).state_reason == "adapter_error"
    assert len(world.turns()) == 1


async def test_operator_removed_drops_the_forum(world: World) -> None:
    fake = world.fake
    await world.start()
    assert any(ready_state(b, world.forum_a) == {"ready": True} for b in fake.heartbeats)
    count = len(fake.heartbeats)
    fake.set_operator(world.forum_a, served=False)
    await world.until(lambda: _gone_since(world, count, world.forum_a))


async def test_binding_created_on_a_served_forum_is_ready_at_once(world: World) -> None:
    fake = world.fake
    _app_folder(world)
    await world.start()
    count = len(fake.heartbeats)
    fake.update_forum(world.werkzeuge, workspace_ref="app")
    await world.until(lambda: _ready_since(world, count, world.werkzeuge))
    run = await fake.trigger("Hello", channel_id=world.werkzeuge)
    assert await world.ended(run.run_id) == "completed"


async def test_periodic_resync_without_an_event(world: World) -> None:
    fake = world.fake
    _app_folder(world)
    forum = fake.add_forum("Projekt-C", "app", served=False)
    await world.start(world.config(channels_resync_interval="200ms"))
    count = len(fake.heartbeats)
    # The server writes no access.changed when the visible targets of the agent do not change
    # (04 8.2), so only the timer finds the new forum.
    fake.set_operator(forum, announce=False)
    await world.until(lambda: _ready_since(world, count, forum))
    assert world.requests("listServedChannels")[1:]


def _summary(world: World, run_id: UUID) -> str:
    (body,) = [r.json for r in world.requests("finishRun") if str(run_id) in r.path]
    return str(body.get("summary", ""))


async def test_new_operator_of_a_forum_without_binding_uses_the_default(world: World) -> None:
    """RUNNER-NO-GIT: a plain forum that gets the runner as operator is a Claude Code chat."""
    fake = world.fake
    forum = fake.add_forum("Ideas", None, served=False)
    await world.start(world.config(default_workspace="."))
    count = len(world.requests("listServedChannels"))
    fake.set_operator(forum)
    await world.until(lambda: len(world.requests("listServedChannels")) > count)
    run = await fake.trigger("Hello", channel_id=forum)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    assert turn["cwd"] == str(world.projects.resolve())


async def test_new_operator_of_a_forum_without_binding_and_default(world: World) -> None:
    fake = world.fake
    forum = fake.add_forum("Ideas", None, served=False)
    await world.start()
    count = len(world.requests("listServedChannels"))
    fake.set_operator(forum)
    await world.until(lambda: len(world.requests("listServedChannels")) > count)
    run = await fake.trigger("Hello", channel_id=forum)
    assert await world.ended(run.run_id) == "failed"
    assert "Bind a folder" in _summary(world, run.run_id)


async def test_binding_to_a_folder_without_git_is_ready_at_once(world: World) -> None:
    """RUNNER-NO-GIT through the live reload of #459: no restart, no Git."""
    fake = world.fake
    (world.projects / "plain").mkdir()
    await world.start()
    count = len(fake.heartbeats)
    fake.update_forum(world.werkzeuge, workspace_ref="plain")
    await world.until(lambda: _ready_since(world, count, world.werkzeuge))
    run = await fake.trigger("Hello", channel_id=world.werkzeuge)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    assert turn["cwd"] == str((world.projects / "plain").resolve())


async def test_unbinding_falls_back_to_the_default_workspace(world: World) -> None:
    fake = world.fake
    await world.start(world.config(default_workspace="."))
    count = len(fake.heartbeats)
    fake.unbind_forum(world.forum_a)
    await world.until(lambda: _gone_since(world, count, world.forum_a))
    run = await fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    assert turn["cwd"] == str(world.projects.resolve())
