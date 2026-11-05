# SPDX-License-Identifier: Apache-2.0
"""Per-channel skill bindings and channel prompts against the pinned Hermes.

The profile binds skills and an extra prompt to one channel of the server the way Hermes' Discord
platform binds them to a Discord channel (``channel_skill_bindings``, ``channel_prompts``). A
person's message in a thread of the bound channel starts a Hermes session with the bound skill
loaded and the channel prompt among the channel instructions on the user side of the turn, next
to the context the server delivered; a channel without a binding stays as before.
"""

from __future__ import annotations

import json
from typing import Any

import pytest
import yaml
from bothy_sdk.message_format import MESSAGE_FORMAT_RULES
from fake_hermes_kit import PLATFORM_KEY, HermesRig, HermesWorld, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

from bothy_hermes_adapter.context_format import SYSTEM_NOTE

pytestmark = pytest.mark.anyio

SKILL_MARKER = "TICKET-ANSWER-SKILL-MARKER"
SKILL = f"""---
name: ticket-answer
description: Handle an answer in a ticket thread
---
# Ticket answer

Follow the {SKILL_MARKER} rule: treat yes or no as the decision on the ticket.
"""
HIDDEN_MARKER = "TICKET-HIDDEN-SKILL-MARKER"
HIDDEN_SKILL = f"""---
name: ticket-hidden
description: A skill the profile disables on this platform
---
# Hidden

Follow the {HIDDEN_MARKER} rule.
"""
PROMPT_MARKER = "TICKET-CHANNEL-PROMPT-MARKER"
CHANNEL_PROMPT = f"{PROMPT_MARKER}: every thread of this forum is exactly one ticket."
BOUND_ASK = "yes, go ahead"
UNBOUND_ASK = "Please plan the trip to the lake."
BIND_ASK = "and the next ticket?"
AUTO_LOADED = 'The "ticket-answer" skill is auto-loaded'


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _texts(content: Any) -> list[str]:
    if isinstance(content, str):
        return [content]
    out: list[str] = []
    if isinstance(content, list):
        for part in content:
            if isinstance(part, dict) and isinstance(part.get("text"), str):
                out.append(part["text"])
    return out


def _system_and_user(body: dict[str, Any]) -> tuple[str, str]:
    system = "\n".join(_texts(body.get("system")))
    messages = body.get("messages") or []
    for message in messages:
        if message.get("role") == "system":
            system += "\n" + "\n".join(_texts(message.get("content")))
    users = [m for m in messages if m.get("role") == "user"]
    user = "\n".join(_texts(users[-1].get("content"))) if users else ""
    return system, user


def _bind(rig: HermesRig, channel: str) -> None:
    """Bind the skills and the prompt to ``channel`` in the profile's platform section.

    ``ticket-hidden`` is bound too but disabled for the platform (``skills.platform_disabled``).
    """
    path = rig.home / "config.yaml"
    config = yaml.safe_load(path.read_text(encoding="utf-8"))
    extra = config["gateway"]["platforms"][PLATFORM_KEY]["extra"]
    extra["channel_skill_bindings"] = [
        {"id": channel, "skills": ["ticket-answer", "ticket-hidden"]}
    ]
    skills = config.setdefault("skills", {})
    skills.setdefault("platform_disabled", {})[PLATFORM_KEY] = ["ticket-hidden"]
    extra["channel_prompts"] = {channel: CHANNEL_PROMPT}
    path.write_text(yaml.safe_dump(config, sort_keys=True), encoding="utf-8")


def _body_for(rig: HermesRig, ask: str) -> tuple[str, str]:
    for body in rig.model.raw:
        if not body.get("tools"):
            continue
        system, user = _system_and_user(body)
        if ask in user:
            return system, user
    raise AssertionError(f"no model request with {ask!r}:\n{gateway_log(rig)}")


async def _done(rig: HermesRig, run_id: Any) -> None:
    try:
        await rig.until(lambda: rig.finished(run_id), "the end of the run")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None
    assert rig.fake.run_state(run_id).state == "completed", gateway_log(rig)


async def test_bound_channel_loads_skills_and_combines_the_prompt(world: HermesWorld) -> None:
    from fake_hermes_kit import _start_gateway

    script = [ScriptTurn(text=(f"Answer {n}.",), paced=True) for n in range(3)]
    texts = [BOUND_ASK, UNBOUND_ASK, BIND_ASK]
    async with hermes_rig(world, script, texts=texts, start=False) as rig:
        skill = rig.home / "skills" / "ticket-answer" / "SKILL.md"
        skill.parent.mkdir(parents=True, exist_ok=True)
        skill.write_text(SKILL, encoding="utf-8")
        hidden = rig.home / "skills" / "ticket-hidden" / "SKILL.md"
        hidden.parent.mkdir(parents=True, exist_ok=True)
        hidden.write_text(HIDDEN_SKILL, encoding="utf-8")
        forum = rig.fake.add_forum("tickets", None)
        _bind(rig, str(forum))
        await _start_gateway(rig)

        # A person's message in a new thread of the bound forum: the thread inherits the
        # forum's binding, as a Discord thread inherits its parent channel's.
        bound = await rig.fake.trigger(BOUND_ASK, channel_id=forum)
        await _done(rig, bound.run_id)
        system, user = _body_for(rig, BOUND_ASK)
        assert SKILL_MARKER in user, gateway_log(rig)
        assert AUTO_LOADED in user
        assert user.index(SKILL_MARKER) < user.index(BOUND_ASK)
        # A bound skill the profile disables on this platform stays out (as in Hermes' gateway).
        assert HIDDEN_MARKER not in user
        # Combined with the context of the server, never replacing it, on the user side after
        # the person's text: the format rules, the profile's prompt, then the note and context
        # (D-FHUSC-2).
        assert PROMPT_MARKER in user, gateway_log(rig)
        assert PROMPT_MARKER not in system
        assert SYSTEM_NOTE in user
        order = [BOUND_ASK, MESSAGE_FORMAT_RULES, PROMPT_MARKER, SYSTEM_NOTE]
        assert [user.index(part) for part in order] == sorted(user.index(p) for p in order)

        # A channel without a binding: no skill, no channel prompt, the server's context only.
        unbound = await rig.fake.trigger(UNBOUND_ASK)
        await _done(rig, unbound.run_id)
        system, user = _body_for(rig, UNBOUND_ASK)
        assert SKILL_MARKER not in user
        assert PROMPT_MARKER not in system + user
        assert SYSTEM_NOTE in user
        assert json.dumps(rig.model.raw).count(PROMPT_MARKER) >= 1

        # A bind to a session Hermes does not know keeps the new session of the switch: it
        # loads the bound skill like any new session.
        rig.fake.start_session(bound.thread_id, "20260101_000000_deadbeef", started=True)
        missing = await rig.fake.trigger(BIND_ASK, thread_id=bound.thread_id, session_mode="bind")
        await _done(rig, missing.run_id)
        _, user = _body_for(rig, BIND_ASK)
        assert SKILL_MARKER in user, gateway_log(rig)
