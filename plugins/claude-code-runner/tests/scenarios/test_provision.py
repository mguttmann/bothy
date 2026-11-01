# SPDX-License-Identifier: Apache-2.0
"""A binding with provision: the runner creates the folder and reports it (09 4.8, 18.12, M4-43)."""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

from .conftest import World

pytestmark = pytest.mark.anyio

_FAKE_GIT = """#!{python}
import pathlib, sys
if sys.argv[1] == "clone":
    (pathlib.Path(sys.argv[-1]) / ".git").mkdir(parents=True)
else:
    pathlib.Path(".git").mkdir()
"""


def fake_git(world: World) -> Path:
    folder = world.root / "fakegit"
    folder.mkdir()
    git = folder / "git"
    git.write_text(_FAKE_GIT.format(python=sys.executable))
    git.chmod(0o755)
    return git


def readiness_of(beat: dict[str, object], channel: str) -> dict[str, object] | None:
    entries = beat.get("project_readiness")
    assert isinstance(entries, list)
    for entry in entries:
        if entry["channel_id"] == channel:
            return {k: v for k, v in entry.items() if k not in ("channel_id", "trash_check")}
    return None


async def test_created_folder_is_reported_ready(world: World) -> None:
    git = fake_git(world)
    await world.start(world.config(git_bin=str(git)))
    fake = world.fake
    forum = str(world.forum_b)
    count = len(fake.heartbeats)
    fake.update_forum(
        world.forum_b,
        workspace_ref="newapp",
        provision="clone",
        repository_url="https://git.example.com/team/newapp.git",
    )
    await world.until(
        lambda: any(readiness_of(b, forum) == {"ready": True} for b in fake.heartbeats[count:])
    )
    assert (world.projects / "newapp" / ".git").is_dir()
    seen = [readiness_of(b, forum) for b in fake.heartbeats[count:]]
    assert {"ready": False, "reason": "provisioning"} in seen


async def test_existing_folder_reports_workspace_exists(world: World) -> None:
    git = fake_git(world)
    (world.projects / "taken").mkdir()
    await world.start(world.config(git_bin=str(git)))
    fake = world.fake
    forum = str(world.forum_b)
    count = len(fake.heartbeats)
    fake.update_forum(world.forum_b, workspace_ref="taken", provision="init")
    await world.until(
        lambda: any(
            readiness_of(b, forum) == {"ready": False, "reason": "workspace_exists"}
            for b in fake.heartbeats[count:]
        )
    )
    assert list((world.projects / "taken").iterdir()) == []
