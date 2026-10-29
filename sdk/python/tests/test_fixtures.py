# SPDX-License-Identifier: Apache-2.0
"""The FakeServer's fixtures decode with the generated core (docs/spec/09-agents.md 16.1)."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

import bothy_sdk  # noqa: F401 - installs the tolerance hooks
from bothy_sdk.generated import models
from bothy_sdk.testing import fakeserver
from bothy_sdk.tolerance import UnknownEvent, decode_event

FIXTURES = Path(fakeserver.__file__).parent / "fixtures"

EVENT_FIXTURES = (
    "agent.invocation",
    "agent.followup",
    "interaction.created",
    "command.invoked",
    "run.cancel_requested",
    "run.state_changed",
    # Constructed from AgentConfigChangedEvent for the runner scenarios (M2-34), not recorded.
    "agent.config_changed",
    # Constructed from AccessChangedEvent for the runner's live channel reload, not recorded.
    "access.changed",
)
# Gateway-only events are not in the Event union; their own generated model decodes them.
EPHEMERAL_FIXTURES = {"command.autocomplete_requested": models.CommandAutocompleteRequestedEvent}
BODY_FIXTURES = {
    "run": models.Run,
    "run_context": models.RunContext,
    "discovery": models.Discovery,
}
_TOKEN_MEMBERS = {"invocation_token", "api_token", "access_token", "dpop"}


def _load(name: str) -> dict[str, Any]:
    loaded: dict[str, Any] = json.loads((FIXTURES / f"{name}.json").read_text(encoding="utf-8"))
    return loaded


def _members(value: object) -> set[str]:
    if isinstance(value, dict):
        return set(value) | {m for v in value.values() for m in _members(v)}
    if isinstance(value, list):
        return {m for v in value for m in _members(v)}
    return set()


def test_every_fixture_file_is_known() -> None:
    names = {p.stem for p in FIXTURES.glob("*.json")}
    assert names == set(EVENT_FIXTURES) | set(EPHEMERAL_FIXTURES) | set(BODY_FIXTURES)


@pytest.mark.parametrize("name", EVENT_FIXTURES)
def test_event_fixture_decodes(name: str) -> None:
    data = _load(name)
    event = decode_event(data)
    assert not isinstance(event, UnknownEvent)
    assert event.to_dict()["type"] == name
    assert isinstance(data["seq"], int)


@pytest.mark.parametrize("name", sorted(EPHEMERAL_FIXTURES))
def test_ephemeral_fixture_decodes(name: str) -> None:
    data = _load(name)
    model = EPHEMERAL_FIXTURES[name].from_dict(data)
    assert model.to_dict()["type"] == name
    assert "seq" not in data


@pytest.mark.parametrize("name", sorted(BODY_FIXTURES))
def test_body_fixture_decodes(name: str) -> None:
    model = BODY_FIXTURES[name].from_dict(_load(name))
    assert model.to_dict()


@pytest.mark.parametrize("name", [*EVENT_FIXTURES, *EPHEMERAL_FIXTURES, *BODY_FIXTURES])
def test_fixture_is_normalized_without_tokens(name: str) -> None:
    data = _load(name)
    assert not (_members(data) & _TOKEN_MEMBERS)
    assert "redelivered" not in data
    text = json.dumps(data)
    assert "eyJ" not in text  # no JWS segment anywhere
