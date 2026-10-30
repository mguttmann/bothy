# SPDX-License-Identifier: Apache-2.0
"""Every forum the runner operates is a Claude Code chat (RUNNER-NO-GIT, 09 18.1).

A bound folder without Git is ready, a forum without a binding works in
``runner.default_workspace`` when the operator set one, and every refusal names its cause in
the ``summary`` of ``finishRun``, which the app shows with the reason in the thread.
"""

from __future__ import annotations

from uuid import UUID

import pytest

from .conftest import World
from .test_live_reload import readiness_of

pytestmark = pytest.mark.anyio


def summary_of(world: World, run_id: UUID) -> str:
    (body,) = [r.json for r in world.requests("finishRun") if str(run_id) in r.path]
    return str(body.get("summary", ""))


def _ready(world: World, channel: UUID) -> bool:
    return any(readiness_of(b, channel) == {"ready": True} for b in world.fake.heartbeats)


async def test_bound_folder_without_git_is_ready_and_runs(world: World) -> None:
    fake = world.fake
    (world.projects / "notes").mkdir()
    forum = fake.add_forum("Notes", "notes")
    await world.start()
    await world.until(lambda: _ready(world, forum))
    run = await fake.trigger("Summarize the notes", channel_id=forum)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    assert turn["cwd"] == str((world.projects / "notes").resolve())
    assert [m for m in fake.messages if m["run_id"] == str(run.run_id)]


async def test_unbound_forum_runs_in_the_default_workspace(world: World) -> None:
    fake = world.fake
    await world.start(world.config(default_workspace="."))
    run = await fake.trigger("Hello", channel_id=world.werkzeuge)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    assert turn["cwd"] == str(world.projects.resolve())
    # A forum without a binding has no readiness entry: there is no binding to make ready.
    assert all(readiness_of(b, world.werkzeuge) is None for b in fake.heartbeats)


async def test_unbound_forum_uses_a_named_default_folder(world: World) -> None:
    (world.projects / "scratch").mkdir()
    await world.start(world.config(default_workspace="scratch"))
    run = await world.fake.trigger("Hello", channel_id=world.werkzeuge)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    assert turn["cwd"] == str((world.projects / "scratch").resolve())


async def test_bound_forum_keeps_its_own_folder_with_a_default(world: World) -> None:
    await world.start(world.config(default_workspace="."))
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    assert turn["cwd"] == str((world.projects / "website").resolve())
    # A bound forum whose folder is missing is not ready; the default never replaces it.
    missing = await world.fake.trigger("Hello", channel_id=world.forum_b)
    assert await world.ended(missing.run_id) == "failed"
    assert "does not exist" in summary_of(world, missing.run_id)
    assert len(world.turns()) == 1


async def test_unbound_forum_without_default_names_what_to_do(world: World) -> None:
    fake = world.fake
    await world.start()
    run = await fake.trigger("Hello", channel_id=world.werkzeuge)
    assert await world.ended(run.run_id) == "failed"
    assert fake.run_state(run.run_id).state_reason == "adapter_error"
    text = summary_of(world, run.run_id)
    assert "no project folder" in text
    assert "Bind a folder" in text
    assert "runner.default_workspace" in text
    assert world.turns() == []


async def test_missing_default_folder_names_the_key(world: World) -> None:
    await world.start(world.config(default_workspace="scratch"))
    run = await world.fake.trigger("Hello", channel_id=world.werkzeuge)
    assert await world.ended(run.run_id) == "failed"
    text = summary_of(world, run.run_id)
    assert "runner.default_workspace" in text
    assert "`workspace_missing`" in text
    assert world.turns() == []


async def test_every_refusal_of_prepare_names_its_cause(world: World) -> None:
    fake = world.fake
    await world.start()
    interaction = await fake.trigger(
        "Approve", channel_id=world.forum_a, trigger_kind="interaction"
    )
    assert await world.ended(interaction.run_id) == "failed"
    assert "`trigger_kind_unsupported`" in summary_of(world, interaction.run_id)
    missing = await fake.trigger("Hello", channel_id=world.forum_b)
    assert await world.ended(missing.run_id) == "failed"
    assert "`workspace_missing`" in summary_of(world, missing.run_id)
