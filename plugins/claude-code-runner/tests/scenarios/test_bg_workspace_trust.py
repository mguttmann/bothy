# SPDX-License-Identifier: Apache-2.0
"""A newly bound project folder runs in background mode without a step on the host.

FIX-RUNNER-NEW-PROJECT-TRUST: a new forum was bound to a new folder and every
run failed with ``background_workspace_untrusted``, because the CLI's global configuration had
no trust entry for it. Before a launch in a bound folder below ``runner.projects_root`` the
runner records that entry itself (ruling D-FIX-RUNNER-NEW-PROJECT-TRUST-1); Fake-Claude refuses
like the pinned CLI while the entry is missing (``bg_trust_check``).
"""

from __future__ import annotations

import json
import os
from pathlib import Path

import pytest

from claude_code_runner.causes import cause

from .conftest import LogLines, World
from .test_bg_sessions import events, launches, players, start

pytestmark = pytest.mark.anyio

_OTHER = {
    "numStartups": 3,
    "oauthAccount": {"emailAddress": "alex@example.org"},
    "projects": {"/elsewhere": {"hasTrustDialogAccepted": False, "allowedTools": ["Read"]}},
}


def config_file(world: World) -> Path:
    path = world.home / ".claude.json"
    path.write_text(json.dumps(_OTHER, indent=2))
    path.chmod(0o600)
    return path


def key(world: World, name: str) -> str:
    return os.path.realpath(world.projects / name)


async def test_a_newly_bound_folder_gets_trust_and_runs(bg_world: World, logs: LogLines) -> None:
    world = bg_world
    path = config_file(world)
    world.scenario(fixture="plain", bg_trust_check=True)
    await start(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert len(launches(world)) == len(players(world)) == 1
    document = json.loads(path.read_text())
    assert document["projects"][key(world, "website")] == {"hasTrustDialogAccepted": True}
    # Every other member stays as it was, the untrusted entry of another folder too.
    assert {k: v for k, v in document.items() if k != "projects"} == {
        k: v for k, v in _OTHER.items() if k != "projects"
    }
    assert document["projects"]["/elsewhere"] == _OTHER["projects"]["/elsewhere"]
    assert path.stat().st_mode & 0o777 == 0o600
    assert not Path(f"{path}.lock").exists()
    assert len(events(logs, "workspace_trust_granted")) == 1
    assert events(logs, "workspace_trust_failed") == []
    # The next run finds the entry and writes nothing.
    before = path.read_bytes()
    run = await world.fake.trigger("Fix more", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert path.read_bytes() == before
    assert len(events(logs, "workspace_trust_granted")) == 1


async def test_the_default_workspace_gets_no_trust(bg_world: World, logs: LogLines) -> None:
    world = bg_world
    path = config_file(world)
    before = path.read_bytes()
    world.scenario(fixture="plain", bg_trust_check=True)
    # A forum without a binding runs in runner.default_workspace: the operator's setting, not
    # the owner's binding, so the runner records no trust for it.
    await start(world, default_workspace="website")
    run = await world.fake.trigger("Fix it", channel_id=world.werkzeuge)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run(run.run_id)["summary"] == cause("background_workspace_untrusted")
    assert path.read_bytes() == before
    assert events(logs, "workspace_trust_granted") == []
    assert events(logs, "workspace_trust_failed") == []


async def test_trust_that_cannot_be_written_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    path = config_file(world)
    before = path.read_bytes()
    lock = Path(f"{path}.lock")
    lock.mkdir()  # the CLI holds its lock of the file the whole time
    world.scenario(fixture="plain", bg_trust_check=True)
    await start(world, cli_probe_timeout="1s")
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert world.fake.run(run.run_id)["summary"] == cause("background_workspace_untrusted")
    assert world.turns() == []  # never print mode
    assert players(world) == []
    assert path.read_bytes() == before
    assert lock.is_dir()  # a lock the runner did not take is never removed
    [failed] = events(logs, "workspace_trust_failed")
    assert failed["code"] == "locked"
    assert failed["run_id"] == str(run.run_id)
    assert str(world.projects) not in json.dumps(failed)  # no path of the host in the log
    assert [e["code"] for e in events(logs, "background_unavailable")] == [
        "background_workspace_untrusted"
    ]
