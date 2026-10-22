from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.command_list import CommandList
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    channel_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/channels/{channel_id}/commands".format(
            channel_id=quote(str(channel_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> CommandList | Problem:
    if response.status_code == 200:
        response_200 = CommandList.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 404:
        response_404 = Problem.from_dict(response.json())

        return response_404

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[CommandList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[CommandList | Problem]:
    """List commands usable in a channel

     Lists the registered commands offered in the channel: those of the channel's effective operator
    first, then those
    of guest agents (only with `allow_mentioned_agents`) and of apps with `write` on the channel, each
    group ordered by
    lower-case name, owner and ID. Per owner and name only the most specific scope appears (`channel`
    before `space`
    before `instance`). The built-in commands are not part of the list; the client shows them from its
    own list. A dm
    channel lists nothing.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CommandList | Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> CommandList | Problem | None:
    """List commands usable in a channel

     Lists the registered commands offered in the channel: those of the channel's effective operator
    first, then those
    of guest agents (only with `allow_mentioned_agents`) and of apps with `write` on the channel, each
    group ordered by
    lower-case name, owner and ID. Per owner and name only the most specific scope appears (`channel`
    before `space`
    before `instance`). The built-in commands are not part of the list; the client shows them from its
    own list. A dm
    channel lists nothing.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CommandList | Problem
    """

    return sync_detailed(
        channel_id=channel_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[CommandList | Problem]:
    """List commands usable in a channel

     Lists the registered commands offered in the channel: those of the channel's effective operator
    first, then those
    of guest agents (only with `allow_mentioned_agents`) and of apps with `write` on the channel, each
    group ordered by
    lower-case name, owner and ID. Per owner and name only the most specific scope appears (`channel`
    before `space`
    before `instance`). The built-in commands are not part of the list; the client shows them from its
    own list. A dm
    channel lists nothing.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CommandList | Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> CommandList | Problem | None:
    """List commands usable in a channel

     Lists the registered commands offered in the channel: those of the channel's effective operator
    first, then those
    of guest agents (only with `allow_mentioned_agents`) and of apps with `write` on the channel, each
    group ordered by
    lower-case name, owner and ID. Per owner and name only the most specific scope appears (`channel`
    before `space`
    before `instance`). The built-in commands are not part of the list; the client shows them from its
    own list. A dm
    channel lists nothing.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CommandList | Problem
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            client=client,
        )
    ).parsed
