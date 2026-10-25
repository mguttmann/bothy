# SPDX-License-Identifier: Apache-2.0
"""A crash of the runner during the upload of an answer's attachments (brief M3-12 note 10 b).

The first runner runs as a process of its own and is killed with SIGKILL while the second part
of the first file is held by the FakeServer; a second runner on the same ``runner.state_dir``
adopts the turn, repeats ``createUpload`` with the stored key (the FakeServer answers the
stored ``201``), reads ``getUpload``, sends only the parts the server does not have, completes
and sends one ``createMessage``. The FakeServer clock follows the real time here, because the
first runner uses the system clock.
"""

from __future__ import annotations

import signal

import pytest
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.faults import Fault, HoldGate

from .conftest import World
from .test_reply_attachments import PART, REPORT, SUMMARY, _limits, _project, _write

pytestmark = pytest.mark.anyio


@pytest.fixture
def world_real_time() -> bool:
    return True


@pytest.fixture
def world_timings() -> dict[str, float]:
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


@pytest.mark.parametrize("held", ["createUpload", "uploadPart"])
async def test_crash_during_upload_resumes_once(world: World, held: str) -> None:
    _limits(world)
    _write(_project(world), {"a.md": REPORT, "b.md": SUMMARY})
    world.scenario(fixture="plain", result_override={"result": "Two.\nATTACH: a.md\nATTACH: b.md"})
    gate = HoldGate()
    if held == "uploadPart":
        world.fake.inject("uploadPart", Fault.passthrough())  # part 1 is confirmed
    world.fake.inject(held, Fault.hold(gate))
    first = world.spawn()
    await world.until(
        lambda: any(h.get("details", {}).get("adapter") for h in world.fake.heartbeats)
    )
    run = await world.fake.trigger("Send both", channel_id=world.forum_a)
    await gate.reached.wait()
    first.send_signal(signal.SIGKILL)
    first.wait(PATIENCE)
    gate.open()  # the held request is stored after the kill; its answer never arrived
    await world.start()
    assert await world.ended(run.run_id) == "completed"

    creates = world.requests("createUpload")
    sessions = list(world.fake.uploads.sessions.values())
    assert len(sessions) == 2  # one upload session per announced file
    assert [s.content() for s in sessions] == [REPORT, SUMMARY]
    if held == "createUpload":
        # No file_id was stored: createUpload again with the stored key, answered from the
        # stored 201 (06 7).
        assert [r.json["name"] for r in creates] == ["a.md", "a.md", "b.md"]
        assert creates[0].headers["idempotency-key"] == creates[1].headers["idempotency-key"]
        assert world.requests("getUpload") == []
    else:
        # The file_id was stored: getUpload, then only the parts the server does not have.
        assert [r.json["name"] for r in creates] == ["a.md", "b.md"]
        assert len(world.requests("getUpload")) == 1
        parts = [
            r.path.rsplit("/", 1)[-1]
            for r in world.requests("uploadPart")
            if str(sessions[0].file_id) in r.path
        ]
        assert parts.count("1") == 1
        assert len(REPORT) == 3 * PART
    (message,) = world.fake.messages
    assert message["attachment_file_ids"] == [str(s.file_id) for s in sessions]
    assert len(world.requests("finishRun")) == 1
    assert not (world.state / "outgoing" / str(run.run_id)).exists()
    assert not list((world.state / "sessions").rglob(f"{run.run_id}.attach-*.json"))
