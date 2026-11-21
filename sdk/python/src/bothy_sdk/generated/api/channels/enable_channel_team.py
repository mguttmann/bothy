from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.team import Team
from ...types import Response


def _get_kwargs(
    channel_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/channels/{channel_id}/team".format(
            channel_id=quote(str(channel_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | Team:
    if response.status_code == 200:
        response_200 = Team.from_dict(response.json())

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

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | Team]:
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
) -> Response[Problem | Team]:
    """Switch a channel to team mode

     Switches a text channel to team mode with the caller as its sponsor (09 3.9.1); on a team channel it
    makes the caller the sponsor and keeps members and brake state. Owners only (`team.update`). A DM
    channel is `409 conflict` with `reason` `dm_channel`, a forum `channel_kind`, an archived or read-
    only channel `channel_archived` or `channel_read_only`. Audited.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Team]
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
) -> Problem | Team | None:
    """Switch a channel to team mode

     Switches a text channel to team mode with the caller as its sponsor (09 3.9.1); on a team channel it
    makes the caller the sponsor and keeps members and brake state. Owners only (`team.update`). A DM
    channel is `409 conflict` with `reason` `dm_channel`, a forum `channel_kind`, an archived or read-
    only channel `channel_archived` or `channel_read_only`. Audited.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Team
    """

    return sync_detailed(
        channel_id=channel_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | Team]:
    """Switch a channel to team mode

     Switches a text channel to team mode with the caller as its sponsor (09 3.9.1); on a team channel it
    makes the caller the sponsor and keeps members and brake state. Owners only (`team.update`). A DM
    channel is `409 conflict` with `reason` `dm_channel`, a forum `channel_kind`, an archived or read-
    only channel `channel_archived` or `channel_read_only`. Audited.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Team]
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
) -> Problem | Team | None:
    """Switch a channel to team mode

     Switches a text channel to team mode with the caller as its sponsor (09 3.9.1); on a team channel it
    makes the caller the sponsor and keeps members and brake state. Owners only (`team.update`). A DM
    channel is `409 conflict` with `reason` `dm_channel`, a forum `channel_kind`, an archived or read-
    only channel `channel_archived` or `channel_read_only`. Audited.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Team
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            client=client,
        )
    ).parsed
