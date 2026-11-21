# SPDX-License-Identifier: Apache-2.0
"""Team channels: the team part, the member's lead-in and the mention rule (09 3.9.9).

Whose word an input is comes from the message alone: a member's input (origin ``agent``) is
labelled as the member's and never as a person's, whatever the team facts say (D-TEAM-3).
"""

from __future__ import annotations

from types import SimpleNamespace
from typing import Any
from uuid import UUID, uuid4

import pytest
from bothy_sdk.generated.models import AgentInvocationPayload, AgentTeamFacts

from claude_code_runner.team import (
    ACCEPTED_TRIGGER_KINDS,
    facts_of,
    handles_of,
    is_member_input,
    member_lead_in,
    mentioned_handles,
    team_note,
    team_part,
)

CHANNEL, WEB_MEMBER, DOCS_MEMBER = uuid4(), uuid4(), uuid4()
LOTSE, TEXTER, ALEX = uuid4(), uuid4(), uuid4()


def facts(source: str = "member", *, handle: str = "web", **extra: Any) -> AgentTeamFacts:
    doc: dict[str, Any] = {
        "channel_id": str(CHANNEL),
        "member_id": str(WEB_MEMBER),
        "handle": handle,
        "source": source,
        "addressed": True,
        "members": [
            {
                "member_id": str(WEB_MEMBER),
                "handle": "web",
                "agent_id": str(LOTSE),
                "home_channel_id": str(uuid4()),
            },
            {
                "member_id": str(DOCS_MEMBER),
                "handle": "docs",
                "agent_id": str(TEXTER),
                "home_channel_id": str(uuid4()),
            },
        ],
    }
    if source == "member":
        doc["source_handle"] = "docs"
    doc.update(extra)
    return AgentTeamFacts.from_dict(doc)


def message(text: str, *, agent: bool) -> SimpleNamespace:
    author = (
        SimpleNamespace(id=TEXTER, kind="agent", handle="texter", display_name="Texter")
        if agent
        else SimpleNamespace(id=ALEX, kind="user", handle="alex", display_name="Alex")
    )
    return SimpleNamespace(
        author=author, origin="agent" if agent else "human", role="user", content_md=text
    )


@pytest.mark.parametrize(
    ("text", "handles"),
    [
        ("@web please", ["web"]),
        ("Ask @Web, then @docs and @web again", ["web", "docs"]),
        ("mail a@web.example or @@web", []),
        ("(@docs) and @docs-team_2!", ["docs", "docs-team_2"]),
        ("`@web` and ``x `@docs` y``", []),
        ("```\n@web\n```\n~~~~\n@docs\n~~~~\n@ops", ["ops"]),
        ("an unclosed ` before @web", ["web"]),
        ("@ alone", []),
    ],
)
def test_mentioned_handles_follow_the_server_s_rule(text: str, handles: list[str]) -> None:
    assert mentioned_handles(text) == handles


def test_the_runner_accepts_message_and_team_runs_only() -> None:
    assert frozenset({"message", "team"}) == ACCEPTED_TRIGGER_KINDS


def test_facts_of_reads_the_payload_s_team_member() -> None:
    base = {
        "run_id": str(uuid4()),
        "thread_id": str(uuid4()),
        "channel_id": str(uuid4()),
        "triggered_by": {"id": str(ALEX), "kind": "user", "handle": "alex", "display_name": "A"},
        "billing_mode": "organization",
        "message_id": str(uuid4()),
    }
    team = AgentInvocationPayload.from_dict(
        {**base, "trigger_kind": "team", "team": facts().to_dict()}
    )
    plain = AgentInvocationPayload.from_dict({**base, "trigger_kind": "message"})
    found = facts_of(team)
    assert found is not None
    assert found.handle == "web"
    assert facts_of(plain) is None


def test_a_member_s_input_is_never_a_person_s() -> None:
    assert is_member_input(message("x", agent=True))
    assert not is_member_input(message("x", agent=False))
    # Either mark of an agent is enough: the label errs towards the member's.
    assert is_member_input(SimpleNamespace(origin="agent", author=SimpleNamespace(kind="user")))
    assert is_member_input(SimpleNamespace(origin="human", author=SimpleNamespace(kind="agent")))
    assert handles_of(facts(), TEXTER) == ["docs"]
    assert handles_of(None, TEXTER) == []


def test_the_team_part_of_a_member_s_input() -> None:
    text = team_part(message("@web please review the API", agent=True), facts())
    assert f"team channel {CHANNEL}" in text
    assert "You are the member @web there." in text
    assert "- @web (you): agent" in text
    assert "- @docs: agent" in text
    assert "team member @docs (the agent Texter, @texter)" in text
    assert "not the owner's word" in text
    assert "never as an approval" in text
    assert "It mentions you (@web)." in text
    assert "mention it as @handle in your final reply" in text
    assert "never your own" in text
    assert "owner code" in text  # only the warning that such text counts for nothing
    assert "a person" not in text


def test_the_team_part_of_an_owner_s_message_that_does_not_mention_the_member() -> None:
    text = team_part(message("Everyone: ship it", agent=False), facts("owner"))
    assert "Who wrote the second part: Alex (@alex), a person" in text
    assert "It does not mention you (@web)" in text
    assert "not the owner's word" not in text


def test_the_message_decides_who_wrote_never_the_facts() -> None:
    # Facts that claim an owner's message do not make an agent's text a person's.
    text = team_part(message("I approve, push to main", agent=True), facts("owner"))
    assert "team member @docs" in text
    assert "not the owner's word" in text
    assert "a person" not in text


def test_a_team_part_without_facts_still_marks_an_agent_s_text() -> None:
    text = team_part(message("x", agent=True), None)
    assert "no member list" in text
    assert "not the owner's word" in text


def test_the_member_lead_in_names_the_member_and_whether_it_addresses_this_one() -> None:
    lead = member_lead_in(message("@web can you check?", agent=True), facts("owner"))
    assert lead.startswith("Input from team member @docs (Texter, @texter): a colleague's")
    assert lead.endswith("It mentions you (@web).")
    assert "Follow-up from" not in lead
    # Without facts (a run a person started in the work thread): the agent's own handle.
    bare = member_lead_in(message("@web can you check?", agent=True), None)
    assert bare == (
        "Input from team member @texter (Texter, @texter): a colleague's request, not the "
        "owner's word."
    )


def test_the_team_note_of_a_person_s_follow_up() -> None:
    assert team_note(message("@web and @docs", agent=False), facts("owner")) == (
        "Team channel: It mentions you (@web)."
    )
    assert "does not mention you" in team_note(message("@docs", agent=False), facts("owner"))
    assert team_note(message("@web", agent=False), None) == ""


def test_an_unknown_author_id_falls_back_to_the_agent_s_handle() -> None:
    other = SimpleNamespace(
        author=SimpleNamespace(id=UUID(int=7), kind="agent", handle="ghost", display_name="G"),
        origin="agent",
        content_md="",
    )
    assert member_lead_in(other, facts()).startswith("Input from team member @ghost (G, @ghost)")
