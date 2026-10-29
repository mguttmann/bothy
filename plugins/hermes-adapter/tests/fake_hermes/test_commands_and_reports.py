# SPDX-License-Identifier: Apache-2.0
"""Hermes commands, skill commands and reports against the pinned Hermes (09 10, 14, 17.3, 17.9).

* A control command runs in the thread's Hermes session, idle and during a turn, and Hermes'
  reply becomes a message of the agent without a run; the turn's own answer stays the run's.
* A skill of the profile becomes a model command whose run is Hermes' skill invocation.
* Reports: ``hermes send`` (a process without the gateway, the standalone sender) to the home
  channel (a forum: a new post), to a text channel (its main thread) and to a fixed thread, and
  a cron job of the profile delivered to ``deliver=<platform key>``.
"""

from __future__ import annotations

import asyncio
import subprocess
from datetime import UTC, datetime, timedelta
from typing import Any

import pytest
from fake_hermes_kit import PLATFORM_KEY, HermesRig, HermesWorld, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

pytestmark = pytest.mark.anyio

TODO = {
    "name": "todo_list",
    "arguments": {"todos": [{"content": "Book", "id": "1", "status": "pending"}]},
}
SKILL_MARKER = "DEMO-SKILL-MARKER"
SKILL = f"""---
name: demo-notes
description: Turn notes into a short summary
---
# Demo notes

Follow the {SKILL_MARKER} rule: answer with one sentence.
"""


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _own_messages(rig: HermesRig) -> list[dict[str, Any]]:
    """Messages of the agent without a run, in order."""
    agent = str(rig.fake.agent_id)
    return [
        m
        for m in rig.fake.world.messages.values()
        if "run_id" not in m and m["author"]["id"] == agent
    ]


async def _registered(rig: HermesRig, *names: str) -> dict[str, dict[str, Any]]:
    def found() -> bool:
        have = {c["name"] for c in rig.fake.commands.values()}
        return set(names) <= have

    try:
        await rig.until(found, "the reconciled commands")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None
    return {c["name"]: c for c in rig.fake.commands.values()}


async def _hermes(rig: HermesRig, *args: str) -> subprocess.CompletedProcess[str]:
    """Run the profile's ``hermes`` command line (a process without the gateway).

    In a thread: the FakeServer runs on this event loop and must answer meanwhile.
    """
    return await asyncio.to_thread(
        subprocess.run,
        [str(rig.world.install.executable), *args],
        env=rig.env,
        cwd=rig.homes.work,
        capture_output=True,
        text=True,
        check=False,
    )


async def _python(rig: HermesRig, code: str) -> subprocess.CompletedProcess[str]:
    return await asyncio.to_thread(
        subprocess.run,
        [str(rig.world.python), "-c", code],
        env=rig.env,
        cwd=rig.homes.work,
        capture_output=True,
        text=True,
        check=False,
    )


async def test_control_command_idle_and_during_a_turn(world: HermesWorld) -> None:
    script = [
        ScriptTurn(tool_calls=(TODO,), hold="first"),
        ScriptTurn(text=("The trip ", "is planned."), paced=True),
    ]
    async with hermes_rig(world, script) as rig:
        commands = await _registered(rig, "context", "version")
        assert commands["version"]["control"] is True
        assert not {"stop", "new", "resume", "title", "status", "yolo", "plan"} & set(commands)
        main = rig.fake.main_thread()
        await rig.fake.invoke_command("version", thread_id=main)
        try:
            await rig.until(lambda: _own_messages(rig), "the reply to /version")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        reply = _own_messages(rig)[0]
        assert reply["thread_id"] == str(main)
        assert reply["content_md"].startswith("Hermes Agent"), reply["content_md"]
        assert rig.requests("startRun") == []
        # During a turn: the reply comes inline and stays outside the run, the answer is the run's.
        triggered = await rig.fake.trigger("Plan the trip.")
        await rig.until(rig.model.requests, "the first model request")
        await asyncio.to_thread(rig.model.wait_held, "first")
        await rig.fake.invoke_command("context", run_id=triggered.run_id)
        try:
            await rig.until(lambda: len(_own_messages(rig)) == 2, "the reply to /context")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert _own_messages(rig)[1]["thread_id"] == str(triggered.thread_id)
        rig.model.release("first")
        try:
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert rig.fake.run_state(triggered.run_id).state == "completed", gateway_log(rig)
        answers = [m for m in rig.fake.messages if m.get("run_id") == str(triggered.run_id)]
        assert [m["content_md"] for m in answers] == ["The trip is planned."]


async def test_skill_command_runs_the_skill(world: HermesWorld) -> None:
    from fake_hermes_kit import _start_gateway

    script = [ScriptTurn(text=("One sentence ", "of summary."), paced=True)]
    texts = [SKILL_MARKER, "the meeting notes"]
    async with hermes_rig(world, script, texts=texts, start=False) as rig:
        skill = rig.home / "skills" / "demo-notes" / "SKILL.md"
        skill.parent.mkdir(parents=True, exist_ok=True)
        skill.write_text(SKILL, encoding="utf-8")
        await _start_gateway(rig)
        commands = await _registered(rig, "demo-notes")
        assert commands["demo-notes"]["control"] is False
        assert commands["demo-notes"]["options"][0]["name"] == "input"
        triggered = await rig.fake.trigger(
            "unused", command="demo-notes", options={"input": "the meeting notes"}
        )
        try:
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert rig.fake.run_state(triggered.run_id).state == "completed", gateway_log(rig)
        requests = rig.model_requests(scripted=True)
        assert any(set(texts) <= set(r.get("scenario_texts", [])) for r in requests), (
            "the skill and the input did not reach the model"
        )
        answers = [m for m in rig.fake.messages if m.get("run_id") == str(triggered.run_id)]
        assert [m["content_md"] for m in answers] == ["One sentence of summary."]
        assert "reply_to_message_id" not in answers[0]


async def test_reports_from_hermes_send_and_cron(world: HermesWorld) -> None:
    from fake_hermes_kit import _start_gateway

    async with hermes_rig(world, [], start=False) as rig:
        fake = rig.fake
        forum = fake.add_forum("support", None)
        token = rig.home / "token"
        again = rig.installer(
            "install",
            "--profile-home",
            str(rig.home),
            "--server-url",
            fake.base_url,
            "--instance-id",
            str(fake.instance_id),
            "--token-file",
            str(token),
            "--busy-input-mode",
            "steer",
            "--report-channel",
            str(forum),
        )
        assert again.returncode == 0, again.stderr[-2000:]
        await _start_gateway(rig)
        # The home channel is a forum: one new post per report, titled by its first line.
        sent = await _hermes(rig, "send", "--to", PLATFORM_KEY, "Ticket 9\nThe printer is down.")
        assert sent.returncode == 0, sent.stderr[-3000:] + gateway_log(rig)
        assert len(fake.created_threads) == 1
        post = fake.thread(fake.created_threads[0])
        assert (post["channel_id"], post["title"]) == (str(forum), "Ticket 9")
        # A text channel: its main thread; a fixed thread: that thread.
        allgemein = fake.world.channel_id
        main = fake.main_thread()
        sent = await _hermes(rig, "send", "--to", f"{PLATFORM_KEY}:{allgemein}", "Nightly: green")
        assert sent.returncode == 0, sent.stderr[-3000:]
        sent = await _hermes(
            rig, "send", "--to", f"{PLATFORM_KEY}:{allgemein}:{main}", "Fixed thread"
        )
        assert sent.returncode == 0, sent.stderr[-3000:]
        in_main = [m["content_md"] for m in fake.world.messages.values()]
        assert "Nightly: green" in in_main
        assert "Fixed thread" in in_main
        assert all("run_id" not in m for m in fake.world.messages.values())
        assert len(fake.created_threads) == 1
        # A refused report is a failure for Hermes, nothing is stored.
        refused = await _hermes(rig, "send", "--to", f"{PLATFORM_KEY}:unassigned", "Nothing")
        assert refused.returncode != 0
        # A cron job of the profile (no model: its script is the job) to `deliver=<key>`.
        scripts = rig.home / "scripts"
        scripts.mkdir(exist_ok=True)
        (scripts / "report.py").write_text(
            "print('Disk report\\nAll disks have room.')\n", encoding="utf-8"
        )
        code = (
            "import json\n"
            "from tools.cronjob_tools import cronjob\n"
            "made = json.loads(cronjob(action='create', schedule='every 1h', name='disks',\n"
            f"    no_agent=True, script='report.py', deliver='{PLATFORM_KEY}'))\n"
            "assert made.get('success'), made\n"
            "ran = json.loads(cronjob(action='run', job_id=made['job_id']))\n"
            "print(json.dumps(ran))\n"
        )
        ran = await _python(rig, code)
        assert ran.returncode == 0, ran.stderr[-3000:]
        try:
            await rig.until(lambda: len(fake.created_threads) == 2, "the cron report")
        except AssertionError:
            raise AssertionError(ran.stdout[-2000:] + gateway_log(rig)) from None
        assert _post_texts(fake, 1, forum)
        assert any("All disks have room." in t for t in _post_texts(fake, 1, forum))
        assert _agents(rig)[-1].startswith("hermes-adapter-standalone/")
        # The gateway's own scheduler fires a due job: the live adapter posts it (no standalone).
        due = (datetime.now(UTC) + timedelta(seconds=2)).isoformat()
        code = (
            "import json\n"
            "from tools.cronjob_tools import cronjob\n"
            f"made = json.loads(cronjob(action='create', schedule='{due}', name='late',\n"
            f"    no_agent=True, script='report.py', deliver='{PLATFORM_KEY}'))\n"
            "assert made.get('success'), made\n"
        )
        made = await _python(rig, code)
        assert made.returncode == 0, made.stderr[-3000:]
        try:
            await rig.until(lambda: len(fake.created_threads) == 3, "the scheduler's report")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert any("All disks have room." in t for t in _post_texts(fake, 2, forum))
        assert _agents(rig)[-1].startswith("hermes-adapter/"), gateway_log(rig)
        assert "falling back to standalone" not in gateway_log(rig)


def _post_texts(fake: Any, index: int, forum: Any) -> list[str]:
    """The messages of the agent's ``index``-th forum post, after checking its forum."""
    thread = fake.created_threads[index]
    assert fake.thread(thread)["channel_id"] == str(forum)
    return [m["content_md"] for m in fake.world.messages.values() if m["thread_id"] == str(thread)]


def _agents(rig: HermesRig) -> list[str]:
    """The ``User-Agent`` of every ``createThread``, in order."""
    return [q.headers.get("user-agent", "") for q in rig.requests("createThread")]
