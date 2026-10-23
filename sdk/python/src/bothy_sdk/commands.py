# SPDX-License-Identifier: Apache-2.0
"""Slash commands of an agent: a declared list, reconciliation and autocomplete.

An adapter may declare its commands (``AdapterConfig.commands``). When it does, the adapter
reconciles them after every ``ready`` (docs/DECISIONS.md D-M2-29-12; 09 10.2 asks for one
request, but the API has no such operation and no list of own commands, a reported spec gap):
own commands are those of ``listChannelCommands`` over the channels of ``listServedChannels``
whose ``owner_principal_id`` is the agent; missing ones are registered, changed ones updated with
``If-Match`` of their version, undeclared ones removed. ``None`` (the default) means the adapter
does not manage commands; an empty list removes all own commands.

Autocomplete (09 10.5, R-AG-14): the handler's ``on_autocomplete`` runs under a guard that ends
at the request's ``expires_at``; more choices than ``command.autocomplete_max_choices`` raise and
nothing is sent; on timeout the task is cancelled and nothing is sent (the server answers an
empty list). The SDK never calls a model here, and a handler must not either.

Examples:
    >>> spec = CommandSpec("status", "Show the run status")
    >>> spec.to_register().name
    'status'
"""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Awaitable, Callable, Mapping, Sequence
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any
from uuid import UUID

from bothy_sdk.client import Client
from bothy_sdk.clock import Clock
from bothy_sdk.errors import ConflictError
from bothy_sdk.generated.api.agent_runtime import list_served_channels
from bothy_sdk.generated.api.commands import (
    list_channel_commands,
    register_command,
    remove_command,
    update_command,
)
from bothy_sdk.generated.models import (
    AgentChannelConfigList,
    Command,
    CommandChoice,
    CommandLevel,
    CommandList,
    CommandOption,
    CommandRegister,
    TargetKind,
)
from bothy_sdk.settings import SdkSettings

__all__ = [
    "AutocompleteRequest",
    "AutocompleteTooManyChoicesError",
    "CommandSpec",
    "ReconcileResult",
    "guarded_autocomplete",
    "reconcile_commands",
]

_LOG = logging.getLogger("bothy_sdk")


@dataclass(frozen=True)
class CommandSpec:
    """A command the adapter declares; maps to ``CommandRegister``.

    Attributes:
        name: The command name.
        description: At most ``command.description_max_chars`` characters.
        options: The options (generated ``CommandOption``).
        scope_kind: Where it applies; ``None`` leaves the server default.
        scope_id: The scope target, if any.
        required_level: The required level, if not the default.
        control: True for a control command.
    """

    name: str
    description: str
    options: Sequence[CommandOption] = field(default_factory=tuple)
    scope_kind: str | None = None
    scope_id: UUID | None = None
    required_level: str | None = None
    control: bool = False

    def to_register(self, settings: SdkSettings | None = None) -> CommandRegister:
        """Build the generated ``CommandRegister``.

        Args:
            settings: The limit ``command.description_max_chars``.

        Returns:
            The registration body.

        Raises:
            ValueError: The description is longer than the limit.
        """
        limit = (settings or SdkSettings()).command_description_max_chars
        if len(self.description) > limit:
            raise ValueError(f"description of {self.name} is longer than the limit")
        body = CommandRegister(name=self.name, description=self.description)
        if self.options:
            body.options = list(self.options)
        if self.scope_kind is not None:
            body.scope_kind = TargetKind(self.scope_kind)
        if self.scope_id is not None:
            body.scope_id = self.scope_id
        if self.required_level is not None:
            body.required_level = CommandLevel(self.required_level)
        if self.control:
            body.control = True
        return body


@dataclass(frozen=True)
class ReconcileResult:
    """What a reconciliation changed (command names)."""

    created: tuple[str, ...] = ()
    updated: tuple[str, ...] = ()
    removed: tuple[str, ...] = ()


def _differs(existing: Command, wanted: CommandRegister) -> bool:
    mine = wanted.to_dict()
    theirs = existing.to_dict()
    return any(theirs.get(k) != v for k, v in mine.items())


async def own_commands(client: Client, agent_id: UUID) -> dict[str, Command]:
    """Return the agent's own commands by name over every served channel.

    Args:
        client: The REST client.
        agent_id: The agent's principal ID.

    Returns:
        Own commands by name.
    """
    served = await client.call_as(AgentChannelConfigList, list_served_channels.asyncio_detailed)
    found: dict[UUID, Command] = {}
    for channel in served.items:
        listed = await client.call_as(
            CommandList, list_channel_commands.asyncio_detailed, channel_id=channel.channel_id
        )
        for command in listed.items:
            if command.owner_principal_id == agent_id:
                found[command.id] = command
    return {c.name: c for c in found.values()}


async def reconcile_commands(
    client: Client,
    agent_id: UUID,
    declared: Sequence[CommandSpec] | None,
    settings: SdkSettings | None = None,
) -> ReconcileResult:
    """Make the agent's own commands equal to ``declared`` (D-M2-29-12).

    Args:
        client: The REST client.
        agent_id: The agent's principal ID.
        declared: The declared commands; ``None`` does nothing.
        settings: The limit of the descriptions.

    Returns:
        What changed.

    Raises:
        ValueError: A declared description is too long (before any request).
        ProblemError: A refusal other than a ``409 conflict`` on register.
    """
    if declared is None:
        return ReconcileResult()
    wanted = {spec.name: spec.to_register(settings) for spec in declared}
    existing = await own_commands(client, agent_id)
    created, updated, removed = [], [], []
    for name, body in wanted.items():
        current = existing.get(name)
        if current is None:
            try:
                await client.call(register_command.asyncio_detailed, body=body)
                created.append(name)
            except ConflictError as exc:
                _LOG.info("command %s not registered: %s", name, exc.code)
        elif _differs(current, body):
            await client.call(
                update_command.asyncio_detailed,
                command_id=current.id,
                body=body,
                if_match=f'"{current.version}"',
            )
            updated.append(name)
    for name, current in existing.items():
        if name not in wanted:
            await client.call(remove_command.asyncio_detailed, command_id=current.id)
            removed.append(name)
    return ReconcileResult(tuple(created), tuple(updated), tuple(removed))


@dataclass(frozen=True)
class AutocompleteRequest:
    """One ``command.autocomplete_requested`` event, as the handler sees it.

    Attributes:
        request_id: The request (the answer names it).
        command_id: The command.
        option: The option being typed.
        value: What the person typed so far.
        options: The other option values so far, as raw dicts.
        channel_id: The channel.
        thread_id: The thread, if any.
        user_id: The person typing.
        expires_at: The deadline of the answer.
    """

    request_id: UUID
    command_id: UUID
    option: str
    value: str
    options: Sequence[Mapping[str, Any]]
    channel_id: UUID
    thread_id: UUID | None
    user_id: UUID
    expires_at: datetime


class AutocompleteTooManyChoicesError(ValueError):
    """The handler returned more than ``command.autocomplete_max_choices`` choices."""


async def guarded_autocomplete(
    handler: Callable[[AutocompleteRequest], Awaitable[Sequence[CommandChoice]]],
    request: AutocompleteRequest,
    send: Callable[[UUID, Sequence[CommandChoice]], Awaitable[None]],
    *,
    clock: Clock,
    settings: SdkSettings | None = None,
) -> bool:
    """Run the handler until the request expires and send its choices.

    Args:
        handler: The handler's ``on_autocomplete``.
        request: The request.
        send: Sends the choices (op ``command.autocomplete_result`` or ``answerAutocomplete``).
        clock: The time source of the guard.
        settings: The limit ``command.autocomplete_max_choices``.

    Returns:
        True when an answer was sent, False after a timeout.

    Raises:
        AutocompleteTooManyChoicesError: Too many choices; nothing was sent.
    """
    limit = (settings or SdkSettings()).autocomplete_max_choices
    remaining = (request.expires_at - clock.now()).total_seconds()
    if remaining <= 0:
        return False
    work = asyncio.ensure_future(handler(request))
    guard = asyncio.ensure_future(clock.sleep(remaining))
    done, _ = await asyncio.wait({work, guard}, return_when=asyncio.FIRST_COMPLETED)
    if work not in done:
        work.cancel()
        _LOG.info("autocomplete %s timed out; nothing sent", request.request_id)
        return False
    guard.cancel()
    choices = list(work.result())
    if len(choices) > limit:
        raise AutocompleteTooManyChoicesError("more choices than command.autocomplete_max_choices")
    await send(request.request_id, choices)
    return True
