# SPDX-License-Identifier: Apache-2.0
"""A stop and a new run while the thread's Hermes session is busy (FIX-hermes-busy-session).

With ``personal`` Hermes keeps an allow list of its own for the platform (09 17.7 layer 3). The
adapter's control sender sends ``/stop`` for a stopped run (17.3); a list without it refuses
the stop, so Hermes' turn goes on while the run is ``cancelled``. A run started on the thread
while that turn still runs (a model request in a long retry, held here by a latch) must not end
``failed`` at once: its turn starts when the session is free (17.6).
"""

from __future__ import annotations

import asyncio
import json
from typing import Any

import pytest
from fake_hermes_kit import HermesRig, HermesWorld, Profile, _start_gateway, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

from bothy_hermes_adapter.installer import CONTROL_SENDER

pytestmark = pytest.mark.anyio

LATE = "Late answer of the stopped turn."
TODO = {
    "name": "todo_list",
    "arguments": {"todos": [{"content": "Migrate", "id": "1", "status": "pending"}]},
}
REFUSED = "no run is open in this chat"
NEXT = "Going on now."


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _script() -> list[ScriptTurn]:
    return [
        # The first turn's model request hangs, as a request in Hermes' rate-limit retry does.
        ScriptTurn(text=(LATE,), hold="retry", paced=True),
        ScriptTurn(text=(NEXT,), paced=True),
    ]


async def _personal(rig: HermesRig, allow_list: str | None = None) -> None:
    """Switch the profile to ``personal`` with Robin as credential holder and start Hermes.

    ``allow_list`` replaces the list the installer wrote (a profile installed before the fix).
    """
    switched = rig.installer(
        "switch-mode",
        "personal",
        "--profile-home",
        str(rig.home),
        "--credential-holder",
        str(rig.fake.robin_id),
    )
    assert switched.returncode == 0, switched.stderr
    if allow_list is not None:
        path = rig.home / ".env"
        lines = [
            line
            for line in path.read_text(encoding="utf-8").splitlines()
            if not line.startswith("BOTHY_ALLOWED_USERS=")
        ]
        lines.append(f"BOTHY_ALLOWED_USERS={allow_list}")
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    rig.fake.world.billing_mode = "personal"
    await _start_gateway(rig)


async def _end(rig: HermesRig, run_id: Any) -> None:
    try:
        await rig.until(lambda: rig.finished(run_id), "the end of the run")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None


def _texts(rig: HermesRig, run_id: Any) -> list[str]:
    return [str(m["content_md"]) for m in rig.fake.messages if m.get("run_id") == str(run_id)]


async def test_stop_reaches_hermes_with_personal_allow_list(world: HermesWorld) -> None:
    """The installer lists the control sender: a stop interrupts the turn, the next run runs."""
    profile = Profile(dotenv={"anthropic_oauth": "oauth"})
    async with hermes_rig(world, _script(), profile=profile, start=False) as rig:
        await _personal(rig)
        env = (rig.home / ".env").read_text(encoding="utf-8")
        listed = next(line for line in env.splitlines() if line.startswith("BOTHY_ALLOWED_USERS="))
        assert listed.split("=", 1)[1].strip("'\"").split(",") == [
            str(rig.fake.robin_id),
            CONTROL_SENDER,
        ]
        first = await rig.fake.trigger("Migrate the host.", author="robin")
        await rig.until(rig.model.requests, "the first model request")
        await asyncio.to_thread(rig.model.wait_held, "retry")
        await rig.fake.cancel(first.run_id)
        await _end(rig, first.run_id)
        assert rig.fake.run_state(first.run_id).state == "cancelled"
        # "resume" continues the thread's session as the server asks for a later message; no
        # ``/new`` goes out before the turn.
        second = await rig.fake.trigger(
            "weiter", author="robin", thread_id=first.thread_id, session_mode="resume"
        )
        await _end(rig, second.run_id)
        log = gateway_log(rig)
        assert f"Unauthorized user: {CONTROL_SENDER}" not in log, log
        assert rig.fake.run_state(second.run_id).state == "completed", log
        assert _texts(rig, second.run_id) == [NEXT]
        rig.model.release("retry")
        await asyncio.sleep(1)
        assert not [m for m in rig.fake.messages if LATE in str(m["content_md"])]


async def test_run_waits_for_a_busy_session_instead_of_failing(world: HermesWorld) -> None:
    """A stop Hermes refused leaves the session busy: the next run waits, it never fails at once.

    The allow list holds only the credential holder, as the installer wrote it before the fix:
    Hermes refuses the control sender's ``/stop``, so the stopped run's turn goes on.
    """
    # The stopped turn's held request asks for a tool: the waiting run never covers it.
    script = [
        ScriptTurn(tool_calls=(TODO,), hold="retry"),
        ScriptTurn(text=(LATE,), paced=True),
        ScriptTurn(text=(NEXT,), paced=True),
    ]
    profile = Profile(dotenv={"anthropic_oauth": "oauth"})
    async with hermes_rig(world, script, profile=profile, start=False) as rig:
        await _personal(rig, allow_list=str(rig.fake.robin_id))
        first = await rig.fake.trigger("Migrate the host.", author="robin")
        await rig.until(rig.model.requests, "the first model request")
        await asyncio.to_thread(rig.model.wait_held, "retry")
        await rig.fake.cancel(first.run_id)
        await _end(rig, first.run_id)
        await rig.until(
            lambda: f"Unauthorized user: {CONTROL_SENDER}" in gateway_log(rig), "the refused stop"
        )
        # "resume" continues the thread's session as the server asks for a later message; no
        # ``/new`` goes out before the turn.
        second = await rig.fake.trigger(
            "weiter", author="robin", thread_id=first.thread_id, session_mode="resume"
        )
        await rig.until(lambda: rig.fake.run_state(second.run_id).state == "running", "running")
        await asyncio.sleep(2)
        view = rig.fake.run_state(second.run_id)
        assert view.state == "running", (view, gateway_log(rig))
        rig.model.release("retry")
        await _end(rig, second.run_id)
        view = rig.fake.run_state(second.run_id)
        assert (view.state, view.state_reason) == ("completed", None), gateway_log(rig)
        # The run's own turn answers; the stopped run's late answer stays dropped
        # (D-HERMES-CRON-COMMANDS-5).
        assert _texts(rig, second.run_id) == [NEXT]
        assert not [m for m in rig.fake.messages if LATE in str(m["content_md"])]
        # The stopped turn's tool call was refused while the run waited (R-AG-44).
        assert any(REFUSED in json.dumps(body) for body in rig.model.raw), gateway_log(rig)
