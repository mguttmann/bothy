# SPDX-License-Identifier: Apache-2.0
"""The session mirror against the pinned Hermes (09 17.13).

A ``hermes -z`` session is a session of the platform ``cli``; the pinned CLI attaches to the
profile's gateway and runs its turns there, so the hooks of a foreign platform reach the plugin
next to the adapter's own platform. With the mirror on for ``cli`` the turns show as one post in
the mirror forum, also when a second command resumes the session.
"""

from __future__ import annotations

import asyncio
import json
import subprocess
from typing import Any

import pytest
import yaml
from fake_hermes_kit import (
    PLATFORM_KEY,
    HermesRig,
    HermesWorld,
    _start_gateway,
    gateway_log,
    hermes_rig,
)
from hermes_harness.fake_model import ScriptTurn

pytestmark = pytest.mark.anyio

TODO = {
    "name": "todo_list",
    "arguments": {"todos": [{"content": "Book", "id": "1", "status": "pending"}]},
}


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


async def _hermes(rig: HermesRig, *args: str) -> subprocess.CompletedProcess[str]:
    """Run the profile's ``hermes`` command line in a thread (the FakeServer answers meanwhile)."""
    return await asyncio.to_thread(
        subprocess.run,
        [str(rig.world.install.executable), *args],
        env=rig.env,
        cwd=rig.homes.work,
        capture_output=True,
        text=True,
        check=False,
    )


def _texts(rig: HermesRig, thread: Any) -> list[str]:
    return [
        m["content_md"] for m in rig.fake.world.messages.values() if m["thread_id"] == str(thread)
    ]


def _texts_of_first(rig: HermesRig) -> list[str]:
    threads = rig.fake.created_threads
    return _texts(rig, threads[0]) if threads else []


async def test_a_cli_session_and_its_resume_show_as_one_mirror_post(world: HermesWorld) -> None:
    script = [
        ScriptTurn(tool_calls=(TODO,)),
        ScriptTurn(text=("The trip is planned.",)),
        ScriptTurn(text=("Booked.",)),
    ]
    async with hermes_rig(world, script, start=False) as rig:
        fake = rig.fake
        forum = fake.add_forum("mirror", None)
        path = rig.home / "config.yaml"
        config = yaml.safe_load(path.read_text(encoding="utf-8"))
        extra = config["gateway"]["platforms"][PLATFORM_KEY]["extra"]
        extra["hermes_adapter.mirror_channel"] = str(forum)
        extra["hermes_adapter.mirror_platforms"] = ["cli"]
        path.write_text(yaml.safe_dump(config, sort_keys=True), encoding="utf-8")
        # The pinned CLI runs its turns in the profile's gateway process (it attaches to it).
        await _start_gateway(rig)
        usage = rig.homes.work / "usage.json"
        first = await _hermes(rig, "-z", "Plan the trip to Rome", "--usage-file", str(usage))
        assert first.returncode == 0, first.stderr[-3000:]
        session = json.loads(usage.read_text(encoding="utf-8"))["session_id"]
        try:
            # The mirror's worker posts after the turn, on a thread of its own.
            await rig.until(lambda: len(_texts_of_first(rig)) == 2, "the first turn's post")
        except AssertionError:
            raise AssertionError(first.stderr[-3000:] + gateway_log(rig)) from None
        assert len(fake.created_threads) == 1
        post = fake.created_threads[0]
        assert fake.thread(post)["channel_id"] == str(forum)
        assert fake.thread(post)["title"] == "cli: Plan the trip to Rome"
        second = await _hermes(rig, "-r", session, "-z", "Book it")
        assert second.returncode == 0, second.stderr[-3000:]
        await rig.until(lambda: len(_texts(rig, post)) == 4, "the resumed turn in the post")
        assert len(fake.created_threads) == 1
        texts = _texts(rig, post)
        assert texts[0].startswith("**Request** (cli session)\n\n> Plan the trip to Rome")
        assert texts[1] == "The trip is planned.\n\nTools: todo_list x1"
        assert texts[2].startswith("**Request** (cli session)\n\n> Book it")
        assert texts[3] == "Booked."
        assert len(texts) == 4
        assert all("run_id" not in m for m in fake.world.messages.values())
        agents = [q.headers.get("user-agent", "") for q in rig.requests("createThread")]
        assert agents[0].startswith("hermes-adapter-mirror/")
