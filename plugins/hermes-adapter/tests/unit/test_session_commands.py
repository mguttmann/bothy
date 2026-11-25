# SPDX-License-Identifier: Apache-2.0
"""Which typed message is an owner's session command (09 17.3, FIX-hermes-session-commands).

Only a run of ``trigger_kind = message`` on an agent of tool class ``host`` (07 SR2: the server
lets only an active owner trigger it), whose triggering message is the run's person's own text
(``origin = human``, author of kind ``user`` equal to the token's ``sub``) without attachments
and exactly one of the session commands, sends the command. Everything else stays text.
"""

from __future__ import annotations

import asyncio
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from uuid import uuid4

import pytest
from adapter_kit import FakeRuntime, rig
from bothy_sdk.generated.models import MessageOrigin, PrincipalKind

from bothy_hermes_adapter.hermes_commands import COMMAND_FAILED
from bothy_hermes_adapter.runtime import CommandRequest, TurnOutcome, TurnRequest
from bothy_hermes_adapter.session_commands import (
    NOT_ANSWERED,
    command_line,
    confirmed,
    owner_session_command,
)

OWNER = uuid4()


def _claims(**changes: Any) -> SimpleNamespace:
    values = {"trigger_kind": "message", "tool_class": "host", "sub": OWNER}
    values.update(changes)
    return SimpleNamespace(**values)


def _trigger(text: str = "/discard", **changes: Any) -> SimpleNamespace:
    values: dict[str, Any] = {
        "origin": MessageOrigin.HUMAN,
        "author": SimpleNamespace(id=OWNER, kind=PrincipalKind.USER),
        "attachments": [],
        "content_md": text,
    }
    values.update(changes)
    return SimpleNamespace(**values)


@pytest.mark.parametrize(
    ("text", "line"),
    [
        ("/discard", "/discard"),
        (" /reset\n", "/reset"),
        ("/New", "/new"),
        ("/stop", "/stop"),
    ],
)
def test_owner_message_of_exactly_a_session_command_is_the_command(text: str, line: str) -> None:
    assert owner_session_command(_claims(), _trigger(text)) == line


@pytest.mark.parametrize(
    "text",
    ["/discard now", "please /discard", "/discharge", "/compress", "/status", "discard", "", "/"],
)
def test_other_texts_stay_text(text: str) -> None:
    assert owner_session_command(_claims(), _trigger(text)) is None


@pytest.mark.parametrize(
    "claims",
    [
        _claims(tool_class="chat"),
        _claims(tool_class="scoped"),
        # A team input: a member's reply, or an owner's copy whose authority is the sponsor.
        _claims(trigger_kind="team"),
        _claims(trigger_kind="followup"),
        _claims(trigger_kind="command"),
        # The message's author is not the run's person.
        _claims(sub=uuid4()),
    ],
)
def test_runs_the_server_does_not_prove_an_owners_stay_text(claims: SimpleNamespace) -> None:
    assert owner_session_command(claims, _trigger()) is None


@pytest.mark.parametrize(
    "trigger",
    [
        _trigger(origin=MessageOrigin.AGENT),
        _trigger(author=SimpleNamespace(id=OWNER, kind=PrincipalKind.AGENT)),
        _trigger(attachments=[object()]),
        _trigger(content_md=None),
        None,
    ],
)
def test_messages_not_the_owners_own_text_stay_text(trigger: SimpleNamespace | None) -> None:
    assert owner_session_command(_claims(), trigger) is None


def test_reset_and_new_are_confirmed_at_once() -> None:
    assert confirmed("/new")
    assert confirmed("/reset")
    assert not confirmed("/discard")
    assert not confirmed("/stop")


def test_command_line_is_lower_case() -> None:
    assert command_line("/DISCARD") == "/discard"


# ---- the run of a session command and a run Hermes leaves without an answer (handler) -------


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.mark.anyio
async def test_owner_discard_run_sends_the_command_and_posts_hermes_reply(tmp_path: Path) -> None:
    """Tool class ``host``: the run sends ``/discard`` and Hermes' reply is its answer."""
    rt = FakeRuntime(replies=["Dropped 1 lost turn."])
    async with rig(tmp_path, runtime=rt, tool_class="host") as r:
        run = await r.fake.trigger("/discard", author="robin")
        await r.until(lambda: r.finished(run.run_id))
    assert r.fake.run_state(run.run_id).state == "completed"
    assert [(c.text, c.confirm) for c in rt.commands] == [("/discard", False)]
    assert rt.turns == []
    assert [m["content_md"] for m in r.fake.messages if m.get("run_id")] == ["Dropped 1 lost turn."]


@pytest.mark.anyio
async def test_owner_reset_is_confirmed_and_no_reply_says_it_failed(tmp_path: Path) -> None:
    rt = FakeRuntime(replies=[])
    async with rig(tmp_path, runtime=rt, tool_class="host") as r:
        run = await r.fake.trigger("/reset", author="robin")
        await r.until(lambda: r.finished(run.run_id))
    assert [(c.text, c.confirm) for c in rt.commands] == [("/reset", True)]
    assert [m["content_md"] for m in r.fake.messages if m.get("run_id")] == [
        COMMAND_FAILED.format(name="reset")
    ]


@pytest.mark.anyio
async def test_discard_on_another_tool_class_is_a_turn(tmp_path: Path) -> None:
    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt, tool_class="scoped") as r:
        run = await r.fake.trigger("/discard", author="robin")
        await r.until(lambda: r.finished(run.run_id))
    assert rt.commands == []
    assert [t.user_text for t in rt.turns] == ["/discard"]


@pytest.mark.anyio
async def test_a_run_hermes_takes_without_a_turn_or_answer_is_answered(tmp_path: Path) -> None:
    """A paused session: Hermes admits the message silently; the run still shows why."""

    async def silent(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=silent)
    async with rig(tmp_path, runtime=rt) as r:
        run = await r.fake.trigger("Hello @Lotse")
        await r.until(lambda: r.finished(run.run_id))
    assert r.fake.run_state(run.run_id).state == "completed"
    assert [m["content_md"] for m in r.fake.messages if m.get("run_id")] == [NOT_ANSWERED]


@pytest.mark.anyio
async def test_a_silent_hermes_turn_stays_silent(tmp_path: Path) -> None:
    """A turn that ran (``pre_llm_call``) and answered nothing gets no notice."""

    async def quiet(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.events.on_turn_started(turn.thread_id)
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=quiet)
    async with rig(tmp_path, runtime=rt) as r:
        run = await r.fake.trigger("Hello @Lotse")
        await r.until(lambda: r.finished(run.run_id))
    assert [m["content_md"] for m in r.fake.messages if m.get("run_id")] == []


@pytest.mark.anyio
async def test_a_followup_during_a_session_command_is_not_steered(tmp_path: Path) -> None:
    """The command run has no turn: a follow-up meanwhile is never passed into Hermes."""
    gate = asyncio.Event()

    class Blocking(FakeRuntime):
        async def run_command(self, command: CommandRequest) -> list[str]:
            self.commands.append(command)
            await gate.wait()
            return ["Dropped 1 lost turn."]

    rt = Blocking()
    async with rig(tmp_path, runtime=rt, tool_class="host") as r:
        run = await r.fake.trigger("/discard", author="robin")
        await r.until(lambda: rt.commands)
        await r.fake.deliver_followup(run.run_id, "and this", author="robin")
        await asyncio.sleep(0.2)
        gate.set()
        await r.until(lambda: r.finished(run.run_id))
    assert rt.steered == []
