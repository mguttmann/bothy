# SPDX-License-Identifier: Apache-2.0
"""Moving the project folder of a deleted forum to the trash (09 18.15, FEAT-delete-spaces-chats).

The runner reports a folder check per bound forum in its readiness entries; the server hands a
folder move in the answer of ``sendHeartbeat``; the runner stops a running turn of the forum,
checks again, moves the folder into the trash under the world's home and reports the outcome in
a later heartbeat.
"""

from __future__ import annotations

import sys
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from .conftest import World
from .test_live_reload import readiness_of

pytestmark = pytest.mark.anyio


@pytest.fixture
def world_timings() -> dict[str, float]:
    # The server never ends a run with cancel_timeout before the runner reports it.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def trash_of(world: World) -> Path:
    """The host trash of the world's home on this platform."""
    if sys.platform == "darwin":
        return world.home / ".Trash"
    return world.home / ".local" / "share" / "Trash" / "files"


def checks(world: World, channel: UUID) -> list[Any]:
    out = []
    for beat in world.fake.heartbeats:
        entry = readiness_of(beat, channel)
        if entry is not None and "trash_check" in entry:
            out.append(entry["trash_check"])
    return out


async def test_the_readiness_entries_carry_the_folder_check(world: World) -> None:
    (world.projects / "notes").mkdir()
    notes = world.fake.add_forum("Notes", "notes")
    await world.start(world.config(trash_check_interval="200ms"))
    await world.until(lambda: {"ok": True} in checks(world, notes))
    await world.until(
        lambda: {"ok": False, "reason": "workspace_missing"} in checks(world, world.forum_b)
    )
    # The website folder of Projekt-A has a `.git` that Git cannot read: never moved.
    await world.until(
        lambda: {"ok": False, "reason": "check_failed"} in checks(world, world.forum_a)
    )


async def test_a_requested_move_goes_to_the_trash_and_is_reported(world: World) -> None:
    folder = world.projects / "notes"
    folder.mkdir()
    (folder / "todo.txt").write_text("keep me")
    notes = world.fake.add_forum("Notes", "notes")
    await world.start(world.config(trash_check_interval="200ms"))
    world.fake.request_project_trash(notes, "notes")
    await world.until(lambda: world.fake.project_trash_results != [])
    assert world.fake.project_trash_results == [{"channel_id": str(notes), "result": "trashed"}]
    assert not folder.exists()
    assert (trash_of(world) / "notes" / "todo.txt").read_text() == "keep me"
    # A later answer without the request repeats nothing.
    count = len(world.fake.heartbeats)
    await world.until(lambda: len(world.fake.heartbeats) > count + 2)
    assert all("project_trash_results" not in b for b in world.fake.heartbeats[count + 1 :])


async def test_a_refused_folder_stays_and_the_code_is_reported(world: World) -> None:
    website = world.projects / "website"
    await world.start(world.config(trash_check_interval="200ms"))
    world.fake.request_project_trash(world.forum_a, "website")
    await world.until(lambda: world.fake.project_trash_results != [])
    assert world.fake.project_trash_results == [
        {"channel_id": str(world.forum_a), "result": "refused", "reason": "check_failed"}
    ]
    assert (website / ".git").is_dir()
    assert not trash_of(world).exists() or not (trash_of(world) / "website").exists()


async def test_a_running_turn_is_stopped_before_the_move(world: World) -> None:
    folder = world.projects / "notes"
    folder.mkdir()
    notes = world.fake.add_forum("Notes", "notes")
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate), echo="never")
    await world.start(world.config(trash_check_interval="200ms"))
    run = await world.fake.trigger("Write the notes", channel_id=notes)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    world.fake.request_project_trash(notes, "notes")
    assert await world.ended(run.run_id) == "cancelled"
    await world.until(lambda: world.fake.project_trash_results != [])
    assert world.fake.project_trash_results == [{"channel_id": str(notes), "result": "trashed"}]
    assert not folder.exists()
    (finish,) = [r.json for r in world.requests("finishRun") if str(run.run_id) in r.path]
    assert finish["reason"] == "channel_read_only"
    # The turn ran in the folder before it moved, and nothing ran after it.
    (turn,) = world.turns()
    assert turn["cwd"] == str(folder.resolve())


async def test_a_restart_does_not_repeat_the_move(world: World) -> None:
    folder = world.projects / "notes"
    folder.mkdir()
    notes = world.fake.add_forum("Notes", "notes")
    runner = await world.start(world.config(trash_check_interval="200ms"))
    world.fake.request_project_trash(notes, "notes")
    await world.until(lambda: world.fake.project_trash_results != [])
    await world.stop(runner)
    # The server asks again (its store was lost); a new folder of the same name stays.
    folder.mkdir()
    (folder / "new.txt").write_text("new")
    world.fake.request_project_trash(notes, "notes")
    await world.start(world.config(trash_check_interval="200ms"))
    await world.until(lambda: len(world.fake.project_trash_results) == 2)
    assert world.fake.project_trash_results[1] == {"channel_id": str(notes), "result": "trashed"}
    assert (folder / "new.txt").read_text() == "new"
