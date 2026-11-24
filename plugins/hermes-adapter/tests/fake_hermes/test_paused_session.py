# SPDX-License-Identifier: Apache-2.0
"""A Hermes session paused after a crash, and an owner's session command.

FIX-hermes-session-commands.

Hermes fences a turn that was running when its process died (a reboot, a power loss) as
``unknown`` and pauses the session's queue at its next start: it answers the first message with a
notice that names ``/discard`` and ``/reset`` and admits every later one silently. The adapter
passes a person's text with ``allow_gateway_control`` off (09 17.4), so ``/discard`` typed in the
app never reached Hermes as a command, and each run after the first ended ``completed`` with no
answer (24.11., production).

* A run on a paused session never ends without an answer (09 17.6).
* A message of an active owner whose text is exactly a session command reaches Hermes as that
  command from the adapter's control sender, and Hermes' reply is the run's answer (09 17.3).
* The same text from a run the server does not prove to be an owner's stays plain text (covered
  by ``tests/unit/test_session_commands.py``).
"""

from __future__ import annotations

import asyncio
import contextlib
import os
import signal
from typing import Any

import pytest
from fake_hermes_kit import HermesRig, HermesWorld, _start_gateway, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

from bothy_hermes_adapter.hermes_commands import COMMAND_FAILED

pytestmark = pytest.mark.anyio

LOST = "Answer of the lost turn."
QUEUED = "Answer of a saved message."
NEXT = "Going on now."
PAUSED = "This conversation is paused"


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _script() -> list[ScriptTurn]:
    # The first turn's model request hangs until Hermes dies; the saved messages Hermes runs
    # after /discard and the next run's turn answer from the rest.
    return [
        ScriptTurn(text=(LOST,), hold="lost"),
        *[ScriptTurn(text=(QUEUED,)) for _ in range(4)],
        ScriptTurn(text=(NEXT,)),
    ]


async def _end(rig: HermesRig, run_id: Any) -> None:
    try:
        await rig.until(lambda: rig.finished(run_id), "the end of the run")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None


def _texts(rig: HermesRig, run_id: Any) -> list[str]:
    return [str(m["content_md"]) for m in rig.fake.messages if m.get("run_id") == str(run_id)]


async def _paused(rig: HermesRig) -> Any:
    """A thread whose Hermes session lost a turn in a crash; returns the thread."""
    first = await rig.fake.trigger("Migrate the host.", author="robin")
    await rig.until(rig.model.requests, "the first model request")
    await asyncio.to_thread(rig.model.wait_held, "lost")
    assert rig.gateway is not None
    assert rig.gateway.pgid is not None
    # SIGKILL at once, as a reboot ends Hermes: no turn ends in order, so Hermes fences the
    # running turn ``unknown`` at its next start.
    os.killpg(rig.gateway.pgid, signal.SIGKILL)
    # macOS answers a later killpg on a group of exited members with EPERM (red team r1).
    with contextlib.suppress(PermissionError):
        rig.gateway.stop()
    rig.model.release("lost")
    rig.fake.set_run_state(first.run_id, "failed", "interrupted")
    await _start_gateway(rig)
    return first.thread_id


async def _run(rig: HermesRig, thread: Any, text: str, **kwargs: Any) -> tuple[Any, list[str]]:
    run = await rig.fake.trigger(
        text, author="robin", thread_id=thread, session_mode="resume", **kwargs
    )
    await _end(rig, run.run_id)
    return run, _texts(rig, run.run_id)


async def test_paused_session_answers_every_run_and_owner_discard_resumes(
    world: HermesWorld,
) -> None:
    """Every run on the paused session shows why; ``/discard`` of an owner drops the lost turn.

    One rig for both, as each start of the pinned Hermes costs about a minute of the CI job's
    limit; ``/discard`` of a run not proven an owner's stays text (unit test of the handler).
    """
    async with hermes_rig(world, _script(), tool_class="host") as rig:
        thread = await _paused(rig)
        _, first = await _run(rig, thread, "Are you there?")
        assert len(first) == 1, (first, gateway_log(rig))
        assert PAUSED in first[0], (first, gateway_log(rig))
        second, texts = await _run(rig, thread, "Still there?")
        assert len(texts) == 1, (texts, gateway_log(rig))
        assert "paused" in texts[0], (texts, gateway_log(rig))
        assert "/discard" in texts[0]
        assert rig.fake.run_state(second.run_id).state == "completed"
        # The owner's /discard (tool class ``host``, 07 SR2) reaches Hermes as the command.
        _, texts = await _run(rig, thread, "/discard")
        assert texts, (texts, gateway_log(rig))
        assert COMMAND_FAILED.format(name="discard") not in texts, (texts, gateway_log(rig))
        assert not any(PAUSED in t for t in texts), (texts, gateway_log(rig))
        await rig.until(lambda: any(QUEUED in str(m["content_md"]) for m in rig.fake.messages))
        await asyncio.sleep(1)
        run, texts = await _run(rig, thread, "Go on.")
        # A model answer of the script (which one depends on how many saved messages ran).
        assert texts[-1:] in ([NEXT], [QUEUED]), (texts, gateway_log(rig))
        assert rig.fake.run_state(run.run_id).state == "completed"


async def test_owner_reset_starts_a_fresh_session(world: HermesWorld) -> None:
    """``/reset`` of an owner gives the thread a new Hermes session that answers."""
    async with hermes_rig(world, _script(), tool_class="host") as rig:
        thread = await _paused(rig)
        before = rig.fake.sessions(thread)
        _, texts = await _run(rig, thread, "/reset")
        assert texts, (texts, gateway_log(rig))
        assert not any(PAUSED in t for t in texts), (texts, gateway_log(rig))
        # Hermes' confirmed reply (returned by its resolver, not sent) is the run's answer.
        assert COMMAND_FAILED.format(name="reset") not in texts, (texts, gateway_log(rig))
        _, texts = await _run(rig, thread, "Go on.")
        assert texts[-1:] in ([NEXT], [QUEUED]), (texts, gateway_log(rig))
        assert rig.fake.sessions(thread) != before
