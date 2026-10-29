# SPDX-License-Identifier: Apache-2.0
"""The runner reports its project folders in the heartbeat (09 4.8, 18.13, M4-44)."""

from __future__ import annotations

import pytest

from .conftest import World

pytestmark = pytest.mark.anyio


async def test_heartbeat_reports_the_git_folders(world: World) -> None:
    (world.projects / "api" / ".git").mkdir(parents=True)
    (world.projects / "notes").mkdir()
    (world.projects / ".cache" / ".git").mkdir(parents=True)
    await world.start(world.config())
    fake = world.fake
    await world.until(lambda: any("project_folders" in b for b in fake.heartbeats))
    beat = next(b for b in fake.heartbeats if "project_folders" in b)
    assert beat["project_folders"] == ["api", "website"]
    count = len(fake.heartbeats)
    (world.projects / "newsite" / ".git").mkdir(parents=True)
    fake.update_forum(world.forum_b, workspace_ref="docs")
    await world.until(
        lambda: any(
            b.get("project_folders") == ["api", "newsite", "website"]
            for b in fake.heartbeats[count:]
        )
    )
