# SPDX-License-Identifier: Apache-2.0
"""Commands: reconciliation of the declared list, the description limit and autocomplete.

docs/DECISIONS.md D-M2-29-11 and Design note 12 of the brief: own commands are those of
``listChannelCommands`` over ``listServedChannels`` owned by the agent; missing ones are
registered, changed ones updated with ``If-Match`` of their version, undeclared ones removed; a
``409 conflict`` on register is skipped; ``None`` makes no request. Autocomplete runs under a
guard that ends at ``expires_at``; too many choices raise and nothing is sent.
"""

from __future__ import annotations

import asyncio
import json
from collections.abc import Sequence
from datetime import timedelta
from typing import Any
from uuid import UUID

import httpx
import pytest

from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import Client
from bothy_sdk.commands import (
    AutocompleteRequest,
    AutocompleteTooManyChoicesError,
    CommandSpec,
    ReconcileResult,
    guarded_autocomplete,
    reconcile_commands,
)
from bothy_sdk.errors import ForbiddenError
from bothy_sdk.generated.models import CommandChoice
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing import FakeClock
from bothy_sdk.testing._patience import eventually

pytestmark = pytest.mark.anyio

BASE = "https://example.com/api/v1"
AGENT = UUID("01900000-0000-7000-8000-000000000301")
OTHER = UUID("01900000-0000-7000-8000-000000000302")
CHANNEL_1 = "01900000-0000-7000-8000-000000000103"
CHANNEL_2 = "01900000-0000-7000-8000-000000000113"
SPACE = "01900000-0000-7000-8000-000000000104"


def _id(n: int) -> str:
    return f"01900000-0000-7000-8000-{n:012d}"


def _command(n: int, name: str, description: str, owner: UUID, version: int = 1) -> dict[str, Any]:
    return {
        "id": _id(n),
        "owner_principal_id": str(owner),
        "name": name,
        "description": description,
        "scope_kind": "instance",
        "options": [],
        "required_level": "trigger",
        "control": False,
        "version": version,
    }


def _channel(channel_id: str) -> dict[str, Any]:
    return {
        "channel_id": channel_id,
        "space_id": SPACE,
        "behavior": {
            "require_mention": True,
            "thread_require_mention": False,
            "free_response": False,
            "auto_thread": True,
            "allow_mentioned_agents": True,
            "history_backfill_limit": 50,
            "prompt": "",
            "skills": [],
            "settings_version": 1,
        },
    }


def _problem(status: int, code: str) -> httpx.Response:
    return httpx.Response(
        status,
        json={
            "type": "about:blank",
            "title": code,
            "status": status,
            "code": code,
            "correlation_id": _id(0xE001),
        },
        headers={"Content-Type": "application/problem+json"},
    )


class _CommandServer:
    """A MockTransport handler playing the five command operations an adapter uses."""

    def __init__(self, listed: dict[str, list[dict[str, Any]]]) -> None:
        self.listed = listed
        self.requests: list[httpx.Request] = []
        self.conflict_on: set[str] = set()
        self.forbidden_on: set[str] = set()
        self._next = 0x900

    def __call__(self, request: httpx.Request) -> httpx.Response:  # noqa: PLR0911 - one answer per route
        self.requests.append(request)
        path = request.url.path.removeprefix("/api/v1")
        method = request.method
        if method == "GET" and path == "/agents/me/channels":
            return httpx.Response(200, json={"items": [_channel(c) for c in self.listed]})
        if method == "GET" and path.startswith("/channels/") and path.endswith("/commands"):
            return httpx.Response(200, json={"items": self.listed[path.split("/")[2]]})
        if method == "POST" and path == "/commands":
            body = json.loads(request.content)
            if body["name"] in self.conflict_on:
                return _problem(409, "conflict")
            if body["name"] in self.forbidden_on:
                return _problem(403, "forbidden")
            self._next += 1
            return httpx.Response(
                201, json=_command(self._next, body["name"], body["description"], AGENT)
            )
        if method == "PUT" and path.startswith("/commands/"):
            body = json.loads(request.content)
            return httpx.Response(
                200, json=_command(int(path[-12:]), body["name"], body["description"], AGENT, 9)
            )
        if method == "DELETE" and path.startswith("/commands/"):
            return httpx.Response(204)
        return _problem(404, "not_found")

    def calls(self, method: str) -> list[httpx.Request]:
        return [r for r in self.requests if r.method == method]

    def writes(self) -> list[httpx.Request]:
        return [r for r in self.requests if r.method != "GET"]


def _client(server: _CommandServer) -> Client:
    return Client(
        BASE,
        Credentials(Secret("example-api-token")),
        settings=SdkSettings(retry_max_attempts=1),
        clock=FakeClock(),
        transport=httpx.MockTransport(server),
    )


def _world() -> _CommandServer:
    same = _command(1, "same", "Unchanged", AGENT)
    return _CommandServer(
        {
            CHANNEL_1: [
                same,
                _command(2, "changed", "Old text", AGENT, version=4),
                _command(3, "stale", "No longer declared", AGENT),
                _command(4, "foreign", "Owned by Texter", OTHER),
                _command(5, "taken", "Texter got the name first", OTHER),
            ],
            # The same instance-wide command shows up in every channel; it counts once.
            CHANNEL_2: [same, _command(6, "elsewhere", "Texter, second channel", OTHER)],
        }
    )


# Reconciliation --------------------------------------------------------------------------------


async def test_reconcile_creates_updates_removes() -> None:
    server = _world()
    server.conflict_on.add("taken")
    declared = [
        CommandSpec("same", "Unchanged"),
        CommandSpec("changed", "New text"),
        CommandSpec("new", "Brand new"),
        CommandSpec("taken", "Wanted, but the server refuses with conflict"),
    ]
    async with _client(server) as client:
        result = await reconcile_commands(client, AGENT, declared)
    assert result == ReconcileResult(created=("new",), updated=("changed",), removed=("stale",))

    posts = server.calls("POST")
    assert sorted(json.loads(r.content)["name"] for r in posts) == ["new", "taken"]
    [put] = server.calls("PUT")
    assert put.url.path == f"/api/v1/commands/{_id(2)}"
    assert put.headers["If-Match"] == '"4"'
    assert json.loads(put.content)["description"] == "New text"
    [delete] = server.calls("DELETE")
    assert delete.url.path == f"/api/v1/commands/{_id(3)}"
    # Commands of other owners are never touched, and the unchanged one gets no write.
    untouched = {_id(1), _id(4), _id(5), _id(6)}
    assert not [r for r in server.writes() if r.url.path.rsplit("/", 1)[-1] in untouched]
    listed = [r.url.path for r in server.calls("GET")]
    assert listed.count(f"/api/v1/channels/{CHANNEL_1}/commands") == 1
    assert listed.count(f"/api/v1/channels/{CHANNEL_2}/commands") == 1


async def test_reconcile_other_refusal_raises() -> None:
    server = _world()
    server.forbidden_on.add("new")
    async with _client(server) as client:
        with pytest.raises(ForbiddenError):
            await reconcile_commands(client, AGENT, [CommandSpec("new", "Brand new")])


async def test_no_command_calls_without_declared_commands() -> None:
    server = _world()
    async with _client(server) as client:
        assert await reconcile_commands(client, AGENT, None) == ReconcileResult()
    assert server.requests == []

    server = _world()
    async with _client(server) as client:
        result = await reconcile_commands(client, AGENT, [])
    assert result == ReconcileResult(removed=("same", "changed", "stale"))
    assert server.calls("POST") == []
    assert server.calls("PUT") == []
    removed = sorted(r.url.path.rsplit("/", 1)[-1] for r in server.calls("DELETE"))
    assert removed == sorted([_id(1), _id(2), _id(3)])


# Description limit -----------------------------------------------------------------------------


def test_description_limit() -> None:
    limit = SdkSettings().command_description_max_chars
    assert CommandSpec("ok", "x" * limit).to_register().description == "x" * limit
    with pytest.raises(ValueError, match="longer than the limit"):
        CommandSpec("long", "x" * (limit + 1)).to_register()
    tight = SdkSettings(command_description_max_chars=5)
    assert CommandSpec("ok", "12345").to_register(tight).name == "ok"
    with pytest.raises(ValueError, match="longer than the limit"):
        CommandSpec("long", "123456").to_register(tight)


async def test_description_limit_checked_before_any_request() -> None:
    server = _world()
    tight = SdkSettings(command_description_max_chars=5)
    async with _client(server) as client:
        with pytest.raises(ValueError, match="longer than the limit"):
            await reconcile_commands(client, AGENT, [CommandSpec("long", "123456")], tight)
    assert server.requests == []


def test_spec_maps_to_register() -> None:
    scope = UUID(CHANNEL_1)
    body = CommandSpec(
        "deploy",
        "Deploy the service",
        scope_kind="channel",
        scope_id=scope,
        required_level="trigger",
        control=True,
    ).to_register()
    assert body.to_dict() == {
        "name": "deploy",
        "description": "Deploy the service",
        "scope_kind": "channel",
        "scope_id": str(scope),
        "required_level": "trigger",
        "control": True,
    }
    assert CommandSpec("plain", "Plain").to_register().to_dict() == {
        "name": "plain",
        "description": "Plain",
    }


# Autocomplete ----------------------------------------------------------------------------------


def _request(clock: FakeClock, expires_in: float) -> AutocompleteRequest:
    return AutocompleteRequest(
        request_id=UUID(_id(0x602)),
        command_id=UUID(_id(0x501)),
        option="target",
        value="st",
        options=[],
        channel_id=UUID(CHANNEL_1),
        thread_id=None,
        user_id=UUID(_id(0x201)),
        expires_at=clock.now() + timedelta(seconds=expires_in),
    )


class _Sender:
    def __init__(self) -> None:
        self.sent: list[tuple[UUID, list[CommandChoice]]] = []

    async def __call__(self, request_id: UUID, choices: Sequence[CommandChoice]) -> None:
        self.sent.append((request_id, list(choices)))


async def _sleeping(clock: FakeClock, n: int) -> None:
    """Wait until ``n`` tasks sleep on the clock, so an advance cannot overtake them."""
    await eventually(lambda: clock.sleepers == n, f"{n} sleepers on the clock")


def _choices(n: int) -> list[CommandChoice]:
    return [CommandChoice(name=f"choice {i}", value=f"v{i}") for i in range(n)]


async def test_autocomplete_answered_before_expiry() -> None:
    clock = FakeClock()
    sender = _Sender()
    request = _request(clock, expires_in=3)

    async def handler(req: AutocompleteRequest) -> Sequence[CommandChoice]:
        await clock.sleep(2)
        return _choices(2)

    task = asyncio.ensure_future(guarded_autocomplete(handler, request, sender, clock=clock))
    await _sleeping(clock, 2)
    await clock.advance(2)
    assert await task is True
    assert sender.sent == [(request.request_id, _choices(2))]
    # The guard is gone: moving past the deadline sends nothing more.
    await clock.advance(5)
    assert len(sender.sent) == 1
    assert clock.sleepers == 0


async def test_autocomplete_timeout_sends_nothing() -> None:
    clock = FakeClock()
    sender = _Sender()
    request = _request(clock, expires_in=3)
    cancelled = asyncio.Event()

    async def handler(req: AutocompleteRequest) -> Sequence[CommandChoice]:
        try:
            await clock.sleep(10)
        except asyncio.CancelledError:
            cancelled.set()
            raise
        return _choices(1)

    task = asyncio.ensure_future(guarded_autocomplete(handler, request, sender, clock=clock))
    await _sleeping(clock, 2)
    await clock.advance(3 - 0.25)
    assert not task.done()
    await clock.advance(0.25)
    assert await task is False
    await asyncio.sleep(0)
    assert cancelled.is_set()
    await clock.advance(10)
    assert sender.sent == []


async def test_autocomplete_already_expired_sends_nothing() -> None:
    clock = FakeClock()
    sender = _Sender()
    calls: list[AutocompleteRequest] = []

    async def handler(req: AutocompleteRequest) -> Sequence[CommandChoice]:
        calls.append(req)
        return _choices(1)

    assert await guarded_autocomplete(handler, _request(clock, 0), sender, clock=clock) is False
    assert calls == []
    assert sender.sent == []


async def test_too_many_choices_raise() -> None:
    clock = FakeClock()
    sender = _Sender()
    limit = SdkSettings().autocomplete_max_choices

    async def too_many(req: AutocompleteRequest) -> Sequence[CommandChoice]:
        return _choices(limit + 1)

    with pytest.raises(AutocompleteTooManyChoicesError):
        await guarded_autocomplete(too_many, _request(clock, 3), sender, clock=clock)
    assert sender.sent == []

    async def exactly(req: AutocompleteRequest) -> Sequence[CommandChoice]:
        return _choices(limit)

    assert await guarded_autocomplete(exactly, _request(clock, 3), sender, clock=clock) is True
    assert len(sender.sent[0][1]) == limit

    tight = SdkSettings(autocomplete_max_choices=2)
    with pytest.raises(AutocompleteTooManyChoicesError):
        await guarded_autocomplete(exactly, _request(clock, 3), sender, clock=clock, settings=tight)
    assert len(sender.sent) == 1
