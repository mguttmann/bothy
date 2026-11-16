# SPDX-License-Identifier: Apache-2.0
"""Background mode of the runner against the real server of this tree (09 18.2 "Background mode").

FEAT-RUNNER-BG-SESSIONS: the runner's own code with the background side of Fake-Claude; the
server is sdkserver. One forum in background mode: a post, a follow-up into the running turn
and a stop.

The file starts a server of its own. Background mode runs only ``personal`` turns, and I1 (07
SR1) allows ``personal`` only while the credential holder is the only principal with ``trigger``
on what the agent operates. So the world of 07 "Examples" is changed here before the first
flow: the owner revokes every grant of a person on the space Support and switches Lotse to
``personal`` with themselves as the credential holder. On the server the other files share,
that would refuse every flow Alex triggers.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import shutil
import tempfile
import time
from collections.abc import AsyncIterator, Iterator
from datetime import datetime
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.ids import uuid7
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.sdkserver import ApiAnswer, RealServer, start_sdkserver

from .conftest import RealWorld, helper

pytestmark = [pytest.mark.anyio, pytest.mark.e2e]

ENDED = ("completed", "failed", "cancelled")
BACKGROUND = {
    "launch_mode": "background",
    "allow_personal": True,
    "background_status_interval": "50ms",
}


def _ok(answer: ApiAnswer, *statuses: int) -> Any:
    assert answer.status in (statuses or (200, 201)), f"status {answer.status}: {answer.body}"
    return answer.body


async def _until(what: str, check: Any) -> Any:
    deadline = time.monotonic() + PATIENCE
    while True:
        value = await check()
        if value:
            return value
        if time.monotonic() > deadline:
            raise AssertionError(f"not within {PATIENCE} s: {what}")
        await asyncio.sleep(0.1)


async def _personal(real: RealServer) -> None:
    """Lotse becomes ``personal`` with the owner as the credential holder (I1, I2)."""
    space = {"target_kind": "space", "target_id": real.world.space_id}
    grants = _ok(await real.call("listGrants", as_="owner", query=space))
    for grant in grants["items"]:
        if grant.get("subject_kind") == "agent":
            continue  # Lotse's own grant on the space
        body = {"reason": "background mode needs a personal agent"}
        _ok(await real.call("revokeGrant", as_="owner", path={"grant_id": grant["id"]}, body=body))
    agent = await real.call("getAgent", as_="owner", path={"agent_id": real.agent_id})
    version = _ok(agent)["version"]
    body = {
        "billing_mode": "personal",
        "credential_holder_id": str(real.world.owner_id),
        "reason": "background mode runs personal turns only",
    }
    switched = await real.call(
        "setAgentBillingMode",
        as_="owner",
        path={"agent_id": real.agent_id},
        body=body,
        header={"If-Match": f'"{version}"'},
    )
    assert _ok(switched)["billing_mode"] == "personal"


@pytest.fixture(scope="module")
async def bg_real(sdkserver_binary: Path) -> AsyncIterator[RealServer]:
    workdir = Path(tempfile.mkdtemp(prefix="bothy-runner-bg-sdkserver-"))
    workdir.chmod(0o700)  # the token files live here
    log = os.environ.get("BOTHY_E2E_SERVER_LOG")
    server = await start_sdkserver(
        sdkserver_binary,
        workdir,
        config={"file.scan.mode": "off"},
        log=Path(log) if log else None,
    )
    try:
        await _personal(server)
        yield server
    finally:
        await server.stop()
        shutil.rmtree(workdir, ignore_errors=True)


@pytest.fixture
def short_dir() -> Iterator[Path]:
    """A short folder for the state directory: the inbox socket of a session lies under it."""
    base = os.environ.get("CLAUDE_CODE_RUNNER_SHORT_TMP") or "/tmp"
    path = Path(tempfile.mkdtemp(prefix="ccr", dir=base))
    try:
        yield path
    finally:
        shutil.rmtree(path, ignore_errors=True)


def _alive(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
    except (ProcessLookupError, PermissionError):
        return False
    return True


@pytest.fixture
async def world(bg_real: RealServer, tmp_path: Path, short_dir: Path) -> AsyncIterator[RealWorld]:
    root = tmp_path / "w"
    root.mkdir(mode=0o700)
    projects = root / "projects"
    (projects / "website" / ".git").mkdir(parents=True)
    home = root / "home"
    (home / ".claude").mkdir(parents=True)
    # The subscription login of the runner user, which a personal turn uses.
    (home / ".claude" / "fake-login").write_text("claude.ai")
    bin_dir = root / "bin"
    capture = root / "capture.jsonl"
    claude = helper.install(bin_dir, {"fixture": "happy"}, capture)
    w = RealWorld(bg_real, root, projects, home, bin_dir, capture, claude)
    try:
        yield w
    finally:
        for runner, task in w.runners:
            await runner.stop()
            with contextlib.suppress(Exception):
                await asyncio.wait_for(task, PATIENCE)
        # No session and no supervisor of Fake-Claude is left running.
        groups = {c["pgid"] for c in w.calls("bg_player")}
        groups |= {c["pgid"] for c in w.calls("bg_supervisor")}
        deadline = time.monotonic() + PATIENCE
        while any(_alive(g) for g in groups) and time.monotonic() < deadline:
            await asyncio.sleep(0.05)
        left = [g for g in groups if _alive(g)]
        assert left == [], f"process groups of background sessions are left: {left}"


async def _start(world: RealWorld, state: Path) -> None:
    await world.start(world.config(state_dir=str(state / "state"), **BACKGROUND))
    await _heartbeat(world.real)


async def _heartbeat(real: RealServer) -> None:
    """Wait for a heartbeat of the agent after now: it reports the modes the runner can prove."""
    since = time.time()

    async def check() -> bool:
        agent = _ok(await real.call("getAgent", as_="owner", path={"agent_id": real.agent_id}))
        at = (agent.get("presence") or {}).get("last_heartbeat_at")
        if at is None:
            return False
        return datetime.fromisoformat(at.replace("Z", "+00:00")).timestamp() > since

    await _until("a heartbeat of the agent", check)


async def _forum_post(real: RealServer, text: str, number: int) -> UUID:
    """The owner, the only person who may trigger a personal agent, posts in a new forum."""
    forum = await real.create_forum(f"Background {number}")
    body = {"title": "Login", "first_message": {"client_msg_id": str(uuid7()), "content_md": text}}
    created = await real.call("createThread", as_="owner", path={"channel_id": forum}, body=body)
    return UUID(_ok(created, 201)["first_message"]["id"])


async def _ended(real: RealServer, run_id: UUID) -> Any:
    async def check() -> Any:
        view = await real.run_state(run_id)
        return view if view.state in ENDED else None

    return await _until(f"the end of run {run_id}", check)


async def _agent_messages(real: RealServer, thread_id: UUID) -> list[dict[str, Any]]:
    page = _ok(await real.call("listThreadMessages", as_="owner", path={"thread_id": thread_id}))
    items = sorted(page["items"], key=lambda m: int(m.get("thread_seq") or 0))
    return [
        m
        for m in items
        if (m.get("author") or {}).get("id") == str(real.agent_id) and m.get("kind") != "run"
    ]


def _entries(world: RealWorld) -> dict[str, Any]:
    """The session list of Fake-Claude: the entries the runner did not remove."""
    path = world.home / ".claude" / "fake-bg" / "state.json"
    sessions: dict[str, Any] = json.loads(path.read_text(encoding="utf-8"))["sessions"]
    return sessions


async def test_a_forum_post_runs_as_a_background_session(world: RealWorld, short_dir: Path) -> None:
    real = world.real
    await _start(world, short_dir)
    message = await _forum_post(real, "Please fix the login form", 1)
    run_id, thread_id = await real.wait_run(message)
    view = await _ended(real, run_id)
    assert (view.state, view.state_reason) == ("completed", None)
    answers = await _agent_messages(real, thread_id)
    assert len(answers) == 1
    assert answers[0]["content_md"].strip()
    # The turn ran as one background session and no print turn was started.
    [launch] = world.calls("bg_launch")
    assert "--bg" in launch["argv"]
    assert world.turns() == []
    # The thread's session on the server is the one the CLI made, and it is reported started.
    [player] = world.calls("bg_player")
    listed = await real.call("listAgentSessions", as_="owner", path={"thread_id": thread_id})
    [current] = [s for s in _ok(listed)["items"] if s.get("is_current")]
    assert current["external_session_ref"] == player["session_id"]
    assert current["started"] is True
    # The run left no entry in the session list behind.
    assert _entries(world) == {}


async def test_a_followup_reaches_the_running_background_turn(
    world: RealWorld, short_dir: Path
) -> None:
    real = world.real
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), gate_after_lines=1)
    await _start(world, short_dir)
    message = await _forum_post(real, "Fix the form", 2)
    run_id, thread_id = await real.wait_run(message)

    async def waiting() -> bool:
        return len(world.calls("gate_wait")) == 1

    await _until("the tool call of the turn at its gate", waiting)
    body = {"client_msg_id": str(uuid7()), "content_md": "Also check the footer"}
    posted = await real.call("createMessage", as_="owner", path={"thread_id": thread_id}, body=body)
    followup = UUID(_ok(posted, 201)["id"])

    async def written() -> bool:
        return len(world.calls("bg_socket")) == 1

    await _until("the follow-up on the session's inbox socket", written)
    assert "Also check the footer" in world.calls("bg_socket")[0]["text"]
    gate.touch()
    assert (await _ended(real, run_id)).state == "completed"
    got = _ok(await real.call("getMessage", as_="owner", path={"message_id": followup}))
    assert got["trigger_state"] == "followup_delivered"
    assert len(await _agent_messages(real, thread_id)) == 1
    assert len(world.calls("bg_launch")) == 1


async def test_a_cancel_stops_the_background_session(world: RealWorld, short_dir: Path) -> None:
    real = world.real
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), gate_after_lines=1)
    await _start(world, short_dir)
    message = await _forum_post(real, "Take your time", 3)
    run_id, thread_id = await real.wait_run(message)

    async def waiting() -> bool:
        return len(world.calls("gate_wait")) == 1

    await _until("the tool call of the turn at its gate", waiting)
    cancelled = await real.call(
        "cancelRun", as_="owner", path={"run_id": run_id}, body={"reason": "e2e"}
    )
    _ok(cancelled, 202)
    view = await _ended(real, run_id)
    assert view.state == "cancelled"
    assert await _agent_messages(real, thread_id) == []
    # The stop was `claude stop`, and the session's process and entry are gone.
    assert len(world.calls("bg_stop")) >= 1
    [player] = world.calls("bg_player")

    async def gone() -> bool:
        return not _alive(player["pgid"]) and _entries(world) == {}

    await _until("the end of the session's process and its entry", gone)
